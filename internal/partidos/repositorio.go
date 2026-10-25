package partidos

import (
	"gorm.io/gorm"

	"canchas-sinteticas/internal/reservas"
)

// Repositorio es la parte de la base que necesita el manejador de partidos.
type Repositorio interface {
	ReservaPorID(id uint) (reservas.Reserva, error)
	PartidoPorReservaID(reservaID uint) (Partido, error)
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

func (r RepositorioGORM) CrearPartido(partido *Partido) error {
	return r.DB.Create(partido).Error
}
