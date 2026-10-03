package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"canchas-sinteticas/internal/canchas"
	"canchas-sinteticas/internal/config"
	"canchas-sinteticas/internal/inscripcion"
	"canchas-sinteticas/internal/middleware"
	"canchas-sinteticas/internal/partidos"
	"canchas-sinteticas/internal/reservas"
	"canchas-sinteticas/internal/respuesta"
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

	if *reset {
		// primero las tablas del lado de los muchos, después las del uno
		db.Migrator().DropTable(
			&inscripcion.Inscripcion{}, &partidos.Partido{},
			&reservas.Pago{}, &reservas.Reserva{},
			&canchas.Horario{}, &canchas.Cancha{},
			&usuarios.Usuario{},
		)
	}

	// 3. Migrar.
	err = db.Debug().AutoMigrate(
		&usuarios.Usuario{},
		&canchas.Cancha{}, &canchas.Horario{},
		&reservas.Reserva{}, &reservas.Pago{},
		&partidos.Partido{},
		&inscripcion.Inscripcion{},
	)
	if err != nil {
		log.Fatal("no se pudo migrar: ", err)
	}

	// 4. Sembrar datos de ejemplo (solo si las tablas están vacías).
	usuarios.Sembrar(db)
	canchas.Sembrar(db, 1) // ID 1: el administrador sembrado arriba
	reservas.Sembrar(db, 1, 2) // Cancha 1, Usuario cliente 2
	partidos.Sembrar(db, 2) // Reserva 2: la reserva confirmada
	inscripcion.Sembrar(db, 1) // Partido 1

	// 5. Enrutador y middleware propios (semana 2).
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

	(&canchas.Manejador{DB: db}).Rutas(r)
	(&reservas.Manejador{DB: db}).Rutas(r)
	(&partidos.Manejador{DB: db}).Rutas(r)
	(&inscripcion.Manejador{DB: db}).Rutas(r)

	// 6. Servidor: el puerto y el tiempo de espera se leen una sola vez, de config.
	servidor := &http.Server{
		Addr:         ":" + cfg.Puerto,
		Handler:      r,
		ReadTimeout:  cfg.TiempoEspera,
		WriteTimeout: cfg.TiempoEspera,
	}
	log.Println("escuchando en el puerto", cfg.Puerto)
	log.Fatal(servidor.ListenAndServe())
}
