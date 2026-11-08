package pagos

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"canchas-sinteticas/internal/eventos"
)

type repoSagasFalso struct {
	sena        Pago
	errSena     error
	errCrear    error
	errVerifica error
	errAnula    error
	errRevierte error

	creado     *Pago
	verificado *Pago
	anulado    *Pago
	revertido  *Pago
}

func (r *repoSagasFalso) CrearEnTx(_ *gorm.DB, pago *Pago) error {
	if r.errCrear != nil {
		return r.errCrear
	}
	if pago.ID == 0 {
		pago.ID = 42
	}
	copia := *pago
	r.creado = &copia
	r.sena = copia
	return nil
}

func (r *repoSagasFalso) SenaPorReservaEnTx(_ *gorm.DB, _ uint) (Pago, error) {
	if r.errSena != nil {
		return Pago{}, r.errSena
	}
	if r.sena.ID == 0 {
		return Pago{}, gorm.ErrRecordNotFound
	}
	return r.sena, nil
}

func (r *repoSagasFalso) MarcarVerificadoEnTx(_ *gorm.DB, _ uint) (Pago, error) {
	if r.errVerifica != nil {
		return Pago{}, r.errVerifica
	}
	pago := r.sena
	pago.Estado = EstadoVerificado
	copia := pago
	r.verificado = &copia
	return pago, nil
}

func (r *repoSagasFalso) MarcarAnuladoEnTx(_ *gorm.DB, _ uint) (Pago, error) {
	if r.errAnula != nil {
		return Pago{}, r.errAnula
	}
	pago := r.sena
	if pago.ID == 0 {
		return Pago{}, gorm.ErrRecordNotFound
	}
	pago.Estado = EstadoAnulado
	copia := pago
	r.anulado = &copia
	return pago, nil
}

func (r *repoSagasFalso) RevertirVerificacionEnTx(_ *gorm.DB, _ uint) (Pago, error) {
	if r.errRevierte != nil {
		return Pago{}, r.errRevierte
	}
	pago := r.sena
	if pago.ID == 0 {
		return Pago{}, gorm.ErrRecordNotFound
	}
	pago.Estado = EstadoPendiente
	copia := pago
	r.revertido = &copia
	return pago, nil
}

func (r *repoSagasFalso) ListarPorReserva(uint) ([]Pago, error) { return nil, nil }

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

type publicadorFalso struct {
	publicados []eventos.Evento
}

func (p *publicadorFalso) Publicar(_ context.Context, evento eventos.Evento) error {
	p.publicados = append(p.publicados, evento)
	return nil
}

func TestOnSenaDeclaradaCreaLaSena(t *testing.T) {
	repo := &repoSagasFalso{}
	procesador := &procesadorFalso{}
	sagas := NuevoSagas(repo, procesador, &publicadorFalso{})

	evento := eventos.NuevoEvento(eventos.TipoSenaDeclarada, 1, "reserva-1", "corr", "",
		eventos.SenaDeclarada{ReservaID: 1, PrecioHora: 25, Metodo: "efectivo"})
	if err := sagas.onSenaDeclarada(context.Background(), evento); err != nil {
		t.Fatalf("onSenaDeclarada: %v", err)
	}
	if repo.creado == nil || repo.creado.Monto != 7.5 || repo.creado.Estado != EstadoPendiente {
		t.Fatalf("la seña no se creó bien: %+v", repo.creado)
	}
	if len(procesador.publicados) != 1 || procesador.publicados[0].Tipo != eventos.TipoSenaRegistrada {
		t.Fatalf("no se publicó SenaRegistrada: %+v", procesador.publicados)
	}
}

func TestOnSenaDeclaradaReutilizaLaSenaExistente(t *testing.T) {
	repo := &repoSagasFalso{sena: Pago{ID: 9, ReservaID: 1, Monto: 7.5, Estado: EstadoPendiente}}
	procesador := &procesadorFalso{}
	sagas := NuevoSagas(repo, procesador, &publicadorFalso{})

	evento := eventos.NuevoEvento(eventos.TipoSenaDeclarada, 1, "reserva-1", "corr", "",
		eventos.SenaDeclarada{ReservaID: 1, PrecioHora: 25})
	if err := sagas.onSenaDeclarada(context.Background(), evento); err != nil {
		t.Fatalf("onSenaDeclarada: %v", err)
	}
	if repo.creado != nil {
		t.Fatal("no debía crear otra seña")
	}
	datos, err := eventos.Decodificar[eventos.SenaRegistrada](procesador.publicados[0])
	if err != nil || datos.PagoID != 9 {
		t.Fatalf("payload inesperado: %+v (%v)", datos, err)
	}
}

