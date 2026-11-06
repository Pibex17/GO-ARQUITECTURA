package reservas

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"canchas-sinteticas/internal/respuesta"
	"canchas-sinteticas/internal/sesion"
)

// ServicioReservas es el contrato que el transporte necesita del caso de uso.
// El manejador depende de esta abstracción, no de la implementación concreta.
type ServicioReservas interface {
	Crear(clienteID uint, entrada EntradaCrear) (Reserva, error)
	Listar(rol string, usuarioID uint, estado string, clienteID *uint) ([]Reserva, error)
	Obtener(id uint, rol string, usuarioID uint) (Reserva, error)
	CambiarEstado(id, administradorID uint, nuevoEstado, metodo, comprobanteURL string) (Reserva, error)
}

// Manejador es la capa de transporte: traduce HTTP al servicio de reservas.
// No conoce GORM ni crea repositorios.
type Manejador struct {
	Servicio ServicioReservas
	Vinculos sesion.ResolutorVinculos
}

func (m *Manejador) vinculos() sesion.ResolutorVinculos {
	if m.Vinculos != nil {
		return m.Vinculos
	}
	return sesion.Vinculos{}
}

// Rutas registra aquí las rutas de este paquete, para no llenar main.go.
// El cliente crea sus reservas y ambos roles consultan; el cambio de
// estado es solo del administrador (decisión J2 y rúbrica del Hito 2).
func (m *Manejador) Rutas(r chi.Router, tokens *sesion.Tokens) {
	conSesion := r.With(tokens.Autenticar)
	soloCliente := conSesion.With(sesion.SoloRol(sesion.RolCliente))
	soloAdmin := conSesion.With(sesion.SoloRol(sesion.RolAdministrador))

	soloCliente.Post("/reservas", m.crear)
	conSesion.Get("/reservas", m.listar)
	conSesion.Get("/reservas/{id}", m.obtener)
	soloAdmin.Patch("/reservas/{id}/estado", m.cambiarEstado)
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

// responderSinVinculo no revela si la cuenta no existe, si no tiene
// UsuarioID o si el rol del usuario no coincide. En todos los casos la
// operación se rechaza sin atribuirle otra identidad.
func responderSinVinculo(w http.ResponseWriter) {
	respuesta.Error(w, http.StatusForbidden,
		"cuenta_sin_vinculo", "la cuenta no está vinculada a un usuario de negocio")
}

func responderError(w http.ResponseWriter, err error) {
	var transicion ErrorTransicion
	switch {
	case errors.As(err, &transicion):
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"transicion_invalida", transicion.Error())
	case errors.Is(err, ErrDatosInvalidos):
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"datos_invalidos", ErrDatosInvalidos.Error())
	case errors.Is(err, ErrCanchaInvalida):
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"cancha_invalida", ErrCanchaInvalida.Error())
	case errors.Is(err, ErrHorarioOcupado):
		respuesta.Error(w, http.StatusConflict,
			"horario_ocupado", ErrHorarioOcupado.Error())
	case errors.Is(err, ErrNoEncontrada):
		respuesta.Error(w, http.StatusNotFound,
			"no_encontrado", ErrNoEncontrada.Error())
	case errors.Is(err, ErrPermisoDenegado):
		respuesta.Error(w, http.StatusForbidden,
			"permiso_denegado", ErrPermisoDenegado.Error())
	case errors.Is(err, ErrEstadoInvalido):
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"estado_invalido", ErrEstadoInvalido.Error())
	case errors.Is(err, ErrRolNoPermitido):
		respuesta.Error(w, http.StatusForbidden,
			"permiso_denegado", ErrRolNoPermitido.Error())
	case errors.Is(err, ErrPagoNoRegistrado):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", ErrPagoNoRegistrado.Error())
	case errors.Is(err, ErrPagoNoVerificado):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", ErrPagoNoVerificado.Error())
	case errors.Is(err, ErrReservaNoGuardada):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", ErrReservaNoGuardada.Error())
	case errors.Is(err, errLeyendoCancha):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", errLeyendoCancha.Error())
	case errors.Is(err, errComprobandoHorario):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", errComprobandoHorario.Error())
	case errors.Is(err, errGuardandoReserva):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", errGuardandoReserva.Error())
	case errors.Is(err, errListandoReservas):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", errListandoReservas.Error())
	case errors.Is(err, errLeyendoReserva):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", errLeyendoReserva.Error())
	case errors.Is(err, errComprobandoPropiedad):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", errComprobandoPropiedad.Error())
	case errors.Is(err, errLeyendoActualizada):
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", errLeyendoActualizada.Error())
	default:
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo procesar la solicitud")
	}
}

