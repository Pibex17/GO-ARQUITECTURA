package partidos

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"canchas-sinteticas/internal/inscripcion"
	"canchas-sinteticas/internal/reservas"
)

// Repositorio es la parte de la base que necesita el contexto de partidos.
type Repositorio interface {
	ReservaPorID(id uint) (reservas.Reserva, error)
	PartidoPorReservaID(reservaID uint) (Partido, error)
	PartidoPorID(id uint) (Partido, error)
	ContarInscripciones(partidoID uint) (int64, error)
	CrearPartidoEnTx(tx *gorm.DB, partido *Partido) error
	PartidoPorReservaIDEnTx(tx *gorm.DB, reservaID uint) (Partido, error)
	GuardarPartidoEnTx(tx *gorm.DB, partido *Partido) error
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

func (r RepositorioGORM) CrearPartidoEnTx(tx *gorm.DB, partido *Partido) error {
	return tx.Create(partido).Error
}

func (r RepositorioGORM) PartidoPorReservaIDEnTx(tx *gorm.DB, reservaID uint) (Partido, error) {
	var partido Partido
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&partido, "reserva_id = ?", reservaID).Error
	return partido, err
}

func (r RepositorioGORM) GuardarPartidoEnTx(tx *gorm.DB, partido *Partido) error {
	return tx.Save(partido).Error
}
