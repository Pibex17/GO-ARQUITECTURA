package reservas

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"canchas-sinteticas/internal/respuesta"
	"canchas-sinteticas/internal/sesion"
	"canchas-sinteticas/internal/usuarios"
)

// Manejador guarda la conexión y las piezas que se pueden sustituir en las
// pruebas. Si Repo o Vinculos quedan en nil, se usan las implementaciones
// GORM sobre DB; así main.go no cambia y las pruebas pueden usar dobles.
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

// POST /reservas
func (m *Manejador) crear(w http.ResponseWriter, r *http.Request) {
	// 1. Leer el JSON. El cuerpo no decide al dueño: no aceptamos
	// ClienteID, porque sale de la sesión.
	var entrada struct {
		CanchaID   uint
		Fecha      string
		HoraInicio string
		HoraFin    string
	}
	if err := json.NewDecoder(r.Body).Decode(&entrada); err != nil {
		respuesta.Error(w, http.StatusBadRequest,
			"cuerpo_malformado", "el cuerpo debe ser JSON")
		return
	}

	// 2. Validar los datos obligatorios de la reserva.
	if entrada.CanchaID == 0 ||
		entrada.Fecha == "" || entrada.HoraInicio == "" || entrada.HoraFin == "" {
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"datos_invalidos", "cancha_id, fecha, hora_inicio y hora_fin son obligatorios")
		return
	}

	// 3. El dueño se deriva de la cuenta autenticada.
	s, ok := sesionActual(w, r)
	if !ok {
		return
	}
	usuario, err := m.vinculos().UsuarioDe(s)
	if err != nil {
		responderSinVinculo(w)
		return
	}

	// --- desde aquí hace falta la base de datos ---

	// La cancha tiene que existir y estar activa.
	cancha, err := m.repositorio().CanchaPorID(entrada.CanchaID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respuesta.Error(w, http.StatusUnprocessableEntity,
				"cancha_invalida", "no existe una cancha activa con ese id")
			return
		}
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo leer la cancha")
		return
	}
	if !cancha.Activa {
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"cancha_invalida", "no existe una cancha activa con ese id")
		return
	}

	// El horario no puede estar ya tomado por una reserva que sigue en pie.
	enConflicto, err := m.repositorio().HayConflicto(
		entrada.CanchaID, entrada.Fecha, entrada.HoraInicio)
	if err != nil {
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo comprobar el horario")
		return
	}
	if enConflicto {
		respuesta.Error(w, http.StatusConflict,
			"horario_ocupado", "ya existe una reserva para esa cancha, fecha y hora")
		return
	}

	reserva := Reserva{
		CanchaID:   entrada.CanchaID,
		ClienteID:  usuario.ID, // el dueño es el usuario vinculado a la cuenta
		Fecha:      entrada.Fecha,
		HoraInicio: entrada.HoraInicio,
		HoraFin:    entrada.HoraFin,
		Estado:     "pendiente", // el servidor decide el estado inicial
	}
	if err := m.repositorio().CrearReserva(&reserva); err != nil {
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo guardar la reserva")
		return
	}
	respuesta.Exito(w, http.StatusCreated, reserva)
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

	filtro := FiltroReservas{Estado: r.URL.Query().Get("estado")}
	switch s.Rol {
	case sesion.RolCliente:
		clienteID := usuario.ID
		filtro.PropietarioClienteID = &clienteID
	case sesion.RolAdministrador:
		administradorID := usuario.ID
		filtro.AdministradorID = &administradorID

		if texto := r.URL.Query().Get("cliente_id"); texto != "" {
			n, err := strconv.Atoi(texto)
			if err != nil || n <= 0 {
				respuesta.Error(w, http.StatusBadRequest,
					"id_invalido", "el cliente_id debe ser un entero positivo")
				return
			}
			clienteID := uint(n)
			filtro.ClienteID = &clienteID
		}
	default:
		respuesta.Error(w, http.StatusForbidden,
			"permiso_denegado", "su rol no tiene permiso para esta acción")
		return
	}

	lista, err := m.repositorio().ListarReservas(filtro)
	if err != nil {
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo listar las reservas")
		return
	}
	respuesta.Exito(w, http.StatusOK, lista)
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

	reserva, err := m.repositorio().ReservaPorID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respuesta.Error(w, http.StatusNotFound,
				"no_encontrado", "no existe una reserva con ese id")
			return
		}
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo leer la reserva")
		return
	}

	permitido, err := m.puedeAcceder(s, usuario, reserva)
	if err != nil {
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo comprobar la propiedad de la reserva")
		return
	}
	if !permitido {
		respuesta.Error(w, http.StatusForbidden,
			"permiso_denegado", "la reserva no es suya ni de sus canchas")
		return
	}

	respuesta.Exito(w, http.StatusOK, reserva)
}

