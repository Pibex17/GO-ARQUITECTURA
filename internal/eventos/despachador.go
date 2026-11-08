package eventos

import (
	"context"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"
)

// Despachador toma los eventos pendientes del outbox y los entrega al bus.
// Es la única pieza que mueve eventos en segundo plano; los casos de uso
// solo escriben en el outbox.
type Despachador struct {
	DB               *gorm.DB
	Repo             RepositorioOutbox
	Bus              Bus
	Intervalo        time.Duration
	AlAgotarIntentos func(ctx context.Context, evento Evento, causa error)

	mu sync.Mutex
}

// NuevoDespachador construye el despachador con un intervalo corto: la API
// responde 202 y el cliente consulta la saga casi de inmediato.
func NuevoDespachador(db *gorm.DB, repo RepositorioOutbox, bus Bus) *Despachador {
	return &Despachador{
		DB:        db,
		Repo:      repo,
		Bus:       bus,
		Intervalo: 100 * time.Millisecond,
	}
}

// Ejecutar corre hasta que el contexto se cancele.
func (d *Despachador) Ejecutar(ctx context.Context) {
	ticker := time.NewTicker(d.Intervalo)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := d.DespacharPendientes(ctx); err != nil {
				log.Printf("despachador de eventos: %v", err)
			}
		}
	}
}

// DespacharPendientes procesa un lote de eventos pendientes.
func (d *Despachador) DespacharPendientes(ctx context.Context) error {
	// Una sola instancia del servidor: el mutex evita que dos ticks
	// concurrentes entreguen el mismo evento dos veces.
	d.mu.Lock()
	defer d.mu.Unlock()

	lista, err := d.Repo.Pendientes(100)
	if err != nil {
		return err
	}

	for _, evento := range lista {
		if err := d.Bus.Publicar(ctx, evento); err != nil {
			intentos := evento.Intentos + 1
			_ = d.Repo.MarcarIntentoFallido(evento.ID, err)
			if intentos >= MaxIntentos && d.AlAgotarIntentos != nil {
				d.AlAgotarIntentos(ctx, evento, err)
			}
			continue
		}
		if err := d.Repo.MarcarPublicado(evento.ID); err != nil {
			return err
		}
	}
	return nil
}
