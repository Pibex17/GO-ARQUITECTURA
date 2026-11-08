package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"canchas-sinteticas/internal/canchas"
	"canchas-sinteticas/internal/config"
	"canchas-sinteticas/internal/eventos"
	"canchas-sinteticas/internal/inscripcion"
	"canchas-sinteticas/internal/middleware"
	"canchas-sinteticas/internal/pagos"
	"canchas-sinteticas/internal/partidos"
	"canchas-sinteticas/internal/reservas"
	"canchas-sinteticas/internal/respuesta"
	"canchas-sinteticas/internal/sagas"
	"canchas-sinteticas/internal/sesion"
	"canchas-sinteticas/internal/usuarios"
)

// conectarConReintentos intenta abrir la base varias veces antes de
// rendirse. Hace falta con Docker Compose: aunque el healthcheck de la
// base ya se comprobó, el primer intento de conexión real a veces todavía
// choca con el último instante de arranque de PostgreSQL.
func conectarConReintentos(dsn string) (*gorm.DB, error) {
	var err error
	var db *gorm.DB
	for intento := 1; intento <= 10; intento++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			return db, nil
		}
		log.Printf("no se pudo conectar (intento %d/10): %v", intento, err)
		time.Sleep(2 * time.Second)
	}
	return nil, err
}

// prepararBase deja la base lista para correr la API: opcionalmente borra
// las tablas, migra el esquema y siembra los datos de ejemplo.
func prepararBase(db *gorm.DB, reset bool) error {
	if reset {
		if err := db.Migrator().DropTable(
			&eventos.EventoProcesado{}, &eventos.Evento{}, &sagas.Saga{},
			&sesion.Cuenta{},
			&inscripcion.Inscripcion{}, &partidos.Partido{},
			&pagos.Pago{}, &reservas.Reserva{},
			&canchas.Horario{}, &canchas.Cancha{},
			&usuarios.Usuario{},
		); err != nil {
			return err
		}
	}

	if err := db.Debug().AutoMigrate(
		&usuarios.Usuario{},
		&canchas.Cancha{}, &canchas.Horario{},
		&reservas.Reserva{}, &pagos.Pago{},
		&partidos.Partido{},
		&inscripcion.Inscripcion{},
		&sesion.Cuenta{},
		&eventos.Evento{}, &eventos.EventoProcesado{},
		&sagas.Saga{},
	); err != nil {
		return err
	}

	usuarios.Sembrar(db)
	// Las cuentas se siembran después: buscan su usuario de negocio por rol.
	if err := sesion.Sembrar(db); err != nil {
		return err
	}
	canchas.Sembrar(db, 1)     // ID 1: el administrador sembrado arriba
	reservas.Sembrar(db, 1, 2) // Cancha 1, Usuario cliente 2
	pagos.Sembrar(db, 2)       // Seña verificada de la reserva confirmada 2
	partidos.Sembrar(db, 2)    // Reserva 2: la reserva confirmada
	inscripcion.Sembrar(db, 1) // Partido 1
	return nil
}

// dependencias agrupa las piezas de la arquitectura de eventos.
type dependencias struct {
	db          *gorm.DB
	outbox      *eventos.PublicadorGORM
	sagasRepo   sagas.RepositorioGORM
	coordinador *sagas.Coordinador
}

