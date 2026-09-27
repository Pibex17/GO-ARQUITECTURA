package canchas

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"canchas-sinteticas/internal/respuesta"
	"canchas-sinteticas/internal/usuarios"
)

// Manejador guarda la conexión. Todas las rutas la leen con m.DB.
type Manejador struct{ DB *gorm.DB }

// Rutas registra aquí las rutas de este paquete, para no llenar main.go.
func (m *Manejador) Rutas(r chi.Router) {
	r.Get("/canchas", m.listar)
	r.Post("/canchas", m.crear)
	r.Patch("/canchas/{id}", m.actualizar)
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

// POST /canchas
func (m *Manejador) crear(w http.ResponseWriter, r *http.Request) {
	// 1. Leer el JSON. Si viene roto, 400 y se sale.
	var c Cancha
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		respuesta.Error(w, http.StatusBadRequest,
			"cuerpo_malformado", "el cuerpo debe ser JSON")
		return
	}
	c.ID = 0 // que nadie elija su propio ID

	// 2. Validar.
	if c.Nombre == "" || c.Direccion == "" {
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"datos_invalidos", "nombre y dirección son obligatorios")
		return
	}
	if c.PrecioHora <= 0 {
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"datos_invalidos", "el precio por hora debe ser mayor que cero")
		return
	}
	if !tiposValidos[c.Tipo] {
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"tipo_invalido", "el tipo de cancha no es válido")
		return
	}
	// Regla propia de este negocio: el administrador tiene que existir
	// y tener el rol correcto, antes de guardar la cancha (2b).
	var admin usuarios.Usuario
	if err := m.DB.First(&admin, c.AdministradorID).Error; err != nil {
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"administrador_invalido", "no existe un usuario con ese administrador_id")
		return
	}
	if admin.Rol != "administrador" {
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"administrador_invalido", "ese usuario no tiene rol administrador")
		return
	}
	c.Activa = true // el servidor decide el valor inicial, no quien crea

	// 3. Guardar.
	if err := m.DB.Debug().Create(&c).Error; err != nil {
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo guardar la cancha")
		return
	}
	respuesta.Exito(w, http.StatusCreated, c)
}

// GET /canchas
// Sin "administrador_id": el catálogo público, solo canchas activas.
// Con "administrador_id=N": todas las canchas de ese administrador (activas o no).
func (m *Manejador) listar(w http.ResponseWriter, r *http.Request) {
	adminID := r.URL.Query().Get("administrador_id")

	consulta := m.DB.Debug().Preload("Horarios")
	if adminID != "" {
		consulta = consulta.Where("administrador_id = ?", adminID)
	} else {
		consulta = consulta.Where("activa = ?", true)
	}

	var lista []Cancha
	if err := consulta.Find(&lista).Error; err != nil {
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo listar las canchas")
		return
	}
	respuesta.Exito(w, http.StatusOK, lista)
}

type entradaActualizarCancha struct {
	Nombre     string
	Tipo       string
	Direccion  string
	PrecioHora float64
	Activa     *bool // puntero: distingue "no lo mandaron" (nil) de "lo mandaron en false"
}

// PATCH /canchas/{id}
func (m *Manejador) actualizar(w http.ResponseWriter, r *http.Request) {
	id, ok := leerID(w, r)
	if !ok {
		return
	}
	var existente Cancha
	if err := m.DB.First(&existente, id).Error; err != nil {
		respuesta.Error(w, http.StatusNotFound,
			"no_encontrado", "no existe una cancha con ese id")
		return
	}

	var entrada entradaActualizarCancha
	if err := json.NewDecoder(r.Body).Decode(&entrada); err != nil {
		respuesta.Error(w, http.StatusBadRequest,
			"cuerpo_malformado", "el cuerpo debe ser JSON")
		return
	}
	if entrada.Tipo != "" && !tiposValidos[entrada.Tipo] {
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"tipo_invalido", "el tipo de cancha no es válido")
		return
	}

	// Solo estos campos se pueden cambiar (decisión D3 de docs/decisiones.md):
	// el AdministradorID no se toca desde este endpoint.
	if entrada.Nombre != "" {
		existente.Nombre = entrada.Nombre
	}
	if entrada.Tipo != "" {
		existente.Tipo = entrada.Tipo
	}
	if entrada.Direccion != "" {
		existente.Direccion = entrada.Direccion
	}
	if entrada.PrecioHora > 0 {
		existente.PrecioHora = entrada.PrecioHora
	}
	if entrada.Activa != nil {
		existente.Activa = *entrada.Activa
	}

	if err := m.DB.Save(&existente).Error; err != nil {
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo actualizar la cancha")
		return
	}
	respuesta.Exito(w, http.StatusOK, existente)
}
