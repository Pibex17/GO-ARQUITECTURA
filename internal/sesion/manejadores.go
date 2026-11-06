package sesion

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"canchas-sinteticas/internal/respuesta"
)

// ServicioSesion es el contrato que el transporte necesita del caso de uso.
// El manejador depende de esta abstracción, no de la implementación concreta.
type ServicioSesion interface {
	Login(usuario, clave string) (string, error)
}

// Manejador es la capa de transporte del inicio de sesión: decodifica la
// petición, llama al servicio y traduce el resultado a HTTP.
type Manejador struct {
	Servicio ServicioSesion
}

// Rutas registra aquí las rutas de este paquete, para no llenar main.go.
func (m *Manejador) Rutas(r chi.Router) {
	r.Post("/login", m.Login) // pública: es la puerta para conseguir el token
}

// Login: POST /login con {"usuario": "...", "clave": "..."}.
// 400 si el JSON está roto · 401 si no coincide · 200 con el token.
func (m *Manejador) Login(w http.ResponseWriter, r *http.Request) {
	var pedido struct {
		Usuario string `json:"usuario"`
		Clave   string `json:"clave"`
	}
	if err := json.NewDecoder(r.Body).Decode(&pedido); err != nil {
		respuesta.Error(w, http.StatusBadRequest,
			"cuerpo_malformado", "el cuerpo debe ser JSON")
		return
	}

	token, err := m.Servicio.Login(pedido.Usuario, pedido.Clave)
	if errors.Is(err, ErrCredencialesInvalidas) {
		respuesta.Error(w, http.StatusUnauthorized,
			"credenciales_invalidas", ErrCredencialesInvalidas.Error())
		return
	}
	if err != nil {
		respuesta.Error(w, http.StatusInternalServerError,
			"error_interno", "no se pudo iniciar sesión")
		return
	}
	respuesta.Exito(w, http.StatusOK, map[string]string{"token": token})
}
