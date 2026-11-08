package pagos

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"canchas-sinteticas/internal/eventos"
)

// Sagas agrupa los suscriptores del contexto de pagos.
type Sagas struct {
	Repo       Repositorio
	Procesador eventos.Procesador
	Publicador eventos.Publicador
}

// NuevoSagas construye los suscriptores de pagos.
func NuevoSagas(repo Repositorio, procesador eventos.Procesador, publicador eventos.Publicador) *Sagas {
	return &Sagas{Repo: repo, Procesador: procesador, Publicador: publicador}
}

// Registrar conecta el contexto de pagos al bus.
func (s *Sagas) Registrar(bus eventos.Bus) {
	bus.Suscribir(eventos.TipoSenaDeclarada, s.onSenaDeclarada)
	bus.Suscribir(eventos.TipoSenaAprobada, s.onSenaAprobada)
	bus.Suscribir(eventos.TipoReservaCancelada, s.onReservaCancelada)
	bus.Suscribir(eventos.TipoReservaNoActualizada, s.onReservaNoActualizada)
	bus.Suscribir(eventos.TipoReservaNoConfirmada, s.onReservaNoConfirmada)
	bus.Suscribir(eventos.TipoProcesamientoFallido, s.onProcesamientoFallido)
}

// onSenaDeclarada crea la seña del 30 %. Si ya existe, es idempotente.
func (s *Sagas) onSenaDeclarada(ctx context.Context, evento eventos.Evento) error {
	datos, err := eventos.Decodificar[eventos.SenaDeclarada](evento)
	if err != nil {
		return err
	}

	err = s.Procesador.Procesar(ctx, evento, func(tx *gorm.DB) ([]eventos.Evento, error) {
		if existente, err := s.Repo.SenaPorReservaEnTx(tx, datos.ReservaID); err == nil {
			return []eventos.Evento{
				eventos.Derivado(evento, eventos.TipoSenaRegistrada, datos.ReservaID,
					eventos.SenaRegistrada{
						ReservaID: datos.ReservaID,
						PagoID:    existente.ID,
						Monto:     existente.Monto,
					}),
			}, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		pago := NuevoPagoSena(datos.ReservaID, datos.PrecioHora, datos.Metodo, datos.ComprobanteURL)
		if err := s.Repo.CrearEnTx(tx, &pago); err != nil {
			return nil, err
		}
		return []eventos.Evento{
			eventos.Derivado(evento, eventos.TipoSenaRegistrada, datos.ReservaID,
				eventos.SenaRegistrada{
					ReservaID: datos.ReservaID,
					PagoID:    pago.ID,
					Monto:     pago.Monto,
				}),
		}, nil
	})
	if err == nil {
		return nil
	}

	fallo := eventos.DerivadoConID(
		evento.ID+"-"+eventos.TipoRegistroSenaFallido,
		evento, eventos.TipoRegistroSenaFallido, datos.ReservaID,
		eventos.RegistroSenaFallido{ReservaID: datos.ReservaID, Motivo: err.Error()},
	)
	return s.Publicador.Publicar(ctx, fallo)
}

// onSenaAprobada verifica la seña pendiente.
func (s *Sagas) onSenaAprobada(ctx context.Context, evento eventos.Evento) error {
	datos, err := eventos.Decodificar[eventos.SenaAprobada](evento)
	if err != nil {
		return err
	}

	err = s.Procesador.Procesar(ctx, evento, func(tx *gorm.DB) ([]eventos.Evento, error) {
		pago, err := s.Repo.MarcarVerificadoEnTx(tx, datos.ReservaID)
		if err != nil {
			return nil, err
		}
		return []eventos.Evento{
			eventos.Derivado(evento, eventos.TipoSenaVerificada, datos.ReservaID,
				eventos.SenaVerificada{ReservaID: datos.ReservaID, PagoID: pago.ID}),
		}, nil
	})
	if err == nil {
		return nil
	}

	fallo := eventos.DerivadoConID(
		evento.ID+"-"+eventos.TipoVerificacionSenaFallida,
		evento, eventos.TipoVerificacionSenaFallida, datos.ReservaID,
		eventos.VerificacionSenaFallida{ReservaID: datos.ReservaID, Motivo: err.Error()},
	)
	return s.Publicador.Publicar(ctx, fallo)
}

// onReservaCancelada anula la seña si existe.
func (s *Sagas) onReservaCancelada(ctx context.Context, evento eventos.Evento) error {
	return s.anularSena(ctx, evento)
}

// onReservaNoActualizada compensa una reserva que no pudo guardar el paso
// a pago_por_verificar.
func (s *Sagas) onReservaNoActualizada(ctx context.Context, evento eventos.Evento) error {
	return s.anularSena(ctx, evento)
}

func (s *Sagas) anularSena(ctx context.Context, evento eventos.Evento) error {
	payload, err := eventos.Decodificar[eventos.ReservaCancelada](evento)
	if err != nil {
		return err
	}

	return s.Procesador.Procesar(ctx, evento, func(tx *gorm.DB) ([]eventos.Evento, error) {
		pago, err := s.Repo.MarcarAnuladoEnTx(tx, payload.ReservaID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // no había seña que anular
		}
		if err != nil {
			return nil, err
		}
		return []eventos.Evento{
			eventos.Derivado(evento, eventos.TipoPagoAnulado, payload.ReservaID,
				eventos.PagoAnulado{ReservaID: payload.ReservaID, PagoID: pago.ID}),
		}, nil
	})
}

// onReservaNoConfirmada revierte la verificación de la seña.
func (s *Sagas) onReservaNoConfirmada(ctx context.Context, evento eventos.Evento) error {
	payload, err := eventos.Decodificar[eventos.ReservaNoConfirmada](evento)
	if err != nil {
		return err
	}

	return s.Procesador.Procesar(ctx, evento, func(tx *gorm.DB) ([]eventos.Evento, error) {
		pago, err := s.Repo.RevertirVerificacionEnTx(tx, payload.ReservaID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return []eventos.Evento{
			eventos.Derivado(evento, eventos.TipoVerificacionRevertida, payload.ReservaID,
				eventos.VerificacionRevertida{ReservaID: payload.ReservaID, PagoID: pago.ID}),
		}, nil
	})
}

// onProcesamientoFallido compensa cuando un evento agotó sus reintentos.
func (s *Sagas) onProcesamientoFallido(ctx context.Context, evento eventos.Evento) error {
	datos, err := eventos.Decodificar[eventos.ProcesamientoFallido](evento)
	if err != nil {
		return err
	}

	switch datos.TipoEvento {
	case eventos.TipoSenaRegistrada:
		return s.Procesador.Procesar(ctx, evento, func(tx *gorm.DB) ([]eventos.Evento, error) {
			pago, err := s.Repo.MarcarAnuladoEnTx(tx, datos.AgregadoID)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			return []eventos.Evento{
				eventos.Derivado(evento, eventos.TipoPagoAnulado, datos.AgregadoID,
					eventos.PagoAnulado{ReservaID: datos.AgregadoID, PagoID: pago.ID}),
			}, nil
		})

	case eventos.TipoSenaVerificada:
		return s.Procesador.Procesar(ctx, evento, func(tx *gorm.DB) ([]eventos.Evento, error) {
			pago, err := s.Repo.RevertirVerificacionEnTx(tx, datos.AgregadoID)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil
			}
			if err != nil {
				return nil, err
			}
			return []eventos.Evento{
				eventos.Derivado(evento, eventos.TipoVerificacionRevertida, datos.AgregadoID,
					eventos.VerificacionRevertida{ReservaID: datos.AgregadoID, PagoID: pago.ID}),
			}, nil
		})
	}
	return nil
}
