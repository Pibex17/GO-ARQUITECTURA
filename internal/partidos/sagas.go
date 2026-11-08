package partidos

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"canchas-sinteticas/internal/eventos"
)

// Sagas agrupa los suscriptores del contexto de partidos.
type Sagas struct {
	Repo       Repositorio
	Procesador eventos.Procesador
	Publicador eventos.Publicador
}

// NuevoSagas construye los suscriptores de partidos.
func NuevoSagas(repo Repositorio, procesador eventos.Procesador, publicador eventos.Publicador) *Sagas {
	return &Sagas{Repo: repo, Procesador: procesador, Publicador: publicador}
}

// Registrar conecta el contexto de partidos al bus.
func (s *Sagas) Registrar(bus eventos.Bus) {
	bus.Suscribir(eventos.TipoReservaValidadaParaPartido, s.onReservaValidada)
	bus.Suscribir(eventos.TipoReservaCancelada, s.onReservaCancelada)
}

// onReservaValidada crea el partido de forma idempotente.
func (s *Sagas) onReservaValidada(ctx context.Context, evento eventos.Evento) error {
	datos, err := eventos.Decodificar[eventos.ReservaValidadaParaPartido](evento)
	if err != nil {
		return err
	}

	return s.Procesador.Procesar(ctx, evento, func(tx *gorm.DB) ([]eventos.Evento, error) {
		existente, err := s.Repo.PartidoPorReservaIDEnTx(tx, datos.ReservaID)
		if err == nil {
			return []eventos.Evento{
				eventos.Derivado(evento, eventos.TipoPartidoCreado, datos.ReservaID,
					eventos.PartidoCreado{ReservaID: datos.ReservaID, PartidoID: existente.ID}),
			}, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		partido := Partido{
			ReservaID:        datos.ReservaID,
			Nombre:           datos.Nombre,
			CupoMaximo:       datos.CupoMaximo,
			CodigoInvitacion: generarCodigoInvitacion(),
			CreadoEn:         time.Now().UTC(),
			Estado:           EstadoActivo,
		}
		if err := s.Repo.CrearPartidoEnTx(tx, &partido); err != nil {
			return nil, err
		}
		return []eventos.Evento{
			eventos.Derivado(evento, eventos.TipoPartidoCreado, datos.ReservaID,
				eventos.PartidoCreado{ReservaID: datos.ReservaID, PartidoID: partido.ID}),
		}, nil
	})
}

// onReservaCancelada cancela el partido si ya existía.
func (s *Sagas) onReservaCancelada(ctx context.Context, evento eventos.Evento) error {
	datos, err := eventos.Decodificar[eventos.ReservaCancelada](evento)
	if err != nil {
		return err
	}

	return s.Procesador.Procesar(ctx, evento, func(tx *gorm.DB) ([]eventos.Evento, error) {
		partido, err := s.Repo.PartidoPorReservaIDEnTx(tx, datos.ReservaID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if partido.Estado == EstadoCancelado {
			return nil, nil
		}
		partido.Estado = EstadoCancelado
		if err := s.Repo.GuardarPartidoEnTx(tx, &partido); err != nil {
			return nil, err
		}
		return []eventos.Evento{
			eventos.Derivado(evento, eventos.TipoPartidoCancelado, datos.ReservaID,
				eventos.PartidoCancelado{ReservaID: datos.ReservaID, PartidoID: partido.ID}),
		}, nil
	})
}
