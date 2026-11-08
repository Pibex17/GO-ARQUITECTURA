package eventos

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// RepositorioOutbox guarda y consulta los eventos pendientes de publicar.
type RepositorioOutbox interface {
	Guardar(tx *gorm.DB, evento *Evento) error
	Pendientes(limite int) ([]Evento, error)
	MarcarPublicado(id string) error
	MarcarIntentoFallido(id string, causa error) error
}

// RepositorioOutboxGORM implementa RepositorioOutbox con PostgreSQL.
type RepositorioOutboxGORM struct {
	DB *gorm.DB
}

func (r RepositorioOutboxGORM) Guardar(tx *gorm.DB, evento *Evento) error {
	return tx.Create(evento).Error
}

func (r RepositorioOutboxGORM) Pendientes(limite int) ([]Evento, error) {
	var lista []Evento
	err := r.DB.
		Where("estado = ?", EstadoPendiente).
		Order("ocurrido_en asc").
		Limit(limite).
		Find(&lista).Error
	return lista, err
}

func (r RepositorioOutboxGORM) MarcarPublicado(id string) error {
	ahora := time.Now().UTC()
	return r.DB.Model(&Evento{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"estado":       EstadoPublicado,
			"publicado_en": &ahora,
			"error":        "",
		}).Error
}

// MaxIntentos es el número de entregas antes de considerar un evento
// definitivamente fallido.
const MaxIntentos = 5

func (r RepositorioOutboxGORM) MarcarIntentoFallido(id string, causa error) error {
	var evento Evento
	if err := r.DB.First(&evento, "id = ?", id).Error; err != nil {
		return err
	}
	evento.Intentos++
	if causa != nil {
		evento.Error = causa.Error()
	}
	if evento.Intentos >= MaxIntentos {
		evento.Estado = EstadoFallido
	} else {
		evento.Estado = EstadoPendiente
	}
	return r.DB.Save(&evento).Error
}

// Publicador es la pieza que usan los casos de uso para dejar un evento
// en el outbox.
type Publicador interface {
	Publicar(ctx context.Context, evento Evento) error
}

// PublicadorGORM guarda el evento en su propia transacción.
type PublicadorGORM struct {
	DB   *gorm.DB
	Repo RepositorioOutbox
}

// NuevoPublicador construye el publicador por defecto.
func NuevoPublicador(db *gorm.DB, repo RepositorioOutbox) *PublicadorGORM {
	return &PublicadorGORM{DB: db, Repo: repo}
}

func (p *PublicadorGORM) Publicar(ctx context.Context, evento Evento) error {
	return p.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return p.Repo.Guardar(tx, &evento)
	})
}

// EnTx guarda el evento dentro de una transacción que ya está en curso.
// Se usa para que el cambio de estado y el evento sean atómicos.
func (p *PublicadorGORM) EnTx(tx *gorm.DB, evento *Evento) error {
	return p.Repo.Guardar(tx, evento)
}