func TestOnSenaDeclaradaCompensaSiNoPuedeCrear(t *testing.T) {
	repo := &repoSagasFalso{errCrear: errors.New("base caída")}
	publicador := &publicadorFalso{}
	sagas := NuevoSagas(repo, &procesadorFalso{}, publicador)

	evento := eventos.NuevoEvento(eventos.TipoSenaDeclarada, 1, "reserva-1", "corr", "",
		eventos.SenaDeclarada{ReservaID: 1, PrecioHora: 25})
	if err := sagas.onSenaDeclarada(context.Background(), evento); err != nil {
		t.Fatalf("onSenaDeclarada: %v", err)
	}
	if len(publicador.publicados) != 1 || publicador.publicados[0].Tipo != eventos.TipoRegistroSenaFallido {
		t.Fatalf("no se publicó la compensación: %+v", publicador.publicados)
	}
	if publicador.publicados[0].ID != evento.ID+"-"+eventos.TipoRegistroSenaFallido {
		t.Fatalf("el id de compensación no es determinista: %q", publicador.publicados[0].ID)
	}
}

func TestOnSenaAprobadaVerificaLaSena(t *testing.T) {
	repo := &repoSagasFalso{sena: Pago{ID: 9, ReservaID: 1, Monto: 7.5, Estado: EstadoPendiente}}
	procesador := &procesadorFalso{}
	sagas := NuevoSagas(repo, procesador, &publicadorFalso{})

	evento := eventos.NuevoEvento(eventos.TipoSenaAprobada, 1, "reserva-1", "corr", "",
		eventos.SenaAprobada{ReservaID: 1})
	if err := sagas.onSenaAprobada(context.Background(), evento); err != nil {
		t.Fatalf("onSenaAprobada: %v", err)
	}
	if repo.verificado == nil || repo.verificado.Estado != EstadoVerificado {
		t.Fatalf("la seña no se verificó: %+v", repo.verificado)
	}
	if len(procesador.publicados) != 1 || procesador.publicados[0].Tipo != eventos.TipoSenaVerificada {
		t.Fatalf("no se publicó SenaVerificada: %+v", procesador.publicados)
	}
}

func TestOnReservaCanceladaAnulaLaSena(t *testing.T) {
	repo := &repoSagasFalso{sena: Pago{ID: 9, ReservaID: 1, Monto: 7.5, Estado: EstadoVerificado}}
	procesador := &procesadorFalso{}
	sagas := NuevoSagas(repo, procesador, &publicadorFalso{})

	evento := eventos.NuevoEvento(eventos.TipoReservaCancelada, 1, "reserva-1", "corr", "",
		eventos.ReservaCancelada{ReservaID: 1})
	if err := sagas.onReservaCancelada(context.Background(), evento); err != nil {
		t.Fatalf("onReservaCancelada: %v", err)
	}
	if repo.anulado == nil || repo.anulado.Estado != EstadoAnulado {
		t.Fatalf("la seña no se anuló: %+v", repo.anulado)
	}
	if len(procesador.publicados) != 1 || procesador.publicados[0].Tipo != eventos.TipoPagoAnulado {
		t.Fatalf("no se publicó PagoAnulado: %+v", procesador.publicados)
	}
}

func TestOnReservaNoConfirmadaRevierteLaVerificacion(t *testing.T) {
	repo := &repoSagasFalso{sena: Pago{ID: 9, ReservaID: 1, Monto: 7.5, Estado: EstadoVerificado}}
	procesador := &procesadorFalso{}
	sagas := NuevoSagas(repo, procesador, &publicadorFalso{})

	evento := eventos.NuevoEvento(eventos.TipoReservaNoConfirmada, 1, "reserva-1", "corr", "",
		eventos.ReservaNoConfirmada{ReservaID: 1, Motivo: "no se pudo confirmar"})
	if err := sagas.onReservaNoConfirmada(context.Background(), evento); err != nil {
		t.Fatalf("onReservaNoConfirmada: %v", err)
	}
	if repo.revertido == nil || repo.revertido.Estado != EstadoPendiente {
		t.Fatalf("la seña no volvió a pendiente: %+v", repo.revertido)
	}
	if len(procesador.publicados) != 1 || procesador.publicados[0].Tipo != eventos.TipoVerificacionRevertida {
		t.Fatalf("no se publicó VerificacionRevertida: %+v", procesador.publicados)
	}
}
