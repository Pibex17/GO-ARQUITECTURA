package partidos

import (
	"errors"

	"gorm.io/gorm"

	"canchas-sinteticas/internal/inscripcion"
	"canchas-sinteticas/internal/reservas"
)

// RepositorioGORM implementa Repositorio con PostgreSQL/GORM.
type RepositorioGORM struct {
	DB *gorm.DB
}

func (r RepositorioGORM) ReservaPorID(id uint) (reservas.Reserva, error) {
	var reserva reservas.Reserva
	err := r.DB.First(&reserva, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// El repositorio traduce el «no encontrado» de GORM al error de
		// dominio de este módulo, para que el servicio no dependa del
		// paquete reservas.
		return reservas.Reserva{}, ErrReservaNoEncontrada
	}
	return reserva, err
}

func (r RepositorioGORM) PartidoPorReservaID(reservaID uint) (Partido, error) {
	var partido Partido
	err := r.DB.First(&partido, "reserva_id = ?", reservaID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Partido{}, ErrNoEncontrado
	}
	return partido, err
}

func (r RepositorioGORM) PartidoPorID(id uint) (Partido, error) {
	var partido Partido
	err := r.DB.First(&partido, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Partido{}, ErrNoEncontrado
	}
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
