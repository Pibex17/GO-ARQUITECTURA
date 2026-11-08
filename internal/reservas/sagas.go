package reservas

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"canchas-sinteticas/internal/eventos"
)

// Sagas agrupa los suscriptores del contexto de reservas.
type Sagas struct {
	Repo       Repositorio
	Procesador eventos.Procesador
	Publicador eventos.Publicador
}

// NuevoSagas construye los suscriptores de reservas.
func NuevoSagas(repo Repositorio, procesador eventos.Procesador, publicador eventos.Publicador) *Sagas {
	return &Sagas{Repo: repo, Procesador: procesador, Publicador: publicador}
}

// Registrar conecta el contexto de reservas al bus.
func (s *Sagas) Registrar(bus eventos.Bus) {
	bus.Suscribir(eventos.TipoSenaRegistrada, s.onSenaRegistrada)
	bus.Suscribir(eventos.TipoSenaVerificada, s.onSenaVerificada)
	bus.Suscribir(eventos.TipoRegistroSenaFallido, s.onFallo)
	bus.Suscribir(eventos.TipoVerificacionSenaFallida, s.onFallo)
	bus.Suscribir(eventos.TipoCancelacionSolicitada, s.onCancelacionSolicitada)
	bus.Suscribir(eventos.TipoJugadaSolicitada, s.onJugadaSolicitada)
	bus.Suscribir(eventos.TipoSolicitudPartidoRecibida, s.onSolicitudPartido)
}

func (s *Sagas) onFallo(context.Context, eventos.Evento) error {
	// La proyección de sagas se encarga de reflejar el fallo. La reserva
	// conserva el estado anterior; no hay nada que escribir aquí.
	return nil
}

// onSenaRegistrada mueve la reserva a pago_por_verificar.
func (s *Sagas) onSenaRegistrada(ctx context.Context, evento eventos.Evento) error {
	datos, err := eventos.Decodificar[eventos.SenaRegistrada](evento)
	if err != nil {
		return err
	}

	err = s.Procesador.Procesar(ctx, evento, func(tx *gorm.DB) ([]eventos.Evento, error) {
		reserva, err := s.Repo.ReservaPorIDEnTx(tx, datos.ReservaID)
		if err != nil {
			return nil, err
		}
		if reserva.Estado == "pago_por_verificar" {
			return nil, nil // idempotente
		}
		if !transicionPermitida(reserva.Estado, "pago_por_verificar") {
			return nil, nil // la reserva se canceló o ya avanzó
		}
		reserva.Estado = "pago_por_verificar"
		if err := s.Repo.GuardarReservaEnTx(tx, &reserva); err != nil {
			return nil, err
		}
		return []eventos.Evento{
			eventos.Derivado(evento, eventos.TipoReservaPasoAPagoPorVerificar, datos.ReservaID,
				eventos.ReservaPasoAPagoPorVerificar{ReservaID: datos.ReservaID}),
		}, nil
	})
	if err == nil {
		return nil
	}

	// El pago ya existe, pero la reserva no pudo guardar el nuevo estado.
	// Se pide al contexto de pagos que anule la seña.
	compensacion := eventos.DerivadoConID(
		evento.ID+"-"+eventos.TipoReservaNoActualizada,
		evento, eventos.TipoReservaNoActualizada, datos.ReservaID,
		eventos.ReservaNoActualizada{ReservaID: datos.ReservaID, Motivo: err.Error()},
	)
	return s.Publicador.Publicar(ctx, compensacion)
}

// onSenaVerificada confirma la reserva.
func (s *Sagas) onSenaVerificada(ctx context.Context, evento eventos.Evento) error {
	datos, err := eventos.Decodificar[eventos.SenaVerificada](evento)
	if err != nil {
		return err
	}

	err = s.Procesador.Procesar(ctx, evento, func(tx *gorm.DB) ([]eventos.Evento, error) {
		reserva, err := s.Repo.ReservaPorIDEnTx(tx, datos.ReservaID)
		if err != nil {
			return nil, err
		}
		if reserva.Estado == "confirmada" {
			return nil, nil
		}
		if !transicionPermitida(reserva.Estado, "confirmada") {
			return nil, nil
		}
		reserva.Estado = "confirmada"
		if err := s.Repo.GuardarReservaEnTx(tx, &reserva); err != nil {
			return nil, err
		}
		return []eventos.Evento{
			eventos.Derivado(evento, eventos.TipoReservaConfirmada, datos.ReservaID,
				eventos.ReservaConfirmada{ReservaID: datos.ReservaID}),
		}, nil
	})
	if err == nil {
		return nil
	}

	compensacion := eventos.DerivadoConID(
		evento.ID+"-"+eventos.TipoReservaNoConfirmada,
		evento, eventos.TipoReservaNoConfirmada, datos.ReservaID,
		eventos.ReservaNoConfirmada{ReservaID: datos.ReservaID, Motivo: err.Error()},
	)
	return s.Publicador.Publicar(ctx, compensacion)
}

