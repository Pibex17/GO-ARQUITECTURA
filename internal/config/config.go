// Package config lee los valores que cambian de máquina en máquina.
// El código no sabe cuánto valen: solo sabe cómo se llaman.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config es todo lo que el servidor necesita saber del entorno donde corre.
type Config struct {
	DatabaseURL  string        // secreto: lleva la contraseña. Sin valor por defecto.
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
