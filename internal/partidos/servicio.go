package partidos

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"canchas-sinteticas/internal/reservas"
)

// Errores de negocio de partidos. El transporte los traduce a HTTP.
var (
	ErrDatosInvalidos      = errors.New("reserva_id, nombre y cupo_maximo son obligatorios")
	ErrNoEncontrado        = errors.New("no existe un partido con ese id")
	ErrReservaNoEncontrada = errors.New("no existe una reserva con ese id")
	ErrReservaAjena        = errors.New("la reserva no es suya")
	ErrReservaNoConfirmada = errors.New("solo se puede crear un partido desde una reserva confirmada")
	ErrPartidoExistente    = errors.New("ya existe un partido para esa reserva")

	errLeyendoReserva      = errors.New("no se pudo leer la reserva")
	errComprobandoPartido  = errors.New("no se pudo comprobar el partido")
	errGenerandoCodigo     = errors.New("no se pudo generar el código de invitación")
	errGuardandoPartido    = errors.New("no se pudo guardar el partido")
	errLeyendoPartido      = errors.New("no se pudo leer el partido")
	errContandoInscripcion = errors.New("no se pudo contar los inscriptos")
)

// Repositorio es la persistencia que necesita el servicio. La interfaz vive
// en la capa de servicio; la implementación GORM está en repositorio.go.
type Repositorio interface {
	ReservaPorID(id uint) (reservas.Reserva, error)
	PartidoPorReservaID(reservaID uint) (Partido, error)
	PartidoPorID(id uint) (Partido, error)
	ContarInscripciones(partidoID uint) (int64, error)
	CrearPartido(partido *Partido) error
}

// EntradaCrear son los datos de negocio de un partido nuevo.
type EntradaCrear struct {
	ReservaID  uint
	Nombre     string
	CupoMaximo int
}

// Detalle es la ficha pública del partido con su ocupación.
type Detalle struct {
	Partido        Partido `json:"partido"`
	Inscriptos     int64   `json:"inscriptos"`
	CuposRestantes int     `json:"cupos_restantes"`
}

// Servicio contiene los casos de uso de partidos.
type Servicio struct {
	repo Repositorio
}

// NuevoServicio inyecta la persistencia que necesitan los casos de uso.
func NuevoServicio(repo Repositorio) *Servicio {
	return &Servicio{repo: repo}
}

// Crear valida la reserva del cliente y genera el código de invitación.
func (s *Servicio) Crear(clienteID uint, entrada EntradaCrear) (Partido, error) {
	if entrada.ReservaID == 0 || entrada.Nombre == "" || entrada.CupoMaximo <= 0 {
		return Partido{}, ErrDatosInvalidos
	}

	reserva, err := s.repo.ReservaPorID(entrada.ReservaID)
	if err != nil {
		// El repositorio traduce el «no encontrado» de GORM a
		// ErrReservaNoEncontrada; el servicio no conoce GORM ni el paquete
		// de reservas más allá del modelo que devuelve la interfaz.
		if errors.Is(err, ErrReservaNoEncontrada) {
			return Partido{}, ErrReservaNoEncontrada
		}
		return Partido{}, fmt.Errorf("%w: %v", errLeyendoReserva, err)
	}
	if reserva.ClienteID != clienteID {
		return Partido{}, ErrReservaAjena
	}
	if reserva.Estado != "confirmada" {
		return Partido{}, ErrReservaNoConfirmada
	}

	if _, err := s.repo.PartidoPorReservaID(entrada.ReservaID); err == nil {
		return Partido{}, ErrPartidoExistente
	} else if !errors.Is(err, ErrNoEncontrado) {
		return Partido{}, fmt.Errorf("%w: %v", errComprobandoPartido, err)
	}

	codigo, err := generarCodigoInvitacion()
	if err != nil {
		return Partido{}, fmt.Errorf("%w: %v", errGenerandoCodigo, err)
	}

	partido := Partido{
		ReservaID:        entrada.ReservaID,
		Nombre:           entrada.Nombre,
		CupoMaximo:       entrada.CupoMaximo,
		CodigoInvitacion: codigo,
		CreadoEn:         time.Now(),
	}
	if err := s.repo.CrearPartido(&partido); err != nil {
		return Partido{}, fmt.Errorf("%w: %v", errGuardandoPartido, err)
	}
	return partido, nil
}

// Obtener devuelve la ficha del partido y el conteo real de inscriptos.
func (s *Servicio) Obtener(id uint) (Detalle, error) {
	partido, err := s.repo.PartidoPorID(id)
	if err != nil {
		if errors.Is(err, ErrNoEncontrado) {
			return Detalle{}, ErrNoEncontrado
		}
		return Detalle{}, fmt.Errorf("%w: %v", errLeyendoPartido, err)
	}

	inscripciones, err := s.repo.ContarInscripciones(partido.ID)
	if err != nil {
		return Detalle{}, fmt.Errorf("%w: %v", errContandoInscripcion, err)
	}

	return Detalle{
		Partido:        partido,
		Inscriptos:     inscripciones,
		CuposRestantes: partido.CupoMaximo - int(inscripciones),
	}, nil
}

// leerAleatorio es una variable para poder probar el camino de error de la
// generación del código sin depender del generador real del sistema.
var leerAleatorio = rand.Read

// generarCodigoInvitacion genera un código único de 6 caracteres.
func generarCodigoInvitacion() (string, error) {
	const caracteres = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // sin 0/O, 1/I/L
	b := make([]byte, 6)
	if _, err := leerAleatorio(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = caracteres[int(b[i])%len(caracteres)]
	}
	return string(b), nil
}
