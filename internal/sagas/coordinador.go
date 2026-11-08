package sagas

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"canchas-sinteticas/internal/eventos"
)

// Registro es la interfaz que usan los manejadores HTTP para asegurar que
// existe la saga de una reserva y para consultar su estado.
type Registro interface {
	Asegurar(ctx context.Context, tipo string, agregadoID uint) (Saga, error)
	Obtener(ctx context.Context, id string) (Saga, error)
}

// Coordinador crea y consulta proyecciones. No orquesta la saga: los pasos
// los deciden los consumidores de cada contexto.
type Coordinador struct {
	Repo Repositorio
}

// NuevoCoordinador construye el coordinador de proyecciones.
func NuevoCoordinador(repo Repositorio) *Coordinador {
	return &Coordinador{Repo: repo}
}

// Asegurar devuelve la saga de un agregado y la crea si todavía no existe.
// La creación es idempotente para tolerar peticiones concurrentes.
func (c *Coordinador) Asegurar(_ context.Context, tipo string, agregadoID uint) (Saga, error) {
	id := ""
	if tipo == TipoCicloReserva {
		id = IDDeReserva(agregadoID)
	}

	if id != "" {
		saga, err := c.Repo.PorID(id)
		if err == nil {
			return saga, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return Saga{}, err
		}
	}

	ahora := time.Now().UTC()
	saga := Saga{
		ID:            id,
		Tipo:          tipo,
		AgregadoID:    agregadoID,
		Estado:        EstadoIniciada,
		Paso:          "iniciada",
		RecursoID:     agregadoID,
		CreadaEn:      ahora,
		ActualizadaEn: ahora,
	}
	if saga.ID == "" {
		saga.ID = eventos.NuevoID()
	}

	if err := c.Repo.Crear(&saga); err != nil {
		return Saga{}, err
	}
	if guardada, err := c.Repo.PorID(saga.ID); err == nil {
		return guardada, nil
	}
	return saga, nil
}

// Obtener devuelve una saga por su identificador.
func (c *Coordinador) Obtener(_ context.Context, id string) (Saga, error) {
	return c.Repo.PorID(id)
}
