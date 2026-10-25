package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

// entornoAislado evita que un .env local o variables del sistema cambien el
// resultado. t.TempDir() no contiene .env, así que godotenv.Load no carga nada.
func entornoAislado(t *testing.T) {
	t.Helper()

	anterior, err := os.Getwd()
	if err != nil {
		t.Fatalf("no se pudo obtener el directorio actual: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("no se pudo cambiar a un directorio temporal: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(anterior); err != nil {
			t.Errorf("no se pudo restaurar el directorio de trabajo: %v", err)
		}
	})

	t.Setenv("DATABASE_URL", "host=localhost")
	t.Setenv("PUERTO", "")
	t.Setenv("TIEMPO_ESPERA_SEGUNDOS", "5")
}

// fijarClaveFirma distingue entre una variable ausente y una variable vacía.
func fijarClaveFirma(t *testing.T, valor *string) {
	t.Helper()

	if valor != nil {
		t.Setenv("CLAVE_FIRMA", *valor)
		return
	}

	anterior, existia := os.LookupEnv("CLAVE_FIRMA")
	if err := os.Unsetenv("CLAVE_FIRMA"); err != nil {
		t.Fatalf("no se pudo quitar CLAVE_FIRMA: %v", err)
	}
	t.Cleanup(func() {
		var err error
		if existia {
			err = os.Setenv("CLAVE_FIRMA", anterior)
		} else {
			err = os.Unsetenv("CLAVE_FIRMA")
		}
		if err != nil {
			t.Errorf("no se pudo restaurar CLAVE_FIRMA: %v", err)
		}
	})
}

func TestCargarSinDatabaseURLFalla(t *testing.T) {
	entornoAislado(t)
	t.Setenv("DATABASE_URL", "") // vacía solo durante esta prueba
	_, err := Cargar()
	if err == nil {
		t.Fatal("se esperaba un error por DATABASE_URL vacía y no hubo")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("el error no dice qué falta: %v", err)
	}
}

func TestCargarSinClaveFirmaFalla(t *testing.T) {
	entornoAislado(t)
	t.Setenv("CLAVE_FIRMA", "")
	_, err := Cargar()
	if err == nil {
		t.Fatal("se esperaba un error por CLAVE_FIRMA vacía y no hubo")
	}
	if !strings.Contains(err.Error(), "CLAVE_FIRMA") {
		t.Fatalf("el error no dice qué falta: %v", err)
	}
}

func TestCargarClaveFirmaInvalida(t *testing.T) {
	casos := []struct {
		nombre string
		valor  *string
	}{
		{"ausente", nil},
		{"vacía", texto("")},
		{"solo espacios", texto(" \t\r\n")},
		{"marcador CAMBIE_ESTO", texto("CAMBIE_ESTO")},
		{"marcador con espacios y minúsculas", texto("  cambie_esto  ")},
		{"marcador de la documentación", texto("texto-largo-e-inventado")},
		{"demasiado corta", texto(strings.Repeat("x", 31))},
	}

	for _, caso := range casos {
		caso := caso
		t.Run(caso.nombre, func(t *testing.T) {
			entornoAislado(t)
			fijarClaveFirma(t, caso.valor)

			_, err := Cargar()
			if err == nil {
				t.Fatal("se esperaba un error por CLAVE_FIRMA inválida y no hubo")
			}
			if !strings.Contains(err.Error(), "CLAVE_FIRMA") {
				t.Fatalf("el error no identifica la variable: %v", err)
			}
			if caso.valor != nil {
				contenido := strings.TrimSpace(*caso.valor)
				if contenido != "" && strings.Contains(err.Error(), contenido) {
					t.Fatalf("el error imprime la clave configurada: %v", err)
				}
			}
		})
	}
}

func TestCargarClaveFirmaValida(t *testing.T) {
	entornoAislado(t)
	valida := strings.Repeat("v", longitudMinimaClaveFirma)
	t.Setenv("CLAVE_FIRMA", valida)

	cfg, err := Cargar()
	if err != nil {
		t.Fatalf("no se esperaba un error con una clave válida: %v", err)
	}
	if cfg.ClaveFirma != valida {
		t.Fatal("la clave válida no se conservó en la configuración")
	}
	if cfg.DatabaseURL != "host=localhost" {
		t.Fatalf("DATABASE_URL cambió: %q", cfg.DatabaseURL)
	}
	if cfg.Puerto != "8080" {
		t.Fatalf("el puerto por defecto cambió: %q", cfg.Puerto)
	}
	if cfg.TiempoEspera != 5*time.Second {
		t.Fatalf("el tiempo de espera por defecto cambió: %v", cfg.TiempoEspera)
	}
}

func texto(s string) *string {
	return &s
}
