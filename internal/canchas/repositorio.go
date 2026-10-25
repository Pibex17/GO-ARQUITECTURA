package canchas

import "gorm.io/gorm"

// FiltroCanchas describe el catálogo que puede ver una sesión.
type FiltroCanchas struct {
	Activa          *bool
	AdministradorID *uint
}

// Repositorio es la parte de la base que necesita el manejador de canchas.
type Repositorio interface {
	CanchaPorID(id uint) (Cancha, error)
	ListarCanchas(filtro FiltroCanchas) ([]Cancha, error)
	CrearCancha(cancha *Cancha) error
	GuardarCancha(cancha *Cancha) error
}

// RepositorioGORM implementa Repositorio con PostgreSQL/GORM.
type RepositorioGORM struct {
	DB *gorm.DB
}

func (r RepositorioGORM) CanchaPorID(id uint) (Cancha, error) {
	var cancha Cancha
	err := r.DB.First(&cancha, id).Error
	return cancha, err
}

func (r RepositorioGORM) ListarCanchas(filtro FiltroCanchas) ([]Cancha, error) {
	consulta := r.DB.Debug().Preload("Horarios")
	if filtro.Activa != nil {
		consulta = consulta.Where("activa = ?", *filtro.Activa)
	}
	if filtro.AdministradorID != nil {
		consulta = consulta.Where("administrador_id = ?", *filtro.AdministradorID)
	}

	var lista []Cancha
	err := consulta.Find(&lista).Error
	return lista, err
}

func (r RepositorioGORM) CrearCancha(cancha *Cancha) error {
	return r.DB.Debug().Create(cancha).Error
}

func (r RepositorioGORM) GuardarCancha(cancha *Cancha) error {
	return r.DB.Save(cancha).Error
}
