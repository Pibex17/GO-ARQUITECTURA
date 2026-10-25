package sesion

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"canchas-sinteticas/internal/respuesta"
)

// Manejador tiene la única ruta de este paquete: iniciar sesión.
type Manejador struct {
	Cuentas RepositorioCuentas
	Tokens  *Tokens
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

	// El mismo mensaje para «no existe» y para «contraseña incorrecta»:
	// quien prueba nombres al azar no averigua cuáles existen.
	const noCoincide = "usuario o contraseña incorrectos"

	cuenta, err := m.Cuentas.PorUsuario(pedido.Usuario)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respuesta.Error(w, http.StatusUnauthorized,
			"credenciales_invalidas", noCoincide)
		return
	}
	if err != nil {
		respuesta.Error(w, http.StatusInternalServerError,
			"error_interno", "no se pudo iniciar sesión")
		return
	}

	if !ClaveCoincide(cuenta.ClaveHash, pedido.Clave) {
		respuesta.Error(w, http.StatusUnauthorized,
			"credenciales_invalidas", noCoincide)
		return
	}

	token, err := m.Tokens.Emitir(cuenta)
	if err != nil {
		respuesta.Error(w, http.StatusInternalServerError,
			"error_interno", "no se pudo iniciar sesión")
		return
	}
	respuesta.Exito(w, http.StatusOK, map[string]string{"token": token})
}
