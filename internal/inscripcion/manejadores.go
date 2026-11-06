package inscripcion

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"canchas-sinteticas/internal/respuesta"
	"canchas-sinteticas/internal/sesion"
)

// ServicioInscripciones es el contrato que el transporte necesita del caso
// de uso. El manejador depende de esta abstracción, no de la implementación.
type ServicioInscripciones interface {
	Crear(partidoID, jugadorID uint, entrada EntradaCrear) (Inscripcion, error)
}

// Manejador es la capa de transporte: traduce HTTP al servicio de
// inscripciones. No conoce GORM ni crea repositorios.
type Manejador struct {
	Servicio ServicioInscripciones
	Vinculos sesion.ResolutorVinculos
}

func (m *Manejador) vinculos() sesion.ResolutorVinculos {
	if m.Vinculos != nil {
		return m.Vinculos
	}
	return sesion.Vinculos{}
}

// Rutas: inscribirse es solo del cliente según la tabla de roles de la
// ficha Hito 1 (sección 6); el administrador no se inscribe.
func (m *Manejador) Rutas(r chi.Router, tokens *sesion.Tokens) {
	conSesion := r.With(tokens.Autenticar)
	soloCliente := conSesion.With(sesion.SoloRol(sesion.RolCliente))

	soloCliente.Post("/partidos/{id}/inscripciones", m.crear)
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
	case errors.Is(err, ErrNoEncontrado):
		respuesta.Error(w, http.StatusNotFound,
			"no_encontrado", ErrNoEncontrado.Error())
	case errors.Is(err, ErrCodigoInvalido):
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"codigo_invalido", ErrCodigoInvalido.Error())
	case errors.Is(err, ErrYaInscripto):
		respuesta.Error(w, http.StatusConflict,
			"ya_inscripto", ErrYaInscripto.Error())
	case errors.Is(err, ErrPartidoLleno):
		respuesta.Error(w, http.StatusConflict,
			"partido_lleno", ErrPartidoLleno.Error())
	case errors.Is(err, errLeyendoPartido):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", errLeyendoPartido.Error())
	case errors.Is(err, errComprobandoInscripcion):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", errComprobandoInscripcion.Error())
	case errors.Is(err, errContandoInscripciones):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", errContandoInscripciones.Error())
	case errors.Is(err, errGuardandoInscripcion):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", errGuardandoInscripcion.Error())
	default:
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo procesar la solicitud")
	}
}

// POST /partidos/{id}/inscripciones
func (m *Manejador) crear(w http.ResponseWriter, r *http.Request) {
	id, ok := leerID(w, r)
	if !ok {
		return
	}

	// El jugador no viene del cuerpo: se deriva de la cuenta autenticada.
	// Si el cuerpo trae JugadorID, se ignora.
	var entrada struct {
		CodigoInvitacion string
	}
	if err := json.NewDecoder(r.Body).Decode(&entrada); err != nil {
		respuesta.Error(w, http.StatusBadRequest, "cuerpo_malformado", "el cuerpo debe ser JSON")
		return
	}

	s, ok := sesionActual(w, r)
	if !ok {
		return
	}
	usuario, err := m.vinculos().UsuarioDe(s)
	if err != nil {
		responderSinVinculo(w)
		return
	}

	nueva, err := m.Servicio.Crear(id, usuario.ID, EntradaCrear{
		CodigoInvitacion: entrada.CodigoInvitacion,
	})
	if err != nil {
		responderError(w, err)
		return
	}
	respuesta.Exito(w, http.StatusCreated, nueva)
}

func leerID(w http.ResponseWriter, r *http.Request) (uint, bool) {
	n, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || n <= 0 {
		respuesta.Error(w, http.StatusBadRequest, "id_invalido", "el id debe ser un entero positivo")
		return 0, false
	}
	return uint(n), true
}
