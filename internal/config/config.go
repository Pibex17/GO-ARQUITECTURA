// Package config lee los valores que cambian de máquina en máquina.
// El código no sabe cuánto valen: solo sabe cómo se llaman.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// longitudMinimaClaveFirma exige al menos 32 bytes para la clave HS256.
// No se valida entropía ni fortaleza criptográfica, solo longitud y marcadores.
const longitudMinimaClaveFirma = 32

// marcadoresClaveFirma son valores de ejemplo que aparecen en la documentación
// y que nunca deben usarse para firmar tokens.
var marcadoresClaveFirma = map[string]struct{}{
	"cambie_esto":             {},
	"texto-largo-e-inventado": {},
}

// validarClaveFirma aplica la política de la clave JWT sin exponer su valor en
// los errores. La comparación de marcadores ignora mayúsculas y espacios
// exteriores para evitar que una variante trivial pase la validación.
func validarClaveFirma(clave string) error {
	normalizada := strings.TrimSpace(clave)
	if normalizada == "" {
		return errors.New("CLAVE_FIRMA no puede estar vacía ni contener solo espacios; " +
			"genere una clave aleatoria propia de al menos 32 bytes")
	}
	if _, esMarcador := marcadoresClaveFirma[strings.ToLower(normalizada)]; esMarcador {
		return errors.New("CLAVE_FIRMA no puede ser un marcador de ejemplo; " +
			"genere una clave aleatoria propia de al menos 32 bytes")
	}
	if len(normalizada) < longitudMinimaClaveFirma {
		return errors.New("CLAVE_FIRMA debe tener al menos 32 bytes; " +
			"genere una clave aleatoria propia")
	}
	return nil
}

// Config es todo lo que el servidor necesita saber del entorno donde corre.
type Config struct {
	DatabaseURL  string        // secreto: lleva la contraseña. Sin valor por defecto.
	ClaveFirma   string        // secreto: firma los tokens. Mínimo 32 bytes; sin marcadores.
	Puerto       string        // configuración: cambia por máquina. Por defecto 8080.
	TiempoEspera time.Duration // configuración: cuánto se espera antes de cortar. Por defecto 5s.
}

// Cargar lee .env si existe, después las variables del sistema.
// Se detiene con un error claro si falta un valor sin valor por defecto.
func Cargar() (Config, error) {
	// Si no hay .env no es un error: en el servidor real las variables
	// ya vienen puestas por el sistema (por ejemplo, en GitHub Actions).
	_ = godotenv.Load()

	var c Config

	c.DatabaseURL = os.Getenv("DATABASE_URL")
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("falta DATABASE_URL " +
			"(copie .env.example a .env y complete la contraseña)")
	}

	c.ClaveFirma = os.Getenv("CLAVE_FIRMA")
	if err := validarClaveFirma(c.ClaveFirma); err != nil {
		return c, err
	}

	c.Puerto = os.Getenv("PUERTO")
	if c.Puerto == "" {
		c.Puerto = "8080"
	}

	segundos := os.Getenv("TIEMPO_ESPERA_SEGUNDOS")
	if segundos == "" {
		segundos = "5"
	}
	n, err := strconv.Atoi(segundos)
	if err != nil || n <= 0 {
		return c, fmt.Errorf("TIEMPO_ESPERA_SEGUNDOS debe ser un entero "+
			"mayor que cero, llegó %q", segundos)
	}
	c.TiempoEspera = time.Duration(n) * time.Second

	return c, nil
}
