package sagas

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"canchas-sinteticas/internal/eventos"
)

type repoFalso struct {
	sagas map[string]Saga
	err   error
}

func nuevoRepoFalso() *repoFalso {
	return &repoFalso{sagas: make(map[string]Saga)}
}

func (r *repoFalso) Crear(saga *Saga) error {
	if r.err != nil {
		return r.err
	}
	if _, existe := r.sagas[saga.ID]; !existe {
		r.sagas[saga.ID] = *saga
	}
	return nil
}

func (r *repoFalso) PorID(id string) (Saga, error) {
	if r.err != nil {
		return Saga{}, r.err
	}
	saga, existe := r.sagas[id]
	if !existe {
		return Saga{}, gorm.ErrRecordNotFound
	}
	return saga, nil
}

func (r *repoFalso) PorAgregado(tipo string, agregadoID uint) (Saga, error) {
	for _, saga := range r.sagas {
		if saga.Tipo == tipo && saga.AgregadoID == agregadoID {
			return saga, nil
		}
	}
	return Saga{}, gorm.ErrRecordNotFound
}

func (r *repoFalso) Guardar(saga *Saga) error {
	if r.err != nil {
		return r.err
	}
	r.sagas[saga.ID] = *saga
	return nil
}

func TestProyeccionRecorreElCicloDeLaReserva(t *testing.T) {
	repo := nuevoRepoFalso()
	proyeccion := NuevaProyeccion(repo)
	ctx := context.Background()

	creada := eventos.NuevoEvento(eventos.TipoReservaCreada, 1, "reserva-1", "corr", "",
		eventos.ReservaCreada{ReservaID: 1})
	if err := proyeccion.aplicar(ctx, creada); err != nil {
		t.Fatalf("ReservaCreada: %v", err)
	}
	saga := repo.sagas["reserva-1"]
	if saga.Estado != EstadoIniciada || saga.Paso != "reserva_creada" {
		t.Fatalf("saga inicial inesperada: %+v", saga)
	}

	pasos := []struct {
		evento      eventos.Evento
		estado      string
		paso        string
		recursoID   uint
		errorEspera string
	}{
		{
			evento: eventos.NuevoEvento(eventos.TipoSenaDeclarada, 1, "reserva-1", "corr", "",
				eventos.SenaDeclarada{ReservaID: 1}),
			estado: EstadoEnProceso, paso: "sena_declarada",
		},
		{
			evento: eventos.NuevoEvento(eventos.TipoSenaRegistrada, 1, "reserva-1", "corr", "",
				eventos.SenaRegistrada{ReservaID: 1, PagoID: 4}),
			estado: EstadoEnProceso, paso: "sena_registrada",
		},
		{
			evento: eventos.NuevoEvento(eventos.TipoReservaPasoAPagoPorVerificar, 1, "reserva-1", "corr", "",
				eventos.ReservaPasoAPagoPorVerificar{ReservaID: 1}),
			estado: EstadoEnProceso, paso: "pago_por_verificar",
		},
		{
			evento: eventos.NuevoEvento(eventos.TipoSenaAprobada, 1, "reserva-1", "corr", "",
				eventos.SenaAprobada{ReservaID: 1}),
			estado: EstadoEnProceso, paso: "sena_aprobada",
		},
		{
			evento: eventos.NuevoEvento(eventos.TipoSenaVerificada, 1, "reserva-1", "corr", "",
				eventos.SenaVerificada{ReservaID: 1, PagoID: 4}),
			estado: EstadoEnProceso, paso: "sena_verificada",
		},
		{
			evento: eventos.NuevoEvento(eventos.TipoReservaConfirmada, 1, "reserva-1", "corr", "",
				eventos.ReservaConfirmada{ReservaID: 1}),
			estado: EstadoEnProceso, paso: "reserva_confirmada",
		},
		{
			evento: eventos.NuevoEvento(eventos.TipoPartidoCreado, 1, "reserva-1", "corr", "",
				eventos.PartidoCreado{ReservaID: 1, PartidoID: 9}),
			estado: EstadoCompletada, paso: "partido_creado", recursoID: 9,
		},
	}

	for _, paso := range pasos {
		if err := proyeccion.aplicar(ctx, paso.evento); err != nil {
			t.Fatalf("%s: %v", paso.evento.Tipo, err)
		}
		saga = repo.sagas["reserva-1"]
		if saga.Estado != paso.estado || saga.Paso != paso.paso {
			t.Fatalf("%s: saga = %+v; se esperaba estado %q y paso %q",
				paso.evento.Tipo, saga, paso.estado, paso.paso)
		}
		if paso.recursoID != 0 && saga.RecursoID != paso.recursoID {
			t.Fatalf("%s: RecursoID = %d; se esperaba %d", paso.evento.Tipo, saga.RecursoID, paso.recursoID)
		}
		if saga.UltimoEventoID != paso.evento.ID {
			t.Fatalf("%s: UltimoEventoID = %q; se esperaba %q",
				paso.evento.Tipo, saga.UltimoEventoID, paso.evento.ID)
		}
	}
}

