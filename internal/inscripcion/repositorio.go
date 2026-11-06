package inscripcion

import (
	"errors"

	"gorm.io/gorm"
)

// Partido es la vista mínima de la tabla partidos que necesita la
// inscripción. Evita importar el paquete partidos y crear un ciclo.
type Partido struct {
	ID               uint
	CodigoInvitacion string
	CupoMaximo       int
}

// RepositorioGORM implementa Repositorio con PostgreSQL/GORM.
type RepositorioGORM struct {
	DB *gorm.DB
}

func (r RepositorioGORM) PartidoPorID(id uint) (Partido, error) {
	var partido Partido
	err := r.DB.Table("partidos").First(&partido, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Partido{}, ErrNoEncontrado
	}
	return partido, err
}

func (r RepositorioGORM) InscripcionPorPartidoYJugador(partidoID, jugadorID uint) (Inscripcion, error) {
	var existente Inscripcion
	err := r.DB.First(&existente, "partido_id = ? AND jugador_id = ?", partidoID, jugadorID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Inscripcion{}, ErrNoEncontrado
	}
	return existente, err
}

func (r RepositorioGORM) ContarInscripciones(partidoID uint) (int64, error) {
	var total int64
	err := r.DB.Model(&Inscripcion{}).Where("partido_id = ?", partidoID).Count(&total).Error
	return total, err
}

func (r RepositorioGORM) CrearInscripcion(inscripcion *Inscripcion) error {
	return r.DB.Create(inscripcion).Error
}
