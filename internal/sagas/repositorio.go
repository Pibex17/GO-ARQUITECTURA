package sagas

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repositorio es la parte de la base que necesita la proyección de sagas.
type Repositorio interface {
	Crear(saga *Saga) error
	PorID(id string) (Saga, error)
	PorAgregado(tipo string, agregadoID uint) (Saga, error)
	Guardar(saga *Saga) error
}

// RepositorioGORM implementa Repositorio con PostgreSQL.
type RepositorioGORM struct {
	DB *gorm.DB
}

func (r RepositorioGORM) Crear(saga *Saga) error {
	return r.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(saga).Error
}

func (r RepositorioGORM) PorID(id string) (Saga, error) {
	var saga Saga
	err := r.DB.First(&saga, "id = ?", id).Error
	return saga, err
}

func (r RepositorioGORM) PorAgregado(tipo string, agregadoID uint) (Saga, error) {
	var saga Saga
	err := r.DB.First(&saga, "tipo = ? AND agregado_id = ?", tipo, agregadoID).Error
	return saga, err
}

func (r RepositorioGORM) Guardar(saga *Saga) error {
	return r.DB.Save(saga).Error
}