func TestProyeccionIgnoraEventosViejosReentregados(t *testing.T) {
	repo := nuevoRepoFalso()
	proyeccion := NuevaProyeccion(repo)
	ctx := context.Background()

	creada := eventos.NuevoEvento(eventos.TipoReservaCreada, 9, "reserva-9", "corr", "",
		eventos.ReservaCreada{ReservaID: 9})
	if err := proyeccion.aplicar(ctx, creada); err != nil {
		t.Fatalf("ReservaCreada: %v", err)
	}
	partido := eventos.NuevoEvento(
		eventos.TipoPartidoCreado, 9, "reserva-9", "corr", "",
		eventos.PartidoCreado{ReservaID: 9, PartidoID: 4})
	if err := proyeccion.aplicar(ctx, partido); err != nil {
		t.Fatalf("PartidoCreado: %v", err)
	}

	// El outbox es at-least-once: un evento viejo puede reaparecer después
	// de uno nuevo. No debe hacer retroceder la proyección.
	if err := proyeccion.aplicar(ctx, creada); err != nil {
		t.Fatalf("ReservaCreada reentregada: %v", err)
	}
	saga := repo.sagas["reserva-9"]
	if saga.Estado != EstadoCompletada || saga.Paso != "partido_creado" || saga.RecursoID != 4 {
		t.Fatalf("la proyección retrocedió: %+v", saga)
	}
}

func TestProyeccionCompensaSinPerderElEstadoCancelada(t *testing.T) {
	repo := nuevoRepoFalso()
	proyeccion := NuevaProyeccion(repo)
	ctx := context.Background()

	if err := proyeccion.aplicar(ctx, eventos.NuevoEvento(
		eventos.TipoReservaCreada, 2, "reserva-2", "corr", "",
		eventos.ReservaCreada{ReservaID: 2})); err != nil {
		t.Fatalf("ReservaCreada: %v", err)
	}

	if err := proyeccion.aplicar(ctx, eventos.NuevoEvento(
		eventos.TipoCancelacionSolicitada, 2, "reserva-2", "corr", "",
		eventos.CancelacionSolicitada{ReservaID: 2})); err != nil {
		t.Fatalf("CancelacionSolicitada: %v", err)
	}
	if saga := repo.sagas["reserva-2"]; saga.Estado != EstadoCompensando {
		t.Fatalf("se esperaba compensando y llegó %+v", saga)
	}

	if err := proyeccion.aplicar(ctx, eventos.NuevoEvento(
		eventos.TipoReservaCancelada, 2, "reserva-2", "corr", "",
		eventos.ReservaCancelada{ReservaID: 2})); err != nil {
		t.Fatalf("ReservaCancelada: %v", err)
	}
	if saga := repo.sagas["reserva-2"]; saga.Estado != EstadoCancelada {
		t.Fatalf("se esperaba cancelada y llegó %+v", saga)
	}

	if err := proyeccion.aplicar(ctx, eventos.NuevoEvento(
		eventos.TipoPagoAnulado, 2, "reserva-2", "corr", "",
		eventos.PagoAnulado{ReservaID: 2, PagoID: 1})); err != nil {
		t.Fatalf("PagoAnulado: %v", err)
	}
	saga := repo.sagas["reserva-2"]
	if saga.Estado != EstadoCancelada || saga.Paso != "pago_anulado" {
		t.Fatalf("la compensación no debe borrar la cancelación: %+v", saga)
	}
}

func TestProyeccionRegistraFallos(t *testing.T) {
	repo := nuevoRepoFalso()
	proyeccion := NuevaProyeccion(repo)
	ctx := context.Background()

	if err := proyeccion.aplicar(ctx, eventos.NuevoEvento(
		eventos.TipoReservaCreada, 3, "reserva-3", "corr", "",
		eventos.ReservaCreada{ReservaID: 3})); err != nil {
		t.Fatalf("ReservaCreada: %v", err)
	}
	if err := proyeccion.aplicar(ctx, eventos.NuevoEvento(
		eventos.TipoPartidoFallido, 3, "reserva-3", "corr", "",
		eventos.PartidoFallido{ReservaID: 3, Motivo: "reserva no confirmada"})); err != nil {
		t.Fatalf("PartidoFallido: %v", err)
	}
	saga := repo.sagas["reserva-3"]
	if saga.Estado != EstadoFallida || saga.Error != "reserva no confirmada" {
		t.Fatalf("no se registró el fallo: %+v", saga)
	}
}

func TestCoordinadorAsegurarEsIdempotente(t *testing.T) {
	repo := nuevoRepoFalso()
	coordinador := NuevoCoordinador(repo)
	ctx := context.Background()

	primera, err := coordinador.Asegurar(ctx, TipoCicloReserva, 8)
	if err != nil {
		t.Fatalf("Asegurar: %v", err)
	}
	segunda, err := coordinador.Asegurar(ctx, TipoCicloReserva, 8)
	if err != nil {
		t.Fatalf("Asegurar (segunda vez): %v", err)
	}
	if primera.ID != segunda.ID || primera.ID != "reserva-8" {
		t.Fatalf("ids inesperados: %q y %q", primera.ID, segunda.ID)
	}
	if len(repo.sagas) != 1 {
		t.Fatalf("se duplicó la saga: %+v", repo.sagas)
	}
}
