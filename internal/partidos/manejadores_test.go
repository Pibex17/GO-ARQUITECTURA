package partidos

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// enrutador arma el servidor de prueba SIN base de datos (DB: nil).
// Sirve porque todas las pruebas de este archivo responden ANTES
// de tocar la base de datos.
func enrutador() http.Handler {
	r := chi.NewRouter()
	(&Manejador{DB: nil}).Rutas(r)
	return r
}

// TestCrear: una función, varias filas. Cada fila es un caso.
func TestCrear(t *testing.T) {
	casos := []struct {
		nombre string
		cuerpo string
		codigo int
	}{
		{
			nombre: "JSON roto responde 400",
			cuerpo: `{"ReservaID": 1`,
			codigo: http.StatusBadRequest,
		},
		{
			nombre: "faltan datos obligatorios responde 422",
			cuerpo: `{"ReservaID": 1}`,
			codigo: http.StatusUnprocessableEntity,
		},
		{
			nombre: "cupo_maximo cero responde 422",
			cuerpo: `{"ReservaID": 1, "Nombre": "Partido", "CupoMaximo": 0}`,
			codigo: http.StatusUnprocessableEntity,
		},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			peticion := httptest.NewRequest(http.MethodPost, "/partidos",
				strings.NewReader(caso.cuerpo))
			grabadora := httptest.NewRecorder()
			enrutador().ServeHTTP(grabadora, peticion)
			if grabadora.Code != caso.codigo {
				t.Fatalf("se esperaba %d y llegó %d con cuerpo %s",
					caso.codigo, grabadora.Code, grabadora.Body.String())
			}
		})
	}
}

// TestObtenerConIDQueNoEsNumeroResponde400: el id de la ruta se valida
// antes de consultar la base de datos.
func TestObtenerConIDQueNoEsNumeroResponde400(t *testing.T) {
	peticion := httptest.NewRequest(http.MethodGet, "/partidos/abc", nil)
	grabadora := httptest.NewRecorder()
	enrutador().ServeHTTP(grabadora, peticion)
	if grabadora.Code != http.StatusBadRequest {
		t.Fatalf("se esperaba 400 y llegó %d", grabadora.Code)
	}
}