// onCancelacionSolicitada aplica la cancelación.
func (s *Sagas) onCancelacionSolicitada(ctx context.Context, evento eventos.Evento) error {
	datos, err := eventos.Decodificar[eventos.CancelacionSolicitada](evento)
	if err != nil {
		return err
	}

	return s.Procesador.Procesar(ctx, evento, func(tx *gorm.DB) ([]eventos.Evento, error) {
		reserva, err := s.Repo.ReservaPorIDEnTx(tx, datos.ReservaID)
		if err != nil {
			return nil, err
		}
		if reserva.Estado == "cancelada" {
			return nil, nil
		}
		if !transicionPermitida(reserva.Estado, "cancelada") {
			return nil, nil
		}
		reserva.Estado = "cancelada"
		if err := s.Repo.GuardarReservaEnTx(tx, &reserva); err != nil {
			return nil, err
		}
		return []eventos.Evento{
			eventos.Derivado(evento, eventos.TipoReservaCancelada, datos.ReservaID,
				eventos.ReservaCancelada{ReservaID: datos.ReservaID}),
		}, nil
	})
}

// onJugadaSolicitada marca la reserva como jugada.
func (s *Sagas) onJugadaSolicitada(ctx context.Context, evento eventos.Evento) error {
	datos, err := eventos.Decodificar[eventos.JugadaSolicitada](evento)
	if err != nil {
		return err
	}

	return s.Procesador.Procesar(ctx, evento, func(tx *gorm.DB) ([]eventos.Evento, error) {
		reserva, err := s.Repo.ReservaPorIDEnTx(tx, datos.ReservaID)
		if err != nil {
			return nil, err
		}
		if reserva.Estado == "jugada" {
			return nil, nil
		}
		if !transicionPermitida(reserva.Estado, "jugada") {
			return nil, nil
		}
		reserva.Estado = "jugada"
		if err := s.Repo.GuardarReservaEnTx(tx, &reserva); err != nil {
			return nil, err
		}
		return []eventos.Evento{
			eventos.Derivado(evento, eventos.TipoReservaJugada, datos.ReservaID,
				eventos.ReservaJugada{ReservaID: datos.ReservaID}),
		}, nil
	})
}

// onSolicitudPartido valida la reserva y habilita la creación del partido.
func (s *Sagas) onSolicitudPartido(ctx context.Context, evento eventos.Evento) error {
	datos, err := eventos.Decodificar[eventos.SolicitudPartidoRecibida](evento)
	if err != nil {
		return err
	}

	err = s.Procesador.Procesar(ctx, evento, func(tx *gorm.DB) ([]eventos.Evento, error) {
		reserva, err := s.Repo.ReservaPorIDEnTx(tx, datos.ReservaID)
		if err != nil {
			return []eventos.Evento{
				eventos.Derivado(evento, eventos.TipoPartidoFallido, datos.ReservaID,
					eventos.PartidoFallido{ReservaID: datos.ReservaID, Motivo: "no se pudo leer la reserva"}),
			}, nil
		}
		if reserva.ClienteID != datos.ClienteID {
			return []eventos.Evento{
				eventos.Derivado(evento, eventos.TipoPartidoFallido, datos.ReservaID,
					eventos.PartidoFallido{ReservaID: datos.ReservaID, Motivo: "la reserva no pertenece al cliente"}),
			}, nil
		}
		if reserva.Estado != "confirmada" {
			return []eventos.Evento{
				eventos.Derivado(evento, eventos.TipoPartidoFallido, datos.ReservaID,
					eventos.PartidoFallido{ReservaID: datos.ReservaID, Motivo: "la reserva no está confirmada"}),
			}, nil
		}
		return []eventos.Evento{
			eventos.Derivado(evento, eventos.TipoReservaValidadaParaPartido, datos.ReservaID,
				eventos.ReservaValidadaParaPartido{
					ReservaID:  datos.ReservaID,
					ClienteID:  datos.ClienteID,
					Nombre:     datos.Nombre,
					CupoMaximo: datos.CupoMaximo,
				}),
		}, nil
	})
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return err
}
