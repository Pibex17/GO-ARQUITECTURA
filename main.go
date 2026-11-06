package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"canchas-sinteticas/internal/almacen"
	"canchas-sinteticas/internal/canchas"
	"canchas-sinteticas/internal/config"
	"canchas-sinteticas/internal/inscripcion"
	"canchas-sinteticas/internal/middleware"
	"canchas-sinteticas/internal/partidos"
	"canchas-sinteticas/internal/reservas"
	"canchas-sinteticas/internal/respuesta"
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
// las tablas, migra el esquema y siembra los datos de ejemplo. La usa
// main() y también la prueba de integración, para que ambas ejecuten
// exactamente el mismo esquema y los mismos datos.
func prepararBase(db *gorm.DB, reset bool) error {
	if reset {
		// primero las tablas del lado de los muchos, después las del uno
		if err := db.Migrator().DropTable(
			&sesion.Cuenta{},
			&inscripcion.Inscripcion{}, &partidos.Partido{},
			&reservas.Pago{}, &reservas.Reserva{},
			&canchas.Horario{}, &canchas.Cancha{},
			&usuarios.Usuario{},
		); err != nil {
			return err
		}
	}

	if err := db.Debug().AutoMigrate(
		&usuarios.Usuario{},
		&canchas.Cancha{}, &canchas.Horario{},
		&reservas.Reserva{}, &reservas.Pago{},
		&partidos.Partido{},
		&inscripcion.Inscripcion{},
		&sesion.Cuenta{},
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
	partidos.Sembrar(db, 2)    // Reserva 2: la reserva confirmada
	inscripcion.Sembrar(db, 1) // Partido 1
	return nil
}

// repositorioReservas elige el adaptador sin cambiar el servicio ni sus
// pruebas. Por defecto se conserva GORM; con ADAPTADOR_RESERVAS=sqlc se usa
// el adaptador sqlc, que resuelve sus propias transacciones.
func repositorioReservas(db *gorm.DB) reservas.Repositorio {
	if os.Getenv("ADAPTADOR_RESERVAS") != "sqlc" {
		return reservas.RepositorioGORM{DB: db}
	}

	conexion, err := db.DB()
	if err != nil {
		log.Printf("no se pudo obtener la conexión *sql.DB: %v; se usa GORM", err)
		return reservas.RepositorioGORM{DB: db}
	}
	return almacen.NuevoReservasSqlc(reservas.RepositorioGORM{DB: db}, conexion)
}

// nuevoEnrutador arma el servidor HTTP completo. No abre la base: recibe
// la conexión ya lista y el emisor de tokens, igual que en main().
func nuevoEnrutador(db *gorm.DB, tokens *sesion.Tokens) http.Handler {
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

	// Composición explícita: se construyen los repositorios GORM, luego los
	// servicios con esas interfaces y al final los manejadores HTTP. Las
	// dependencias apuntan Transporte → Servicio → Repositorio.
	vinculos := sesion.Vinculos{Repo: sesion.RepositorioVinculosGORM{DB: db}}

	// El guardia de sesión se registra antes de las rutas de negocio:
	// todas lo reciben y deciden su nivel (sesión, solo cliente o solo
	// administrador). POST /login se registra aparte, sin guardia.
	(&canchas.Manejador{
		Servicio: canchas.NuevoServicio(canchas.RepositorioGORM{DB: db}),
		Vinculos: vinculos,
	}).Rutas(r, tokens)
	(&reservas.Manejador{
		Servicio: reservas.NuevoServicio(repositorioReservas(db)),
		Vinculos: vinculos,
	}).Rutas(r, tokens)
	(&partidos.Manejador{
		Servicio: partidos.NuevoServicio(partidos.RepositorioGORM{DB: db}),
		Vinculos: vinculos,
	}).Rutas(r, tokens)
	(&inscripcion.Manejador{
		Servicio: inscripcion.NuevoServicio(inscripcion.RepositorioGORM{DB: db}),
		Vinculos: vinculos,
	}).Rutas(r, tokens)

	// Sesión: POST /login es la única ruta pública.
	(&sesion.Manejador{
		Servicio: sesion.NuevoServicio(
			sesion.RepositorioCuentasGORM{DB: db}, tokens),
	}).Rutas(r)

	return r
}

func main() {
	reset := flag.Bool("reset", false, "borra las tablas y arranca con la base vacía")
	flag.Parse()

	// 1. Leer el entorno. Si falta algo, se detiene aquí con un mensaje claro.
	cfg, err := config.Cargar()
	if err != nil {
		log.Fatal("configuración: ", err)
	}

	// 2. Conectar, con reintentos (hace falta al arrancar con Docker Compose).
	db, err := conectarConReintentos(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("no se pudo conectar tras varios intentos: ", err)
	}

	// 3. Migrar y sembrar.
	if err := prepararBase(db, *reset); err != nil {
		log.Fatal("no se pudo preparar la base: ", err)
	}

	// 4. Servidor: el puerto y el tiempo de espera se leen una sola vez, de config.
	servidor := &http.Server{
		Addr:         ":" + cfg.Puerto,
		Handler:      nuevoEnrutador(db, sesion.NuevosTokens(cfg.ClaveFirma)),
		ReadTimeout:  cfg.TiempoEspera,
		WriteTimeout: cfg.TiempoEspera,
	}
	log.Println("escuchando en el puerto", cfg.Puerto)
	log.Fatal(servidor.ListenAndServe())
}
