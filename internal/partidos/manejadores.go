package partidos

import (
	"crypto/rand"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"canchas-sinteticas/internal/inscripcion"
	"canchas-sinteticas/internal/respuesta"
	"canchas-sinteticas/internal/reservas"
)

type Manejador struct{ DB *gorm.DB }

func (m *Manejador) Rutas(r chi.Router) {
	r.Post("/partidos", m.crear)
	r.Get("/partidos/{id}", m.obtener)
}

func leerID(w http.ResponseWriter, r *http.Request) (uint, bool) {
	n, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || n <= 0 {
		respuesta.Error(w, http.StatusBadRequest, "id_invalido", "el id debe ser un entero positivo")
		return 0, false
	}
	return uint(n), true
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

	// Verificar que la reserva existe
	var reserva reservas.Reserva
	if err := m.DB.First(&reserva, entrada.ReservaID).Error; err != nil {
		respuesta.Error(w, http.StatusNotFound, "no_encontrado", "no existe una reserva con ese id")
		return
	}

	// Verificar que la reserva está confirmada
	if reserva.Estado != "confirmada" {
		respuesta.Error(w, http.StatusUnprocessableEntity, "reserva_no_confirmada", "solo se puede crear un partido desde una reserva confirmada")
		return
	}

	// Verificar que no exista ya un partido para esa reserva
	var existente Partido
	if err := m.DB.First(&existente, "reserva_id = ?", entrada.ReservaID).Error; err == nil {
		respuesta.Error(w, http.StatusConflict, "partido_existente", "ya existe un partido para esa reserva")
		return
	}

	partido := Partido{
		ReservaID:        entrada.ReservaID,
		Nombre:           entrada.Nombre,
		CupoMaximo:       entrada.CupoMaximo,
		CodigoInvitacion: generarCodigoInvitacion(),
		CreadoEn:         time.Now(),
	}

	if err := m.DB.Create(&partido).Error; err != nil {
		respuesta.Error(w, http.StatusInternalServerError, "error_base", "no se pudo guardar el partido")
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

	var partido Partido
	if err := m.DB.First(&partido, id).Error; err != nil {
		respuesta.Error(w, http.StatusNotFound, "no_encontrado", "no existe un partido con ese id")
		return
	}

	var inscripciones int64
	m.DB.Model(&inscripcion.Inscripcion{}).Where("partido_id = ?", partido.ID).Count(&inscripciones)

	respuesta.Exito(w, http.StatusOK, map[string]any{
		"partido":        partido,
		"inscriptos":     inscripciones,
		"cupos_restantes": partido.CupoMaximo - int(inscripciones),
	})
}
