package config

import (
	"strings"
	"testing"
)

func TestCargarSinDatabaseURLFalla(t *testing.T) {
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
	t.Setenv("DATABASE_URL", "host=localhost") // válida solo durante esta prueba
	t.Setenv("CLAVE_FIRMA", "")
	_, err := Cargar()
	if err == nil {
		t.Fatal("se esperaba un error por CLAVE_FIRMA vacía y no hubo")
	}
	if !strings.Contains(err.Error(), "CLAVE_FIRMA") {
		t.Fatalf("el error no dice qué falta: %v", err)
	}
}
