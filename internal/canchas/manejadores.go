package canchas

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"canchas-sinteticas/internal/respuesta"
	"canchas-sinteticas/internal/sesion"
)

// ServicioCanchas es el contrato que el transporte necesita del caso de uso.
// El manejador depende de esta abstracción, no de la implementación concreta.
type ServicioCanchas interface {
	Crear(administradorID uint, entrada EntradaCrear) (Cancha, error)
	Listar(rol string, usuarioID uint, administradorID *uint) ([]Cancha, error)
	Actualizar(id, administradorID uint, entrada EntradaActualizar) (Cancha, error)
}

// Manejador es la capa de transporte: traduce HTTP al servicio de canchas.
// No conoce GORM ni crea repositorios.
type Manejador struct {
	Servicio ServicioCanchas
	Vinculos sesion.ResolutorVinculos
}

func (m *Manejador) vinculos() sesion.ResolutorVinculos {
	if m.Vinculos != nil {
		return m.Vinculos
	}
	return sesion.Vinculos{}
}

// Rutas registra aquí las rutas de este paquete, para no llenar main.go.
// GET /canchas es para cualquier sesión; crear y editar son solo del
// administrador (ficha Hito 1, secciones 6 y 7).
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

func responderError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNoEncontrada):
		respuesta.Error(w, http.StatusNotFound,
			"no_encontrado", "no existe una cancha con ese id")
	case errors.Is(err, ErrFaltanDatos), errors.Is(err, ErrPrecioInvalido):
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"datos_invalidos", err.Error())
	case errors.Is(err, ErrTipoInvalido):
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"tipo_invalido", "el tipo de cancha no es válido")
	case errors.Is(err, ErrNoEsSuya):
		respuesta.Error(w, http.StatusForbidden,
			"permiso_denegado", "la cancha no es suya")
	case errors.Is(err, ErrRolNoPermitido):
		respuesta.Error(w, http.StatusForbidden,
			"permiso_denegado", "su rol no tiene permiso para esta acción")
	case errors.Is(err, errLeyendo):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo leer la cancha")
	case errors.Is(err, errListando):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo listar las canchas")
	case errors.Is(err, errEditando):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo actualizar la cancha")
	default:
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo guardar la cancha")
	}
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

	s, ok := sesionActual(w, r)
	if !ok {
		return
	}
	usuario, err := m.vinculos().UsuarioDe(s)
	if err != nil {
		responderSinVinculo(w)
		return
	}

	c, err := m.Servicio.Crear(usuario.ID, EntradaCrear{
		Nombre:     entrada.Nombre,
		Tipo:       entrada.Tipo,
		Direccion:  entrada.Direccion,
		PrecioHora: entrada.PrecioHora,
	})
	if err != nil {
		responderError(w, err)
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

	var administradorID *uint
	if s.Rol == sesion.RolCliente {
		if texto := r.URL.Query().Get("administrador_id"); texto != "" {
			n, err := strconv.Atoi(texto)
			if err != nil || n <= 0 {
				respuesta.Error(w, http.StatusBadRequest,
					"id_invalido", "el administrador_id debe ser un entero positivo")
				return
			}
			id := uint(n)
			administradorID = &id
		}
	}

	lista, err := m.Servicio.Listar(s.Rol, usuario.ID, administradorID)
	if err != nil {
		responderError(w, err)
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

	var entrada entradaActualizarCancha
	if err := json.NewDecoder(r.Body).Decode(&entrada); err != nil {
		respuesta.Error(w, http.StatusBadRequest,
			"cuerpo_malformado", "el cuerpo debe ser JSON")
		return
	}

	actualizada, err := m.Servicio.Actualizar(id, usuario.ID, EntradaActualizar{
		Nombre:     entrada.Nombre,
		Tipo:       entrada.Tipo,
		Direccion:  entrada.Direccion,
		PrecioHora: entrada.PrecioHora,
		Activa:     entrada.Activa,
	})
	if err != nil {
		responderError(w, err)
		return
	}
	respuesta.Exito(w, http.StatusOK, actualizada)
}
