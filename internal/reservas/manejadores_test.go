package reservas // el MISMO paquete que manejadores.go

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// enrutador arma el servidor de prueba SIN base de datos (DB: nil).
// Sirve porque todas las pruebas de este archivo responden ANTES
// de la línea "desde aquí hace falta la base de datos".
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
			cuerpo: `{"CanchaID": 1`,
			codigo: http.StatusBadRequest,
		},
		{
			// Regla propia de este negocio (2b): faltan fecha, hora_inicio
			// y hora_fin. Responde antes de tocar m.DB.
			nombre: "faltan datos obligatorios responde 422",
			cuerpo: `{"CanchaID": 1, "ClienteID": 2}`,
			codigo: http.StatusUnprocessableEntity,
		},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			peticion := httptest.NewRequest(http.MethodPost, "/reservas",
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

// TestCambiarEstado: la validación del estado pedido responde antes de
// leer la reserva de la base de datos.
func TestCambiarEstado(t *testing.T) {
	casos := []struct {
		nombre string
		cuerpo string
		codigo int
	}{
		{
			nombre: "JSON roto responde 400",
			cuerpo: `{"NuevoEstado": "confirmada"`,
			codigo: http.StatusBadRequest,
		},
		{
			nombre: "estado inventado responde 422",
			cuerpo: `{"NuevoEstado": "volando"}`,
			codigo: http.StatusUnprocessableEntity,
		},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			peticion := httptest.NewRequest(http.MethodPatch, "/reservas/1/estado",
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

// El id de la ruta se valida antes de consultar la base de datos.
func TestCambiarEstadoConIDQueNoEsNumeroResponde400(t *testing.T) {
	peticion := httptest.NewRequest(http.MethodPatch, "/reservas/abc/estado",
		strings.NewReader(`{"NuevoEstado": "confirmada"}`))
	grabadora := httptest.NewRecorder()
	enrutador().ServeHTTP(grabadora, peticion)
	if grabadora.Code != http.StatusBadRequest {
		t.Fatalf("se esperaba 400 y llegó %d", grabadora.Code)
	}
}