// puedeAcceder aplica la regla de propiedad de la ficha: el cliente solo
// ve sus reservas; el administrador solo las de sus canchas.
func (m *Manejador) puedeAcceder(
	s sesion.Sesion, usuario usuarios.Usuario, reserva Reserva,
) (bool, error) {
	switch s.Rol {
	case sesion.RolCliente:
		return reserva.ClienteID == usuario.ID, nil
	case sesion.RolAdministrador:
		cancha, err := m.repositorio().CanchaPorID(reserva.CanchaID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil // no se puede probar que sea suya
		}
		if err != nil {
			return false, err
		}
		return cancha.AdministradorID == usuario.ID, nil
	default:
		return false, nil
	}
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

	// 2. Leer el JSON. Si viene roto, 400 y se sale.
	var entrada entradaCambiarEstado
	if err := json.NewDecoder(r.Body).Decode(&entrada); err != nil {
		respuesta.Error(w, http.StatusBadRequest,
			"cuerpo_malformado", "el cuerpo debe ser JSON")
		return
	}
	// 3. Validar el estado pedido. Esta SÍ responde antes de tocar m.DB.
	if !estadosValidos[entrada.NuevoEstado] {
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"estado_invalido", "ese estado no existe")
		return
	}

	// --- desde aquí hace falta la base de datos ---

	reserva, err := m.repositorio().ReservaPorID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respuesta.Error(w, http.StatusNotFound,
				"no_encontrado", "no existe una reserva con ese id")
			return
		}
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo leer la reserva")
		return
	}

	// 4. El administrador solo puede cambiar reservas de sus canchas.
	cancha, err := m.repositorio().CanchaPorID(reserva.CanchaID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respuesta.Error(w, http.StatusForbidden,
				"permiso_denegado", "la reserva no es suya ni de sus canchas")
			return
		}
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo comprobar la cancha de la reserva")
		return
	}
	if cancha.AdministradorID != usuario.ID {
		respuesta.Error(w, http.StatusForbidden,
			"permiso_denegado", "la reserva no es suya ni de sus canchas")
		return
	}

	// 5. La transición pedida tiene que estar permitida (Sección 5 de la ficha).
	// El Hito 2 exige 422 para la transición prohibida.
	if !transicionPermitida(reserva.Estado, entrada.NuevoEstado) {
		respuesta.Error(w, http.StatusUnprocessableEntity,
			"transicion_invalida", "no se puede pasar de "+reserva.Estado+" a "+entrada.NuevoEstado)
		return
	}

	// 6. Efectos según a qué estado se pasa.
	switch entrada.NuevoEstado {
	case "pago_por_verificar":
		pago := Pago{
			ReservaID:      reserva.ID,
			Tipo:           "sena",
			Monto:          cancha.PrecioHora * 0.30,
			Metodo:         entrada.Metodo,
			ComprobanteURL: entrada.ComprobanteURL,
			Estado:         "pendiente",
		}
		if err := m.repositorio().CrearPago(&pago); err != nil {
			respuesta.Error(w, http.StatusInternalServerError,
				"error_base", "no se pudo registrar el pago")
			return
		}
	case "confirmada":
		if err := m.repositorio().MarcarPagoSenaVerificado(reserva.ID); err != nil {
			respuesta.Error(w, http.StatusInternalServerError,
				"error_base", "no se pudo verificar el pago")
			return
		}
	}

	reserva.Estado = entrada.NuevoEstado
	if err := m.repositorio().GuardarReserva(&reserva); err != nil {
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo actualizar la reserva")
		return
	}

	actualizada, err := m.repositorio().ReservaPorID(reserva.ID)
	if err != nil {
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo leer la reserva actualizada")
		return
	}
	respuesta.Exito(w, http.StatusOK, actualizada)
}
