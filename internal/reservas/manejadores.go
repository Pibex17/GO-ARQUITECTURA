package reservas

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"canchas-sinteticas/internal/canchas"
	"canchas-sinteticas/internal/respuesta"
	"canchas-sinteticas/internal/usuarios"
)

// Manejador guarda la conexión. Todas las rutas la leen con m.DB.
type Manejador struct{ DB *gorm.DB }

// Rutas registra aquí las rutas de este paquete, para no llenar main.go.
func (m *Manejador) Rutas(r chi.Router) {
	r.Post("/reservas", m.crear)
	r.Get("/reservas", m.listar)
	r.Patch("/reservas/{id}/estado", m.cambiarEstado)
}

// leerID saca el id de la URL y lo convierte a número.
func leerID(w http.ResponseWriter, r *http.Request) (uint, bool) {
	n, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || n <= 0 {
		respuesta.Error(w, http.StatusBadRequest,
			"id_invalido", "el id debe ser un entero positivo")
		return 0, false
	}
	return uint(n), true
}

// POST /reservas
func (m *Manejador) crear(w http.ResponseWriter, r *http.Request) {
	// 1. Leer el JSON. Si viene roto, 400 y se sale.
	var entrada Reserva
	if err := json.NewDecoder(r.Body).Decode(&entrada); err != nil {
		respuesta.Error(w, http.StatusBadRequest,
			"cuerpo_malformado", "el cuerpo debe ser JSON")
		return
	}

	// 2. Validar los datos obligatorios. Esta es la regla propia de este
	// negocio (2b): ninguna todavía toca m.DB.
	if entrada.CanchaID == 0 || entrada.ClienteID == 0 ||
		entrada.Fecha == "" || entrada.HoraInicio == "" || entrada.HoraFin == "" {
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"datos_invalidos", "cancha_id, cliente_id, fecha, hora_inicio y hora_fin son obligatorios")
		return
	}

	// --- desde aquí hace falta la base de datos ---

	// La cancha tiene que existir y estar activa.
	var cancha canchas.Cancha
	if err := m.DB.First(&cancha, entrada.CanchaID).Error; err != nil || !cancha.Activa {
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"cancha_invalida", "no existe una cancha activa con ese id")
		return
	}
	// El cliente tiene que existir y tener rol cliente.
	var cliente usuarios.Usuario
	if err := m.DB.First(&cliente, entrada.ClienteID).Error; err != nil || cliente.Rol != "cliente" {
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"cliente_invalido", "no existe un usuario cliente con ese id")
		return
	}
	// El horario no puede estar ya tomado por una reserva que sigue en pie.
	var enConflicto int64
	m.DB.Model(&Reserva{}).
		Where("cancha_id = ? AND fecha = ? AND hora_inicio = ? AND estado <> ?",
			entrada.CanchaID, entrada.Fecha, entrada.HoraInicio, "cancelada").
		Count(&enConflicto)
	if enConflicto > 0 {
		respuesta.Error(w, http.StatusConflict,
			"horario_ocupado", "ya existe una reserva para esa cancha, fecha y hora")
		return
	}

	reserva := Reserva{
		CanchaID:   entrada.CanchaID,
		ClienteID:  entrada.ClienteID,
		Fecha:      entrada.Fecha,
		HoraInicio: entrada.HoraInicio,
		HoraFin:    entrada.HoraFin,
		Estado:     "pendiente", // el servidor decide el estado inicial, no quien crea
	}
	if err := m.DB.Debug().Create(&reserva).Error; err != nil {
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo guardar la reserva")
		return
	}
	respuesta.Exito(w, http.StatusCreated, reserva)
}

// GET /reservas — acepta ?estado= y ?cliente_id= para filtrar.
// Preload("Pagos") evita una consulta por cada reserva (2c).
func (m *Manejador) listar(w http.ResponseWriter, r *http.Request) {
	consulta := m.DB.Debug().Preload("Pagos")

	if estado := r.URL.Query().Get("estado"); estado != "" {
		consulta = consulta.Where("estado = ?", estado)
	}
	if clienteID := r.URL.Query().Get("cliente_id"); clienteID != "" {
		consulta = consulta.Where("cliente_id = ?", clienteID)
	}

	var lista []Reserva
	if err := consulta.Find(&lista).Error; err != nil {
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo listar las reservas")
		return
	}
	respuesta.Exito(w, http.StatusOK, lista)
}

type entradaCambiarEstado struct {
	NuevoEstado    string
	Metodo         string // solo si NuevoEstado es "pago_por_verificar"
	ComprobanteURL string // solo si NuevoEstado es "pago_por_verificar"
}

// PATCH /reservas/{id}/estado
func (m *Manejador) cambiarEstado(w http.ResponseWriter, r *http.Request) {
	id, ok := leerID(w, r)
	if !ok {
		return
	}

	// 1. Leer el JSON. Si viene roto, 400 y se sale.
	var entrada entradaCambiarEstado
	if err := json.NewDecoder(r.Body).Decode(&entrada); err != nil {
		respuesta.Error(w, http.StatusBadRequest,
			"cuerpo_malformado", "el cuerpo debe ser JSON")
		return
	}
	// 2. Validar el estado pedido. Esta SÍ responde antes de tocar m.DB.
	if !estadosValidos[entrada.NuevoEstado] {
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"estado_invalido", "ese estado no existe")
		return
	}

	// --- desde aquí hace falta la base de datos ---

	var reserva Reserva
	if err := m.DB.First(&reserva, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respuesta.Error(w, http.StatusNotFound,
				"no_encontrado", "no existe una reserva con ese id")
			return
		}
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo leer la reserva")
		return
	}

	// 3. La transición pedida tiene que estar permitida (Sección 5 de la ficha).
	if !transicionPermitida(reserva.Estado, entrada.NuevoEstado) {
		respuesta.Error(w, http.StatusBadRequest,
			"transicion_invalida", "no se puede pasar de "+reserva.Estado+" a "+entrada.NuevoEstado)
		return
	}

	// 4. Efectos según a qué estado se pasa.
	switch entrada.NuevoEstado {
	case "pago_por_verificar":
		var cancha canchas.Cancha
		if err := m.DB.First(&cancha, reserva.CanchaID).Error; err != nil {
			respuesta.Error(w, http.StatusInternalServerError,
				"error_base", "no se pudo calcular la seña")
			return
		}
		pago := Pago{
			ReservaID:      reserva.ID,
			Tipo:           "sena",
			Monto:          cancha.PrecioHora * 0.30,
			Metodo:         entrada.Metodo,
			ComprobanteURL: entrada.ComprobanteURL,
			Estado:         "pendiente",
		}
		if err := m.DB.Create(&pago).Error; err != nil {
			respuesta.Error(w, http.StatusInternalServerError,
				"error_base", "no se pudo registrar el pago")
			return
		}
	case "confirmada":
		m.DB.Model(&Pago{}).
			Where("reserva_id = ? AND tipo = ? AND estado = ?", reserva.ID, "sena", "pendiente").
			Update("estado", "verificado")
	}

	reserva.Estado = entrada.NuevoEstado
	if err := m.DB.Save(&reserva).Error; err != nil {
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo actualizar la reserva")
		return
	}
	m.DB.Preload("Pagos").First(&reserva, reserva.ID)
	respuesta.Exito(w, http.StatusOK, reserva)
}