// construirDependencias arma el bus, el outbox, la proyección de sagas y
// los suscriptores de cada contexto. También arranca el despachador, que
// es la única pieza que mueve eventos en segundo plano.
func construirDependencias(db *gorm.DB) *dependencias {
	repoOutbox := eventos.RepositorioOutboxGORM{DB: db}
	publicador := eventos.NuevoPublicador(db, repoOutbox)
	bus := eventos.NuevoBusEnMemoria()
	repoSagas := sagas.RepositorioGORM{DB: db}
	coordinador := sagas.NuevoCoordinador(repoSagas)

	// La proyección observa todos los eventos y actualiza la tabla sagas.
	sagas.NuevaProyeccion(repoSagas).Registrar(bus)

	reservas.NuevoSagas(
		reservas.RepositorioGORM{DB: db},
		eventos.NuevoProcesadorGORM(db, repoOutbox, "reservas"),
		publicador,
	).Registrar(bus)

	pagos.NuevoSagas(
		pagos.RepositorioGORM{DB: db},
		eventos.NuevoProcesadorGORM(db, repoOutbox, "pagos"),
		publicador,
	).Registrar(bus)

	partidos.NuevoSagas(
		partidos.RepositorioGORM{DB: db},
		eventos.NuevoProcesadorGORM(db, repoOutbox, "partidos"),
		publicador,
	).Registrar(bus)

	despachador := eventos.NuevoDespachador(db, repoOutbox, bus)
	despachador.AlAgotarIntentos = func(ctx context.Context, evento eventos.Evento, causa error) {
		motivo := "el evento agotó sus reintentos"
		if causa != nil {
			motivo = causa.Error()
		}
		fallo := eventos.NuevoEvento(
			eventos.TipoProcesamientoFallido, evento.AgregadoID,
			evento.SagaID, evento.CorrelationID, evento.ID,
			eventos.ProcesamientoFallido{
				EventoID:   evento.ID,
				TipoEvento: evento.Tipo,
				AgregadoID: evento.AgregadoID,
				SagaID:     evento.SagaID,
				Motivo:     motivo,
			})
		_ = publicador.Publicar(ctx, fallo)
	}
	go despachador.Ejecutar(context.Background())

	return &dependencias{
		db:          db,
		outbox:      publicador,
		sagasRepo:   repoSagas,
		coordinador: coordinador,
	}
}

// nuevoEnrutadorCon arma el servidor HTTP completo sobre dependencias ya
// construidas.
func nuevoEnrutadorCon(dep *dependencias, tokens *sesion.Tokens) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Registro)
	r.Use(middleware.Recuperacion)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		respuesta.Error(w, http.StatusNotFound, "ruta_inexistente", "la ruta no existe")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		respuesta.Error(w, http.StatusMethodNotAllowed,
			"metodo_no_permitido", "el método no está permitido en esta ruta")
	})

	(&canchas.Manejador{DB: dep.db}).Rutas(r, tokens)
	(&reservas.Manejador{
		DB:      dep.db,
		Eventos: dep.outbox,
		Sagas:   dep.coordinador,
	}).Rutas(r, tokens)
	(&partidos.Manejador{
		DB:      dep.db,
		Eventos: dep.outbox,
		Sagas:   dep.coordinador,
	}).Rutas(r, tokens)
	(&inscripcion.Manejador{DB: dep.db}).Rutas(r, tokens)
	(&sagas.Manejador{DB: dep.db, Repo: dep.sagasRepo}).Rutas(r, tokens)

	(&sesion.Manejador{
		Cuentas: sesion.RepositorioCuentasGORM{DB: dep.db},
		Tokens:  tokens,
	}).Rutas(r)

	return r
}

// nuevoEnrutador arma el servidor y sus dependencias. La prueba de
// integración usa esta misma función para ejecutar exactamente el mismo
// esquema, las mismas rutas y la misma coreografía.
func nuevoEnrutador(db *gorm.DB, tokens *sesion.Tokens) http.Handler {
	return nuevoEnrutadorCon(construirDependencias(db), tokens)
}

func main() {
	reset := flag.Bool("reset", false, "borra las tablas y arranca con la base vacía")
	flag.Parse()

	cfg, err := config.Cargar()
	if err != nil {
		log.Fatal("configuración: ", err)
	}

	db, err := conectarConReintentos(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("no se pudo conectar tras varios intentos: ", err)
	}

	if err := prepararBase(db, *reset); err != nil {
		log.Fatal("no se pudo preparar la base: ", err)
	}

	servidor := &http.Server{
		Addr:         ":" + cfg.Puerto,
		Handler:      nuevoEnrutador(db, sesion.NuevosTokens(cfg.ClaveFirma)),
		ReadTimeout:  cfg.TiempoEspera,
		WriteTimeout: cfg.TiempoEspera,
	}
	log.Println("escuchando en el puerto", cfg.Puerto)
	log.Fatal(servidor.ListenAndServe())
}
