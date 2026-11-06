package partidos

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"canchas-sinteticas/internal/respuesta"
	"canchas-sinteticas/internal/sesion"
)

// ServicioPartidos es el contrato que el transporte necesita del caso de uso.
// El manejador depende de esta abstracción, no de la implementación concreta.
type ServicioPartidos interface {
	Crear(clienteID uint, entrada EntradaCrear) (Partido, error)
	Obtener(id uint) (Detalle, error)
}

// Manejador es la capa de transporte: traduce HTTP al servicio de partidos.
// No conoce GORM ni crea repositorios.
type Manejador struct {
	Servicio ServicioPartidos
	Vinculos sesion.ResolutorVinculos
}

func (m *Manejador) vinculos() sesion.ResolutorVinculos {
	if m.Vinculos != nil {
		return m.Vinculos
	}
	return sesion.Vinculos{}
}

// Rutas: el cliente crea el partido de su reserva; el detalle lo puede
// ver cualquier sesión (ficha Hito 1, sección 6).
func (m *Manejador) Rutas(r chi.Router, tokens *sesion.Tokens) {
	conSesion := r.With(tokens.Autenticar)
	soloCliente := conSesion.With(sesion.SoloRol(sesion.RolCliente))

	soloCliente.Post("/partidos", m.crear)
	conSesion.Get("/partidos/{id}", m.obtener)
}

func leerID(w http.ResponseWriter, r *http.Request) (uint, bool) {
	n, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || n <= 0 {
		respuesta.Error(w, http.StatusBadRequest, "id_invalido", "el id debe ser un entero positivo")
		return 0, false
	}
	return uint(n), true
}

func sesionActual(w http.ResponseWriter, r *http.Request) (sesion.Sesion, bool) {
	s, ok := sesion.De(r)
	if !ok {
		respuesta.Error(w, http.StatusUnauthorized,
			"falta_token", "falta el token: inicie sesión en POST /login")
		return sesion.Sesion{}, false
	}
	return s, true
}

func responderSinVinculo(w http.ResponseWriter) {
	respuesta.Error(w, http.StatusForbidden,
		"cuenta_sin_vinculo", "la cuenta no está vinculada a un usuario de negocio")
}

func responderError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrDatosInvalidos):
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"datos_invalidos", ErrDatosInvalidos.Error())
	case errors.Is(err, ErrReservaNoEncontrada):
		respuesta.Error(w, http.StatusNotFound,
			"no_encontrado", ErrReservaNoEncontrada.Error())
	case errors.Is(err, ErrReservaAjena):
		respuesta.Error(w, http.StatusForbidden,
			"permiso_denegado", ErrReservaAjena.Error())
	case errors.Is(err, ErrReservaNoConfirmada):
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"reserva_no_confirmada", ErrReservaNoConfirmada.Error())
	case errors.Is(err, ErrPartidoExistente):
		respuesta.Error(w, http.StatusConflict,
			"partido_existente", ErrPartidoExistente.Error())
	case errors.Is(err, ErrNoEncontrado):
		respuesta.Error(w, http.StatusNotFound,
			"no_encontrado", ErrNoEncontrado.Error())
	case errors.Is(err, errLeyendoReserva):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", errLeyendoReserva.Error())
	case errors.Is(err, errComprobandoPartido):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", errComprobandoPartido.Error())
	case errors.Is(err, errGenerandoCodigo):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", errGenerandoCodigo.Error())
	case errors.Is(err, errGuardandoPartido):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", errGuardandoPartido.Error())
	case errors.Is(err, errLeyendoPartido):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", errLeyendoPartido.Error())
	case errors.Is(err, errContandoInscripcion):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", errContandoInscripcion.Error())
	default:
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo procesar la solicitud")
	}
}

// POST /partidos
func (m *Manejador) crear(w http.ResponseWriter, r *http.Request) {
	var entrada struct {
		ReservaID  uint
		Nombre     string
		CupoMaximo int
	}
	if err := json.NewDecoder(r.Body).Decode(&entrada); err != nil {
		respuesta.Error(w, http.StatusBadRequest, "cuerpo_malformado", "el cuerpo debe ser JSON")
		return
	}

	// El dueño del partido es el cliente autenticado, no un ID del cuerpo.
	s, ok := sesionActual(w, r)
	if !ok {
		return
	}
	usuario, err := m.vinculos().UsuarioDe(s)
	if err != nil {
		responderSinVinculo(w)
		return
	}

	partido, err := m.Servicio.Crear(usuario.ID, EntradaCrear{
		ReservaID:  entrada.ReservaID,
		Nombre:     entrada.Nombre,
		CupoMaximo: entrada.CupoMaximo,
	})
	if err != nil {
		responderError(w, err)
		return
	}
	respuesta.Exito(w, http.StatusCreated, partido)
}

// GET /partidos/{id}
func (m *Manejador) obtener(w http.ResponseWriter, r *http.Request) {
	id, ok := leerID(w, r)
	if !ok {
		return
	}

	detalle, err := m.Servicio.Obtener(id)
	if err != nil {
		responderError(w, err)
		return
	}
	respuesta.Exito(w, http.StatusOK, detalle)
}
