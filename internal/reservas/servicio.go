package reservas

import (
	"errors"
	"fmt"

	"canchas-sinteticas/internal/canchas"
)

// Errores de negocio de reservas. El transporte los traduce a HTTP.
var (
	ErrNoEncontrada      = errors.New("no existe una reserva con ese id")
	ErrDatosInvalidos    = errors.New("cancha_id, fecha, hora_inicio y hora_fin son obligatorios")
	ErrCanchaInvalida    = errors.New("no existe una cancha activa con ese id")
	ErrHorarioOcupado    = errors.New("ya existe una reserva para esa cancha, fecha y hora")
	ErrRolNoPermitido    = errors.New("su rol no tiene permiso para esta acción")
	ErrPermisoDenegado   = errors.New("la reserva no es suya ni de sus canchas")
	ErrEstadoInvalido    = errors.New("ese estado no existe")
	ErrPagoNoRegistrado  = errors.New("no se pudo registrar el pago")
	ErrPagoNoVerificado  = errors.New("no se pudo verificar el pago")
	ErrReservaNoGuardada = errors.New("no se pudo actualizar la reserva")

	// Errores internos: distinguen el origen del fallo sin filtrar detalles
	// de GORM a la capa HTTP.
	errLeyendoCancha        = errors.New("no se pudo leer la cancha")
	errComprobandoHorario   = errors.New("no se pudo comprobar el horario")
	errGuardandoReserva     = errors.New("no se pudo guardar la reserva")
	errListandoReservas     = errors.New("no se pudo listar las reservas")
	errLeyendoReserva       = errors.New("no se pudo leer la reserva")
	errComprobandoPropiedad = errors.New("no se pudo comprobar la propiedad de la reserva")
	errLeyendoActualizada   = errors.New("no se pudo leer la reserva actualizada")
)

// ErrorTransicion describe una transición prohibida conservando el mensaje
// que ya devolvía la API.
type ErrorTransicion struct {
	De string
	A  string
}

func (e ErrorTransicion) Error() string {
	return "no se puede pasar de " + e.De + " a " + e.A
}

// Repositorio es la persistencia que necesita el servicio. La interfaz vive
// en la capa de servicio; la implementación GORM está en repositorio.go.
type Repositorio interface {
	CanchaPorID(id uint) (canchas.Cancha, error)
	HayConflicto(canchaID uint, fecha, horaInicio string) (bool, error)
	CrearReserva(reserva *Reserva) error
	ListarReservas(filtro FiltroReservas) ([]Reserva, error)
	ReservaPorID(id uint) (Reserva, error)
	AplicarCambioEstado(reserva *Reserva, pago *Pago, verificarPago bool) error
}

// EntradaCrear son los datos de negocio de una reserva nueva.
type EntradaCrear struct {
	CanchaID   uint
	Fecha      string
	HoraInicio string
	HoraFin    string
}

// Servicio contiene los casos de uso de reservas y pagos.
type Servicio struct {
	repo Repositorio
}

// NuevoServicio inyecta la persistencia que necesitan los casos de uso.
func NuevoServicio(repo Repositorio) *Servicio {
	return &Servicio{repo: repo}
}

// Crear valida la cancha y el horario, y guarda una reserva pendiente.
func (s *Servicio) Crear(clienteID uint, entrada EntradaCrear) (Reserva, error) {
	if entrada.CanchaID == 0 ||
		entrada.Fecha == "" || entrada.HoraInicio == "" || entrada.HoraFin == "" {
		return Reserva{}, ErrDatosInvalidos
	}

	cancha, err := s.repo.CanchaPorID(entrada.CanchaID)
	if err != nil {
		if errors.Is(err, canchas.ErrNoEncontrada) {
			return Reserva{}, ErrCanchaInvalida
		}
		return Reserva{}, fmt.Errorf("%w: %v", errLeyendoCancha, err)
	}
	if !cancha.Activa {
		return Reserva{}, ErrCanchaInvalida
	}

	enConflicto, err := s.repo.HayConflicto(
		entrada.CanchaID, entrada.Fecha, entrada.HoraInicio)
	if err != nil {
		return Reserva{}, fmt.Errorf("%w: %v", errComprobandoHorario, err)
	}
	if enConflicto {
		return Reserva{}, ErrHorarioOcupado
	}

	reserva := Reserva{
		CanchaID:   entrada.CanchaID,
		ClienteID:  clienteID,
		Fecha:      entrada.Fecha,
		HoraInicio: entrada.HoraInicio,
		HoraFin:    entrada.HoraFin,
		Estado:     "pendiente", // el servidor decide el estado inicial
	}
	if err := s.repo.CrearReserva(&reserva); err != nil {
		return Reserva{}, fmt.Errorf("%w: %v", errGuardandoReserva, err)
	}
	return reserva, nil
}

