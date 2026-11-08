package eventos

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// EventoProcesado registra qué consumidor ya procesó un evento. La clave
// compuesta permite que dos módulos distintos procesen el mismo evento.
type EventoProcesado struct {
	EventoID    string    `gorm:"primaryKey;size:64"`
	Consumidor  string    `gorm:"primaryKey;size:64"`
	ProcesadoEn time.Time `gorm:"not null"`
}

// TableName fija el nombre de la tabla de idempotencia.
func (EventoProcesado) TableName() string { return "eventos_procesados" }

// FuncionProceso ejecuta el cambio de estado del consumidor y devuelve los
// eventos nuevos que deben quedar en el outbox, todo dentro de la misma
// transacción.
type FuncionProceso func(tx *gorm.DB) ([]Evento, error)

// Procesador ejecuta un consumidor de forma idempotente.
type Procesador interface {
	Procesar(ctx context.Context, evento Evento, fn FuncionProceso) error
}

// ProcesadorGORM implementa Procesador con PostgreSQL.
type ProcesadorGORM struct {
	DB         *gorm.DB
	Repo       RepositorioOutbox
	Consumidor string
}

// NuevoProcesadorGORM construye un procesador para un consumidor concreto.
func NuevoProcesadorGORM(db *gorm.DB, repo RepositorioOutbox, consumidor string) *ProcesadorGORM {
	return &ProcesadorGORM{DB: db, Repo: repo, Consumidor: consumidor}
}

// Procesar inserta la marca de idempotencia, ejecuta el cambio y guarda los
// eventos resultantes en el outbox dentro de la misma transacción.
// Si el evento ya fue procesado, no ejecuta el cambio y devuelve nil.
func (p *ProcesadorGORM) Procesar(ctx context.Context, evento Evento, fn FuncionProceso) error {
	return p.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		resultado := tx.Exec(
			`INSERT INTO eventos_procesados (evento_id, consumidor, procesado_en)
			 VALUES (?, ?, ?)
			 ON CONFLICT (evento_id, consumidor) DO NOTHING`,
			evento.ID, p.Consumidor, time.Now().UTC(),
		)
		if resultado.Error != nil {
			return resultado.Error
		}
		if resultado.RowsAffected == 0 {
			return nil // ya procesado por este consumidor
		}

		nuevos, err := fn(tx)
		if err != nil {
			return err
		}
		for i := range nuevos {
			if err := p.Repo.Guardar(tx, &nuevos[i]); err != nil {
				return err
			}
		}
		return nil
	})
}
