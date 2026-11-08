package reservas

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"canchas-sinteticas/internal/canchas"
	"canchas-sinteticas/internal/eventos"
)

type repoSagasFalso struct {
	reserva    Reserva
	errReserva error
	errGuardar error
	guardada   *Reserva
}

func (r *repoSagasFalso) CanchaPorID(uint) (canchas.Cancha, error) { return canchas.Cancha{}, nil }
func (r *repoSagasFalso) HayConflicto(uint, string, string) (bool, error) {
	return false, nil
}
func (r *repoSagasFalso) CrearReservaConEvento(*Reserva, ConstructorEvento) error { return nil }
func (r *repoSagasFalso) ListarReservas(FiltroReservas) ([]Reserva, error)        { return nil, nil }
func (r *repoSagasFalso) ReservaPorID(uint) (Reserva, error)                      { return r.reserva, r.errReserva }

func (r *repoSagasFalso) ReservaPorIDEnTx(_ *gorm.DB, _ uint) (Reserva, error) {
	return r.reserva, r.errReserva
}

func (r *repoSagasFalso) GuardarReservaEnTx(_ *gorm.DB, reserva *Reserva) error {
	if r.errGuardar != nil {
		return r.errGuardar
	}
	copia := *reserva
	r.guardada = &copia
	r.reserva = copia
	return nil
}

type procesadorFalso struct {
	publicados []eventos.Evento
	err        error
}

func (p *procesadorFalso) Procesar(_ context.Context, _ eventos.Evento, fn eventos.FuncionProceso) error {
	if p.err != nil {
		return p.err
	}
	nuevos, err := fn(nil)
	if err != nil {
		return err
	}
	p.publicados = append(p.publicados, nuevos...)
	return nil
}

type publicadorSagasFalso struct {
	publicados []eventos.Evento
}

func (p *publicadorSagasFalso) Publicar(_ context.Context, evento eventos.Evento) error {
	p.publicados = append(p.publicados, evento)
	return nil
}

func TestOnSenaRegistradaMueveLaReserva(t *testing.T) {
	repo := &repoSagasFalso{reserva: Reserva{ID: 1, Estado: "pendiente"}}
	procesador := &procesadorFalso{}
	sagas := NuevoSagas(repo, procesador, &publicadorSagasFalso{})

	evento := eventos.NuevoEvento(eventos.TipoSenaRegistrada, 1, "reserva-1", "corr", "",
		eventos.SenaRegistrada{ReservaID: 1, PagoID: 3, Monto: 7.5})
	if err := sagas.onSenaRegistrada(context.Background(), evento); err != nil {
		t.Fatalf("onSenaRegistrada: %v", err)
	}
	if repo.guardada == nil || repo.guardada.Estado != "pago_por_verificar" {
		t.Fatalf("la reserva no cambió: %+v", repo.guardada)
	}
	if len(procesador.publicados) != 1 || procesador.publicados[0].Tipo != eventos.TipoReservaPasoAPagoPorVerificar {
		t.Fatalf("no se publicó el paso esperado: %+v", procesador.publicados)
	}
}

func TestOnSenaRegistradaCompensaSiNoPuedeGuardar(t *testing.T) {
	repo := &repoSagasFalso{
		reserva:    Reserva{ID: 1, Estado: "pendiente"},
		errGuardar: errors.New("base caída"),
	}
	publicador := &publicadorSagasFalso{}
	sagas := NuevoSagas(repo, &procesadorFalso{}, publicador)

	evento := eventos.NuevoEvento(eventos.TipoSenaRegistrada, 1, "reserva-1", "corr", "",
		eventos.SenaRegistrada{ReservaID: 1, PagoID: 3, Monto: 7.5})
	if err := sagas.onSenaRegistrada(context.Background(), evento); err != nil {
		t.Fatalf("onSenaRegistrada: %v", err)
	}
	if len(publicador.publicados) != 1 || publicador.publicados[0].Tipo != eventos.TipoReservaNoActualizada {
		t.Fatalf("no se publicó la compensación: %+v", publicador.publicados)
	}
	if publicador.publicados[0].ID != evento.ID+"-"+eventos.TipoReservaNoActualizada {
		t.Fatalf("el id de compensación no es determinista: %q", publicador.publicados[0].ID)
	}
}

func TestOnSenaVerificadaConfirmaLaReserva(t *testing.T) {
	repo := &repoSagasFalso{reserva: Reserva{ID: 1, Estado: "pago_por_verificar"}}
	procesador := &procesadorFalso{}
	sagas := NuevoSagas(repo, procesador, &publicadorSagasFalso{})

	evento := eventos.NuevoEvento(eventos.TipoSenaVerificada, 1, "reserva-1", "corr", "",
		eventos.SenaVerificada{ReservaID: 1, PagoID: 3})
	if err := sagas.onSenaVerificada(context.Background(), evento); err != nil {
		t.Fatalf("onSenaVerificada: %v", err)
	}
	if repo.guardada == nil || repo.guardada.Estado != "confirmada" {
		t.Fatalf("la reserva no se confirmó: %+v", repo.guardada)
	}
	if len(procesador.publicados) != 1 || procesador.publicados[0].Tipo != eventos.TipoReservaConfirmada {
		t.Fatalf("no se publicó la confirmación: %+v", procesador.publicados)
	}
}

