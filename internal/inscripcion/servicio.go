package inscripcion

import (
	"errors"
	"fmt"
	"time"
)

// Errores de negocio de inscripciones. El transporte los traduce a HTTP.
var (
	ErrDatosInvalidos = errors.New("codigo_invitacion es obligatorio")
	ErrNoEncontrado   = errors.New("no existe un partido con ese id")
	ErrCodigoInvalido = errors.New("el código de invitación no es válido")
	ErrYaInscripto    = errors.New("el jugador ya está inscrito en este partido")
	ErrPartidoLleno   = errors.New("el partido ya no tiene cupos disponibles")

	errLeyendoPartido         = errors.New("no se pudo leer el partido")
	errComprobandoInscripcion = errors.New("no se pudo comprobar la inscripción")
	errContandoInscripciones  = errors.New("no se pudo contar los inscriptos")
	errGuardandoInscripcion   = errors.New("no se pudo guardar la inscripción")
)

// Repositorio es la persistencia que necesita el servicio. La interfaz vive
// en la capa de servicio; la implementación GORM está en repositorio.go.
type Repositorio interface {
	PartidoPorID(id uint) (Partido, error)
	InscripcionPorPartidoYJugador(partidoID, jugadorID uint) (Inscripcion, error)
	ContarInscripciones(partidoID uint) (int64, error)
	CrearInscripcion(inscripcion *Inscripcion) error
}

// EntradaCrear son los datos de negocio de una inscripción.
type EntradaCrear struct {
	CodigoInvitacion string
}

// Servicio contiene los casos de uso de inscripciones a partidos.
type Servicio struct {
	repo Repositorio
}

// NuevoServicio inyecta la persistencia que necesitan los casos de uso.
func NuevoServicio(repo Repositorio) *Servicio {
	return &Servicio{repo: repo}
}

// Crear valida el código, la duplicidad y el cupo antes de inscribir.
func (s *Servicio) Crear(partidoID, jugadorID uint, entrada EntradaCrear) (Inscripcion, error) {
	if entrada.CodigoInvitacion == "" {
		return Inscripcion{}, ErrDatosInvalidos
	}

	partido, err := s.repo.PartidoPorID(partidoID)
	if err != nil {
		if errors.Is(err, ErrNoEncontrado) {
			return Inscripcion{}, ErrNoEncontrado
		}
		return Inscripcion{}, fmt.Errorf("%w: %v", errLeyendoPartido, err)
	}

	if partido.CodigoInvitacion != entrada.CodigoInvitacion {
		return Inscripcion{}, ErrCodigoInvalido
	}

	if _, err := s.repo.InscripcionPorPartidoYJugador(partidoID, jugadorID); err == nil {
		return Inscripcion{}, ErrYaInscripto
	} else if !errors.Is(err, ErrNoEncontrado) {
		return Inscripcion{}, fmt.Errorf("%w: %v", errComprobandoInscripcion, err)
	}

	inscripciones, err := s.repo.ContarInscripciones(partidoID)
	if err != nil {
		return Inscripcion{}, fmt.Errorf("%w: %v", errContandoInscripciones, err)
	}
	if int(inscripciones) >= partido.CupoMaximo {
		return Inscripcion{}, ErrPartidoLleno
	}

	nueva := Inscripcion{
		PartidoID: partidoID,
		JugadorID: jugadorID,
		CreadoEn:  time.Now(),
	}
	if err := s.repo.CrearInscripcion(&nueva); err != nil {
		return Inscripcion{}, fmt.Errorf("%w: %v", errGuardandoInscripcion, err)
	}
	return nueva, nil
}
