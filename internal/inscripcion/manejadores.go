package inscripcion

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"canchas-sinteticas/internal/respuesta"
	"canchas-sinteticas/internal/sesion"
)

// Manejador guarda la conexión y las piezas sustituibles por dobles.
type Manejador struct {
	DB       *gorm.DB
	Repo     Repositorio
	Vinculos sesion.ResolutorVinculos
}

func (m *Manejador) repositorio() Repositorio {
	if m.Repo != nil {
		return m.Repo
	}
	return RepositorioGORM{DB: m.DB}
}

func (m *Manejador) vinculos() sesion.ResolutorVinculos {
	if m.Vinculos != nil {
		return m.Vinculos
	}
	return sesion.Vinculos{Repo: sesion.RepositorioVinculosGORM{DB: m.DB}}
}

// Rutas: inscribirse es solo del cliente; el administrador no se inscribe.
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

	if entrada.CodigoInvitacion == "" {
		respuesta.Error(w, http.StatusUnprocessableEntity, "datos_invalidos", "codigo_invitacion es obligatorio")
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

	// Verificar que el partido existe.
	partido, err := m.repositorio().PartidoPorID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respuesta.Error(w, http.StatusNotFound, "no_encontrado", "no existe un partido con ese id")
			return
		}
		respuesta.Error(w, http.StatusInternalServerError, "error_base", "no se pudo leer el partido")
		return
	}

	// Verificar que el código coincida.
	if partido.CodigoInvitacion != entrada.CodigoInvitacion {
		respuesta.Error(w, http.StatusUnprocessableEntity, "codigo_invalido", "el código de invitación no es válido")
		return
	}

	// Verificar que el jugador autenticado no esté ya inscrito.
	if _, err := m.repositorio().InscripcionPorPartidoYJugador(id, usuario.ID); err == nil {
		respuesta.Error(w, http.StatusConflict, "ya_inscripto", "el jugador ya está inscrito en este partido")
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		respuesta.Error(w, http.StatusInternalServerError, "error_base", "no se pudo comprobar la inscripción")
		return
	}

	// Verificar que haya cupo.
	inscripciones, err := m.repositorio().ContarInscripciones(id)
	if err != nil {
		respuesta.Error(w, http.StatusInternalServerError, "error_base", "no se pudo contar los inscriptos")
		return
	}
	if int(inscripciones) >= partido.CupoMaximo {
		respuesta.Error(w, http.StatusConflict, "partido_lleno", "el partido ya no tiene cupos disponibles")
		return
	}

	nueva := Inscripcion{
		PartidoID: id,
		JugadorID: usuario.ID, // el dueño es el usuario vinculado a la cuenta
		CreadoEn:  time.Now(),
	}

	if err := m.repositorio().CrearInscripcion(&nueva); err != nil {
		respuesta.Error(w, http.StatusInternalServerError, "error_base", "no se pudo guardar la inscripción")
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
