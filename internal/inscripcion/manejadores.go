package inscripcion

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"canchas-sinteticas/internal/respuesta"
	"canchas-sinteticas/internal/usuarios"
)

type Manejador struct{ DB *gorm.DB }

func (m *Manejador) Rutas(r chi.Router) {
	r.Post("/partidos/{id}/inscripciones", m.crear)
}

// POST /partidos/{id}/inscripciones
func (m *Manejador) crear(w http.ResponseWriter, r *http.Request) {
	id, ok := leerID(w, r)
	if !ok {
		return
	}

	var entrada struct {
		CodigoInvitacion string
		JugadorID        uint
	}
	if err := json.NewDecoder(r.Body).Decode(&entrada); err != nil {
		respuesta.Error(w, http.StatusBadRequest, "cuerpo_malformado", "el cuerpo debe ser JSON")
		return
	}

	if entrada.CodigoInvitacion == "" || entrada.JugadorID == 0 {
		respuesta.Error(w, http.StatusUnprocessableEntity, "datos_invalidos", "codigo_invitacion y jugador_id son obligatorios")
		return
	}

	// Verificar que el partido existe
	var partido struct {
		ID               uint
		CodigoInvitacion string
		CupoMaximo       int
	}
	if err := m.DB.Table("partidos").First(&partido, id).Error; err != nil {
		respuesta.Error(w, http.StatusNotFound, "no_encontrado", "no existe un partido con ese id")
		return
	}

	// Verificar que el código coincida
	if partido.CodigoInvitacion != entrada.CodigoInvitacion {
		respuesta.Error(w, http.StatusUnprocessableEntity, "codigo_invalido", "el código de invitación no es válido")
		return
	}

	// Verificar que el jugador existe
	var jugador usuarios.Usuario
	if err := m.DB.First(&jugador, entrada.JugadorID).Error; err != nil {
		respuesta.Error(w, http.StatusNotFound, "no_encontrado", "no existe un usuario con ese id")
		return
	}

	// Verificar que no esté ya inscrito
	var existente Inscripcion
	if err := m.DB.First(&existente, "partido_id = ? AND jugador_id = ?", id, entrada.JugadorID).Error; err == nil {
		respuesta.Error(w, http.StatusConflict, "ya_inscripto", "el jugador ya está inscrito en este partido")
		return
	}

	// Verificar que haya cupo
	var inscripciones int64
	m.DB.Model(&Inscripcion{}).Where("partido_id = ?", id).Count(&inscripciones)
	if int(inscripciones) >= partido.CupoMaximo {
		respuesta.Error(w, http.StatusConflict, "partido_lleno", "el partido ya no tiene cupos disponibles")
		return
	}

	inscripcion := Inscripcion{
		PartidoID: id,
		JugadorID: entrada.JugadorID,
		CreadoEn:  time.Now(),
	}

	if err := m.DB.Create(&inscripcion).Error; err != nil {
		respuesta.Error(w, http.StatusInternalServerError, "error_base", "no se pudo guardar la inscripción")
		return
	}

	respuesta.Exito(w, http.StatusCreated, inscripcion)
}

func leerID(w http.ResponseWriter, r *http.Request) (uint, bool) {
	n, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || n <= 0 {
		respuesta.Error(w, http.StatusBadRequest, "id_invalido", "el id debe ser un entero positivo")
		return 0, false
	}
	return uint(n), true
}
