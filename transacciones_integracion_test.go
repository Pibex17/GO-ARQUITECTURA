//go:build integration

package main

import (
	"errors"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"canchas-sinteticas/internal/almacen"
	"canchas-sinteticas/internal/reservas"
)

// TestTransaccionReservaPagoEsAtomica fuerza el fallo de la segunda
// escritura del caso de uso: la reserva se actualiza y el pago con monto 0
// viola la restricción monto > 0. La transacción debe dejar la reserva en
// "pendiente" y sin pagos, tanto con GORM como con sqlc.
//
// Ejecución:
//
//	TEST_DATABASE_URL="host=localhost user=postgres password=postgres dbname=canchas_test port=5432 sslmode=disable" \
//	  go test -tags=integration . -run TestTransaccionReservaPagoEsAtomica -v -count=1
func TestTransaccionReservaPagoEsAtomica(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("defina TEST_DATABASE_URL para ejecutar la prueba de integración")
	}

	db, err := gorm.Open(postgres.Open(dsn),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("no se pudo conectar a TEST_DATABASE_URL: %v", err)
	}

	casos := []struct {
		nombre string
		repo   func(*gorm.DB) reservas.Repositorio
	}{
		{
			nombre: "gorm",
			repo: func(db *gorm.DB) reservas.Repositorio {
				return reservas.RepositorioGORM{DB: db}
			},
		},
		{
			nombre: "sqlc",
			repo: func(db *gorm.DB) reservas.Repositorio {
				conexion, err := db.DB()
				if err != nil {
					t.Fatalf("no se pudo obtener *sql.DB: %v", err)
				}
				return almacen.NuevoReservasSqlc(reservas.RepositorioGORM{DB: db}, conexion)
			},
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			if err := prepararBase(db, true); err != nil {
				t.Fatalf("no se pudo preparar la base: %v", err)
			}

			// La cancha sembrada vale 25. Un precio 0 hace que la seña
			// calculada sea 0 y que PostgreSQL rechace el pago.
			if err := db.Exec("UPDATE canchas SET precio_hora = 0 WHERE id = 1").Error; err != nil {
				t.Fatalf("no se pudo forzar el precio de la cancha: %v", err)
			}

			servicio := reservas.NuevoServicio(caso.repo(db))
			_, err := servicio.CambiarEstado(
				1, 1, "pago_por_verificar", "efectivo",
				"https://ejemplo.invalid/comprobante.jpg",
			)
			t.Logf("adaptador=%s error=%v", caso.nombre, err)
			if err == nil {
				t.Fatal("se esperaba que la restricción monto > 0 hiciera fallar el pago")
			}
			if !errors.Is(err, reservas.ErrPagoNoRegistrado) {
				t.Fatalf("se esperaba ErrPagoNoRegistrado, llegó: %v", err)
			}

			var estado string
			if err := db.Raw("SELECT estado FROM reservas WHERE id = ?", 1).
				Scan(&estado).Error; err != nil {
				t.Fatalf("no se pudo leer el estado de la reserva: %v", err)
			}
			var pagos int64
			if err := db.Raw("SELECT count(*) FROM pagos WHERE reserva_id = ?", 1).
				Scan(&pagos).Error; err != nil {
				t.Fatalf("no se pudo contar los pagos: %v", err)
			}

			t.Logf("adaptador=%s reserva.estado=%q pagos=%d", caso.nombre, estado, pagos)
			if estado != "pendiente" {
				t.Fatalf("la reserva quedó en %q; la transacción debía revertirla a pendiente", estado)
			}
			if pagos != 0 {
				t.Fatalf("quedaron %d pagos; la transacción debía revertirlos", pagos)
			}
		})
	}
}