func TestOnCancelacionSolicitadaCancelaLaReserva(t *testing.T) {
	repo := &repoSagasFalso{reserva: Reserva{ID: 1, Estado: "confirmada"}}
	procesador := &procesadorFalso{}
	sagas := NuevoSagas(repo, procesador, &publicadorSagasFalso{})

	evento := eventos.NuevoEvento(eventos.TipoCancelacionSolicitada, 1, "reserva-1", "corr", "",
		eventos.CancelacionSolicitada{ReservaID: 1})
	if err := sagas.onCancelacionSolicitada(context.Background(), evento); err != nil {
		t.Fatalf("onCancelacionSolicitada: %v", err)
	}
	if repo.guardada == nil || repo.guardada.Estado != "cancelada" {
		t.Fatalf("la reserva no se canceló: %+v", repo.guardada)
	}
	if len(procesador.publicados) != 1 || procesador.publicados[0].Tipo != eventos.TipoReservaCancelada {
		t.Fatalf("no se publicó la cancelación: %+v", procesador.publicados)
	}
}

func TestOnJugadaSolicitadaCierraLaReserva(t *testing.T) {
	repo := &repoSagasFalso{reserva: Reserva{ID: 1, Estado: "confirmada"}}
	procesador := &procesadorFalso{}
	sagas := NuevoSagas(repo, procesador, &publicadorSagasFalso{})

	evento := eventos.NuevoEvento(eventos.TipoJugadaSolicitada, 1, "reserva-1", "corr", "",
		eventos.JugadaSolicitada{ReservaID: 1})
	if err := sagas.onJugadaSolicitada(context.Background(), evento); err != nil {
		t.Fatalf("onJugadaSolicitada: %v", err)
	}
	if repo.guardada == nil || repo.guardada.Estado != "jugada" {
		t.Fatalf("la reserva no quedó jugada: %+v", repo.guardada)
	}
	if len(procesador.publicados) != 1 || procesador.publicados[0].Tipo != eventos.TipoReservaJugada {
		t.Fatalf("no se publicó la jugada: %+v", procesador.publicados)
	}
}

func TestOnSolicitudPartidoRechazaReservaNoConfirmada(t *testing.T) {
	repo := &repoSagasFalso{reserva: Reserva{ID: 1, ClienteID: 2, Estado: "pendiente"}}
	procesador := &procesadorFalso{}
	sagas := NuevoSagas(repo, procesador, &publicadorSagasFalso{})

	evento := eventos.NuevoEvento(eventos.TipoSolicitudPartidoRecibida, 1, "reserva-1", "corr", "",
		eventos.SolicitudPartidoRecibida{ReservaID: 1, ClienteID: 2, Nombre: "Amistoso", CupoMaximo: 10})
	if err := sagas.onSolicitudPartido(context.Background(), evento); err != nil {
		t.Fatalf("onSolicitudPartido: %v", err)
	}
	if len(procesador.publicados) != 1 || procesador.publicados[0].Tipo != eventos.TipoPartidoFallido {
		t.Fatalf("no se rechazó la solicitud: %+v", procesador.publicados)
	}
}

func TestOnSolicitudPartidoValidaReservaConfirmada(t *testing.T) {
	repo := &repoSagasFalso{reserva: Reserva{ID: 1, ClienteID: 2, Estado: "confirmada"}}
	procesador := &procesadorFalso{}
	sagas := NuevoSagas(repo, procesador, &publicadorSagasFalso{})

	evento := eventos.NuevoEvento(eventos.TipoSolicitudPartidoRecibida, 1, "reserva-1", "corr", "",
		eventos.SolicitudPartidoRecibida{ReservaID: 1, ClienteID: 2, Nombre: "Amistoso", CupoMaximo: 10})
	if err := sagas.onSolicitudPartido(context.Background(), evento); err != nil {
		t.Fatalf("onSolicitudPartido: %v", err)
	}
	if len(procesador.publicados) != 1 || procesador.publicados[0].Tipo != eventos.TipoReservaValidadaParaPartido {
		t.Fatalf("no se validó la reserva: %+v", procesador.publicados)
	}
	datos, err := eventos.Decodificar[eventos.ReservaValidadaParaPartido](procesador.publicados[0])
	if err != nil || datos.ReservaID != 1 || datos.Nombre != "Amistoso" {
		t.Fatalf("payload inesperado: %+v (%v)", datos, err)
	}
}