// POST /reservas
func (m *Manejador) crear(w http.ResponseWriter, r *http.Request) {
	// 1. Leer el JSON con el DTO de entrada. El cuerpo no decide al dueño
	// ni el estado: ReservaPeticion no tiene esos campos.
	var pedido ReservaPeticion
	if err := json.NewDecoder(r.Body).Decode(&pedido); err != nil {
		respuesta.Error(w, http.StatusBadRequest,
			"cuerpo_malformado", "el cuerpo debe ser JSON")
		return
	}

	// 2. El dueño se deriva de la cuenta autenticada.
	s, ok := sesionActual(w, r)
	if !ok {
		return
	}
	usuario, err := m.vinculos().UsuarioDe(s)
	if err != nil {
		responderSinVinculo(w)
		return
	}

	reserva, err := m.Servicio.Crear(usuario.ID, pedido.aEntrada())
	if err != nil {
		responderError(w, err)
		return
	}
	respuesta.Exito(w, http.StatusCreated, nuevaReservaRespuesta(reserva))
}

// GET /reservas — el cliente solo ve las suyas; el administrador solo las
// de sus canchas. ?estado= puede reducir, nunca ampliar. ?cliente_id= se
// ignora para el cliente y solo se aplica dentro de las canchas del admin.
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

	var clienteID *uint
	if s.Rol == sesion.RolAdministrador {
		if texto := r.URL.Query().Get("cliente_id"); texto != "" {
			n, err := strconv.Atoi(texto)
			if err != nil || n <= 0 {
				respuesta.Error(w, http.StatusBadRequest,
					"id_invalido", "el cliente_id debe ser un entero positivo")
				return
			}
			id := uint(n)
			clienteID = &id
		}
	}

	lista, err := m.Servicio.Listar(
		s.Rol, usuario.ID, r.URL.Query().Get("estado"), clienteID)
	if err != nil {
		responderError(w, err)
		return
	}
	respuesta.Exito(w, http.StatusOK, nuevasReservasRespuesta(lista))
}

// GET /reservas/{id}
func (m *Manejador) obtener(w http.ResponseWriter, r *http.Request) {
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

	reserva, err := m.Servicio.Obtener(id, s.Rol, usuario.ID)
	if err != nil {
		responderError(w, err)
		return
	}
	respuesta.Exito(w, http.StatusOK, nuevaReservaRespuesta(reserva))
}

// PATCH /reservas/{id}/estado
func (m *Manejador) cambiarEstado(w http.ResponseWriter, r *http.Request) {
	id, ok := leerID(w, r)
	if !ok {
		return
	}

	// 1. Autenticación y autorización ANTES de evaluar el cuerpo o la
	// transición de negocio. La ruta ya exige administrador con SoloRol,
	// pero se repite aquí para que un cambio en el enrutamiento no deje
	// la regla de estados como primera defensa: un rol no autorizado debe
	// recibir 403, nunca 422.
	s, ok := sesionActual(w, r)
	if !ok {
		return
	}
	if s.Rol != sesion.RolAdministrador {
		respuesta.Error(w, http.StatusForbidden,
			"permiso_denegado", "su rol no tiene permiso para esta acción")
		return
	}
	usuario, err := m.vinculos().UsuarioDe(s)
	if err != nil {
		responderSinVinculo(w)
		return
	}

	// 2. Leer el JSON con el DTO de cambio de estado. Si viene roto, 400.
	var pedido CambioEstadoPeticion
	if err := json.NewDecoder(r.Body).Decode(&pedido); err != nil {
		respuesta.Error(w, http.StatusBadRequest,
			"cuerpo_malformado", "el cuerpo debe ser JSON")
		return
	}

	// 3. El servicio valida el estado antes de consultar el repositorio.
	actualizada, err := m.Servicio.CambiarEstado(
		id, usuario.ID, pedido.NuevoEstado, pedido.Metodo, pedido.ComprobanteURL)
	if err != nil {
		responderError(w, err)
		return
	}
	respuesta.Exito(w, http.StatusOK, nuevaReservaRespuesta(actualizada))
}
