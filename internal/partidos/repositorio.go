package partidos

import (
	"gorm.io/gorm"

	"canchas-sinteticas/internal/inscripcion"
	"canchas-sinteticas/internal/reservas"
)

// Repositorio es la parte de la base que necesita el manejador de partidos.
type Repositorio interface {
	ReservaPorID(id uint) (reservas.Reserva, error)
	PartidoPorReservaID(reservaID uint) (Partido, error)
	PartidoPorID(id uint) (Partido, error)
	ContarInscripciones(partidoID uint) (int64, error)
	CrearPartido(partido *Partido) error
}

// RepositorioGORM implementa Repositorio con PostgreSQL/GORM.
type RepositorioGORM struct {
	DB *gorm.DB
}

func (r RepositorioGORM) ReservaPorID(id uint) (reservas.Reserva, error) {
	var reserva reservas.Reserva
	err := r.DB.First(&reserva, id).Error
	return reserva, err
}

func (r RepositorioGORM) PartidoPorReservaID(reservaID uint) (Partido, error) {
	var partido Partido
	err := r.DB.First(&partido, "reserva_id = ?", reservaID).Error
	return partido, err
}

func (r RepositorioGORM) PartidoPorID(id uint) (Partido, error) {
	var partido Partido
	err := r.DB.First(&partido, id).Error
	return partido, err
}

func (r RepositorioGORM) ContarInscripciones(partidoID uint) (int64, error) {
	var total int64
	err := r.DB.Model(&inscripcion.Inscripcion{}).
		Where("partido_id = ?", partidoID).
		Count(&total).Error
	return total, err
}

func (r RepositorioGORM) CrearPartido(partido *Partido) error {
	return r.DB.Create(partido).Error
}
