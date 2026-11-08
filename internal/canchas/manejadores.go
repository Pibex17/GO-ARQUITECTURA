package canchas

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

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

// Rutas registra aquí las rutas de este paquete, para no llenar main.go.
// GET /canchas es para cualquier sesión; crear y editar son solo del
// administrador dueño de la cancha.
func (m *Manejador) Rutas(r chi.Router, tokens *sesion.Tokens) {
	conSesion := r.With(tokens.Autenticar)
	soloAdmin := conSesion.With(sesion.SoloRol(sesion.RolAdministrador))

	conSesion.Get("/canchas", m.listar)
	soloAdmin.Post("/canchas", m.crear)
	soloAdmin.Patch("/canchas/{id}", m.actualizar)
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

// POST /canchas
func (m *Manejador) crear(w http.ResponseWriter, r *http.Request) {
	// El cuerpo no incluye AdministradorID: el dueño sale de la sesión.
	var entrada struct {
		Nombre     string
		Tipo       string
		Direccion  string
		PrecioHora float64
	}
	if err := json.NewDecoder(r.Body).Decode(&entrada); err != nil {
		respuesta.Error(w, http.StatusBadRequest,
			"cuerpo_malformado", "el cuerpo debe ser JSON")
		return
	}

	if entrada.Nombre == "" || entrada.Direccion == "" {
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"datos_invalidos", "nombre y dirección son obligatorios")
		return
	}
	if entrada.PrecioHora <= 0 {
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"datos_invalidos", "el precio por hora debe ser mayor que cero")
		return
	}
	if !tiposValidos[entrada.Tipo] {
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"tipo_invalido", "el tipo de cancha no es válido")
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

	c := Cancha{
		Nombre:          entrada.Nombre,
		Tipo:            entrada.Tipo,
		Direccion:       entrada.Direccion,
		PrecioHora:      entrada.PrecioHora,
		Activa:          true,       // el servidor decide el valor inicial
		AdministradorID: usuario.ID, // el dueño es la cuenta autenticada
	}
	if err := m.repositorio().CrearCancha(&c); err != nil {
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo guardar la cancha")
		return
	}
	respuesta.Exito(w, http.StatusCreated, c)
}

// GET /canchas — el cliente solo ve activas; el administrador solo las
// suyas, activas e inactivas. El query no amplía esos límites.
func (m *Manejador) listar(w http.ResponseWriter, r *http.Request) {
	s, ok := sesionActual(w, r)
	if !ok {
		return
	}
	usuario, err := m.vinculos().UsuarioDe(s)
	if err != nil {
		responderSinVinculo(w)
		return
	}

	filtro := FiltroCanchas{}
	switch s.Rol {
	case sesion.RolCliente:
		activa := true
		filtro.Activa = &activa

		// Se conserva el filtro opcional del catálogo, pero siempre dentro
		// de las canchas activas.
		if texto := r.URL.Query().Get("administrador_id"); texto != "" {
			n, err := strconv.Atoi(texto)
			if err != nil || n <= 0 {
				respuesta.Error(w, http.StatusBadRequest,
					"id_invalido", "el administrador_id debe ser un entero positivo")
				return
			}
			administradorID := uint(n)
			filtro.AdministradorID = &administradorID
		}
	case sesion.RolAdministrador:
		administradorID := usuario.ID
		filtro.AdministradorID = &administradorID
	default:
		respuesta.Error(w, http.StatusForbidden,
			"permiso_denegado", "su rol no tiene permiso para esta acción")
		return
	}

	lista, err := m.repositorio().ListarCanchas(filtro)
	if err != nil {
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

	s, ok := sesionActual(w, r)
	if !ok {
		return
	}
	usuario, err := m.vinculos().UsuarioDe(s)
	if err != nil {
		responderSinVinculo(w)
		return
	}

	existente, err := m.repositorio().CanchaPorID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respuesta.Error(w, http.StatusNotFound,
				"no_encontrado", "no existe una cancha con ese id")
			return
		}
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo leer la cancha")
		return
	}
	if existente.AdministradorID != usuario.ID {
		respuesta.Error(w, http.StatusForbidden,
			"permiso_denegado", "la cancha no es suya")
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

	// Solo estos campos se pueden cambiar: el AdministradorID no se toca
	// desde este endpoint.
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

	if err := m.repositorio().GuardarCancha(&existente); err != nil {
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo actualizar la cancha")
		return
	}
	respuesta.Exito(w, http.StatusOK, existente)
}
