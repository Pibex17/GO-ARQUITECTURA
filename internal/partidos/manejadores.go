package partidos

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"canchas-sinteticas/internal/eventos"
	"canchas-sinteticas/internal/respuesta"
	"canchas-sinteticas/internal/sagas"
	"canchas-sinteticas/internal/sesion"
)

// Manejador guarda la conexión y las piezas sustituibles por dobles.
type Manejador struct {
	DB       *gorm.DB
	Repo     Repositorio
	Vinculos sesion.ResolutorVinculos
	Eventos  eventos.Publicador
	Sagas    sagas.Registro
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

func (m *Manejador) eventos() eventos.Publicador {
	if m.Eventos != nil {
		return m.Eventos
	}
	return eventos.NuevoPublicador(m.DB, eventos.RepositorioOutboxGORM{DB: m.DB})
}

func (m *Manejador) sagas() sagas.Registro {
	if m.Sagas != nil {
		return m.Sagas
	}
	return sagas.NuevoCoordinador(sagas.RepositorioGORM{DB: m.DB})
}

// Rutas: el cliente solicita el partido de su reserva y la creación se
// completa por coreografía; el detalle lo puede ver cualquier sesión.
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

// generarCodigoInvitacion genera un código único de 6 caracteres.
func generarCodigoInvitacion() string {
	const caracteres = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // sin 0/O, 1/I/L
	b := make([]byte, 6)
	rand.Read(b)
	for i := range b {
		b[i] = caracteres[int(b[i])%len(caracteres)]
	}
	return string(b)
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

	if entrada.ReservaID == 0 || entrada.Nombre == "" || entrada.CupoMaximo <= 0 {
		respuesta.Error(w, http.StatusUnprocessableEntity, "datos_invalidos", "reserva_id, nombre y cupo_maximo son obligatorios")
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

	// Verificar que la reserva existe y que pertenece al cliente autenticado.
	reserva, err := m.repositorio().ReservaPorID(entrada.ReservaID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respuesta.Error(w, http.StatusNotFound, "no_encontrado", "no existe una reserva con ese id")
			return
		}
		respuesta.Error(w, http.StatusInternalServerError, "error_base", "no se pudo leer la reserva")
		return
	}
	if reserva.ClienteID != usuario.ID {
		respuesta.Error(w, http.StatusForbidden, "permiso_denegado", "la reserva no es suya")
		return
	}

	// Verificar que la reserva está confirmada.
	if reserva.Estado != "confirmada" {
		respuesta.Error(w, http.StatusUnprocessableEntity, "reserva_no_confirmada", "solo se puede crear un partido desde una reserva confirmada")
		return
	}

	// Verificar que no exista ya un partido para esa reserva.
	if _, err := m.repositorio().PartidoPorReservaID(entrada.ReservaID); err == nil {
		respuesta.Error(w, http.StatusConflict, "partido_existente", "ya existe un partido para esa reserva")
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		respuesta.Error(w, http.StatusInternalServerError, "error_base", "no se pudo comprobar el partido")
		return
	}

	saga, err := m.sagas().Asegurar(r.Context(), sagas.TipoCicloReserva, entrada.ReservaID)
	if err != nil {
		respuesta.Error(w, http.StatusInternalServerError, "error_base", "no se pudo preparar la saga del partido")
		return
	}

	evento := eventos.NuevoEvento(
		eventos.TipoSolicitudPartidoRecibida, entrada.ReservaID,
		saga.ID, eventos.NuevoID(), "",
		eventos.SolicitudPartidoRecibida{
			ReservaID:  entrada.ReservaID,
			ClienteID:  usuario.ID,
			Nombre:     entrada.Nombre,
			CupoMaximo: entrada.CupoMaximo,
		})
	if err := m.eventos().Publicar(r.Context(), evento); err != nil {
		respuesta.Error(w, http.StatusInternalServerError, "error_base", "no se pudo publicar la solicitud del partido")
		return
	}

	w.Header().Set("Location", "/sagas/"+saga.ID)
	respuesta.Exito(w, http.StatusAccepted, map[string]any{
		"saga_id": saga.ID,
		"estado":  saga.Estado,
		"paso":    saga.Paso,
	})
}

// GET /partidos/{id}
func (m *Manejador) obtener(w http.ResponseWriter, r *http.Request) {
	id, ok := leerID(w, r)
	if !ok {
		return
	}

	partido, err := m.repositorio().PartidoPorID(id)
	if err != nil {
		respuesta.Error(w, http.StatusNotFound, "no_encontrado", "no existe un partido con ese id")
		return
	}

	inscripciones, _ := m.repositorio().ContarInscripciones(partido.ID)

	respuesta.Exito(w, http.StatusOK, map[string]any{
		"partido":         partido,
		"inscriptos":      inscripciones,
		"cupos_restantes": partido.CupoMaximo - int(inscripciones),
	})
}
