package sagas

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"canchas-sinteticas/internal/eventos"
)

// Proyeccion escucha todos los eventos y actualiza la tabla sagas.
// Es un observador, no un orquestador: no publica eventos ni decide pasos.
type Proyeccion struct {
	Repo Repositorio
}

// NuevaProyeccion construye la proyección.
func NuevaProyeccion(repo Repositorio) *Proyeccion {
	return &Proyeccion{Repo: repo}
}

// Registrar se suscribe a todos los eventos.
func (p *Proyeccion) Registrar(bus eventos.Bus) {
	bus.Suscribir("*", p.aplicar)
}

func (p *Proyeccion) aplicar(_ context.Context, evento eventos.Evento) error {
	if evento.SagaID == "" {
		return nil
	}

	saga, err := p.Repo.PorID(evento.SagaID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Solo ReservaCreada abre una saga. Los demás eventos deben
		// encontrar una proyección creada por el manejador o por esta
		// misma proyección.
		if evento.Tipo != eventos.TipoReservaCreada {
			return nil
		}
		ahora := time.Now().UTC()
		saga = Saga{
			ID:            evento.SagaID,
			Tipo:          TipoCicloReserva,
			AgregadoID:    evento.AgregadoID,
			Estado:        EstadoIniciada,
			Paso:          "reserva_creada",
			RecursoID:     evento.AgregadoID,
			CreadaEn:      ahora,
			ActualizadaEn: ahora,
		}
		if err := p.Repo.Crear(&saga); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// La entrega es at-least-once y un evento viejo puede reaparecer después
	// de uno más nuevo. La proyección no debe retroceder: si el evento ya se
	// aplicó o su ocurrencia es anterior al último aplicado, se ignora.
	if evento.ID == saga.UltimoEventoID {
		return nil
	}
	if !saga.UltimoEventoEn.IsZero() && evento.OcurridoEn.Before(saga.UltimoEventoEn) {
		return nil
	}

	if !aplicarCambio(&saga, evento) {
		return nil
	}

	saga.UltimoEventoID = evento.ID
	saga.UltimoEventoEn = evento.OcurridoEn
	saga.ActualizadaEn = time.Now().UTC()
	return p.Repo.Guardar(&saga)
}

// aplicarCambio traduce un evento a un paso de la saga.
func aplicarCambio(saga *Saga, evento eventos.Evento) bool {
	switch evento.Tipo {
	case eventos.TipoReservaCreada:
		saga.Estado = EstadoIniciada
		saga.Paso = "reserva_creada"
		saga.RecursoID = evento.AgregadoID

	case eventos.TipoSenaDeclarada:
		saga.Estado = EstadoEnProceso
		saga.Paso = "sena_declarada"

	case eventos.TipoSenaRegistrada:
		saga.Paso = "sena_registrada"

	case eventos.TipoReservaPasoAPagoPorVerificar:
		saga.Paso = "pago_por_verificar"

	case eventos.TipoSenaAprobada:
		saga.Paso = "sena_aprobada"

	case eventos.TipoSenaVerificada:
		saga.Paso = "sena_verificada"

	case eventos.TipoReservaConfirmada:
		saga.Paso = "reserva_confirmada"

	case eventos.TipoRegistroSenaFallido:
		datos, _ := eventos.Decodificar[eventos.RegistroSenaFallido](evento)
		saga.Estado = EstadoFallida
		saga.Paso = "registro_sena_fallido"
		saga.Error = datos.Motivo

	case eventos.TipoVerificacionSenaFallida:
		datos, _ := eventos.Decodificar[eventos.VerificacionSenaFallida](evento)
		saga.Estado = EstadoFallida
		saga.Paso = "verificacion_sena_fallida"
		saga.Error = datos.Motivo

	case eventos.TipoReservaNoActualizada:
		datos, _ := eventos.Decodificar[eventos.ReservaNoActualizada](evento)
		saga.Estado = EstadoCompensando
		saga.Paso = "compensando_reserva"
		saga.Error = datos.Motivo

	case eventos.TipoReservaNoConfirmada:
		datos, _ := eventos.Decodificar[eventos.ReservaNoConfirmada](evento)
		saga.Estado = EstadoCompensando
		saga.Paso = "compensando_confirmacion"
		saga.Error = datos.Motivo

	case eventos.TipoPagoAnulado:
		// Una cancelación conserva su estado terminal; la anulación es
		// una compensación dentro de ella.
		if saga.Estado != EstadoCancelada {
			saga.Estado = EstadoCompensada
		}
		saga.Paso = "pago_anulado"

	case eventos.TipoVerificacionRevertida:
		if saga.Estado != EstadoCancelada {
			saga.Estado = EstadoCompensada
		}
		saga.Paso = "verificacion_revertida"

	case eventos.TipoCancelacionSolicitada:
		saga.Estado = EstadoCompensando
		saga.Paso = "cancelacion_solicitada"

	case eventos.TipoReservaCancelada:
		saga.Estado = EstadoCancelada
		saga.Paso = "reserva_cancelada"

	case eventos.TipoPartidoCancelado:
		saga.Paso = "partido_cancelado"

	case eventos.TipoJugadaSolicitada:
		saga.Estado = EstadoEnProceso
		saga.Paso = "jugada_solicitada"

	case eventos.TipoReservaJugada:
		saga.Estado = EstadoCompletada
		saga.Paso = "jugada"

	case eventos.TipoSolicitudPartidoRecibida:
		saga.Estado = EstadoEnProceso
		saga.Paso = "solicitud_partido"

	case eventos.TipoReservaValidadaParaPartido:
		saga.Paso = "reserva_validada"

	case eventos.TipoPartidoCreado:
		datos, _ := eventos.Decodificar[eventos.PartidoCreado](evento)
		saga.Estado = EstadoCompletada
		saga.Paso = "partido_creado"
		saga.RecursoID = datos.PartidoID

	case eventos.TipoPartidoFallido:
		datos, _ := eventos.Decodificar[eventos.PartidoFallido](evento)
		saga.Estado = EstadoFallida
		saga.Paso = "partido_fallido"
		saga.Error = datos.Motivo

	case eventos.TipoProcesamientoFallido:
		datos, _ := eventos.Decodificar[eventos.ProcesamientoFallido](evento)
		saga.Estado = EstadoFallida
		saga.Paso = "procesamiento_fallido"
		saga.Error = datos.Motivo

	default:
		return false
	}
	return true
}