// Listar aplica la visibilidad de las reservas según el rol. El cliente
// solo ve las suyas; el administrador solo las de sus canchas.
func (s *Servicio) Listar(
	rol string, usuarioID uint, estado string, clienteID *uint,
) ([]Reserva, error) {
	filtro := FiltroReservas{Estado: estado}
	switch rol {
	case "cliente":
		propietario := usuarioID
		filtro.PropietarioClienteID = &propietario
	case "administrador":
		administradorID := usuarioID
		filtro.AdministradorID = &administradorID
		filtro.ClienteID = clienteID
	default:
		return nil, ErrRolNoPermitido
	}

	lista, err := s.repo.ListarReservas(filtro)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errListandoReservas, err)
	}
	return lista, nil
}

// Obtener devuelve una reserva solo si pertenece al cliente o a una cancha
// del administrador.
func (s *Servicio) Obtener(id uint, rol string, usuarioID uint) (Reserva, error) {
	reserva, err := s.repo.ReservaPorID(id)
	if err != nil {
		if errors.Is(err, ErrNoEncontrada) {
			return Reserva{}, ErrNoEncontrada
		}
		return Reserva{}, fmt.Errorf("%w: %v", errLeyendoReserva, err)
	}

	permitido, err := s.puedeAcceder(rol, usuarioID, reserva)
	if err != nil {
		return Reserva{}, err
	}
	if !permitido {
		return Reserva{}, ErrPermisoDenegado
	}
	return reserva, nil
}

func (s *Servicio) puedeAcceder(rol string, usuarioID uint, reserva Reserva) (bool, error) {
	switch rol {
	case "cliente":
		return reserva.ClienteID == usuarioID, nil
	case "administrador":
		cancha, err := s.repo.CanchaPorID(reserva.CanchaID)
		if errors.Is(err, canchas.ErrNoEncontrada) {
			return false, nil // no se puede probar que sea suya
		}
		if err != nil {
			return false, fmt.Errorf("%w: %v", errComprobandoPropiedad, err)
		}
		return cancha.AdministradorID == usuarioID, nil
	default:
		return false, nil
	}
}

// CambiarEstado aplica la máquina de estados, comprueba propiedad y ejecuta
// los efectos del nuevo estado (seña o verificación) en una transacción.
func (s *Servicio) CambiarEstado(
	id, administradorID uint, nuevoEstado, metodo, comprobanteURL string,
) (Reserva, error) {
	if !estadosValidos[nuevoEstado] {
		return Reserva{}, ErrEstadoInvalido
	}

	reserva, err := s.repo.ReservaPorID(id)
	if err != nil {
		if errors.Is(err, ErrNoEncontrada) {
			return Reserva{}, ErrNoEncontrada
		}
		return Reserva{}, fmt.Errorf("%w: %v", errLeyendoReserva, err)
	}

	cancha, err := s.repo.CanchaPorID(reserva.CanchaID)
	if err != nil {
		if errors.Is(err, canchas.ErrNoEncontrada) {
			return Reserva{}, ErrPermisoDenegado
		}
		return Reserva{}, fmt.Errorf("%w: %v", errComprobandoPropiedad, err)
	}
	if cancha.AdministradorID != administradorID {
		return Reserva{}, ErrPermisoDenegado
	}

	if !transicionPermitida(reserva.Estado, nuevoEstado) {
		return Reserva{}, ErrorTransicion{De: reserva.Estado, A: nuevoEstado}
	}

	var pago *Pago
	verificarPago := false
	switch nuevoEstado {
	case "pago_por_verificar":
		nuevo := NuevoPagoSena(reserva.ID, cancha.PrecioHora, metodo, comprobanteURL)
		pago = &nuevo
	case "confirmada":
		verificarPago = true
	}

	reserva.Estado = nuevoEstado
	if err := s.repo.AplicarCambioEstado(&reserva, pago, verificarPago); err != nil {
		return Reserva{}, err
	}

	actualizada, err := s.repo.ReservaPorID(reserva.ID)
	if err != nil {
		return Reserva{}, fmt.Errorf("%w: %v", errLeyendoActualizada, err)
	}
	return actualizada, nil
}
