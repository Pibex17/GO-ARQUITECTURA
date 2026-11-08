package partidos

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"canchas-sinteticas/internal/eventos"
	"canchas-sinteticas/internal/reservas"
)

type repoSagasFalso struct {
	partido    Partido
	errPartido error
	errCrear   error
	errGuardar error

	creado   *Partido
	guardado *Partido
}

func (r *repoSagasFalso) ReservaPorID(uint) (reservas.Reserva, error) {
	return reservas.Reserva{}, nil
}
func (r *repoSagasFalso) PartidoPorReservaID(uint) (Partido, error) { return r.partido, r.errPartido }
func (r *repoSagasFalso) PartidoPorID(uint) (Partido, error)        { return r.partido, nil }
func (r *repoSagasFalso) ContarInscripciones(uint) (int64, error)   { return 0, nil }

func (r *repoSagasFalso) CrearPartidoEnTx(_ *gorm.DB, partido *Partido) error {
	if r.errCrear != nil {
		return r.errCrear
	}
	if partido.ID == 0 {
		partido.ID = 77
	}
	copia := *partido
	r.creado = &copia
	return nil
}

func (r *repoSagasFalso) PartidoPorReservaIDEnTx(_ *gorm.DB, _ uint) (Partido, error) {
	if r.errPartido != nil {
		return Partido{}, r.errPartido
	}
	if r.partido.ID == 0 {
		return Partido{}, gorm.ErrRecordNotFound
	}
	return r.partido, nil
}

func (r *repoSagasFalso) GuardarPartidoEnTx(_ *gorm.DB, partido *Partido) error {
	if r.errGuardar != nil {
		return r.errGuardar
	}
	copia := *partido
	r.guardado = &copia
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

func TestOnReservaValidadaCreaElPartido(t *testing.T) {
	repo := &repoSagasFalso{}
	procesador := &procesadorFalso{}
	sagas := NuevoSagas(repo, procesador, &publicadorSagasFalso{})

	evento := eventos.NuevoEvento(eventos.TipoReservaValidadaParaPartido, 1, "reserva-1", "corr", "",
		eventos.ReservaValidadaParaPartido{
			ReservaID: 1, ClienteID: 2, Nombre: "Amistoso", CupoMaximo: 10,
		})
	if err := sagas.onReservaValidada(context.Background(), evento); err != nil {
		t.Fatalf("onReservaValidada: %v", err)
	}
	if repo.creado == nil || repo.creado.ReservaID != 1 || repo.creado.Estado != EstadoActivo {
		t.Fatalf("el partido no se creó bien: %+v", repo.creado)
	}
	if repo.creado.CodigoInvitacion == "" {
		t.Fatal("el partido no recibió código de invitación")
	}
	if len(procesador.publicados) != 1 || procesador.publicados[0].Tipo != eventos.TipoPartidoCreado {
		t.Fatalf("no se publicó PartidoCreado: %+v", procesador.publicados)
	}
	datos, err := eventos.Decodificar[eventos.PartidoCreado](procesador.publicados[0])
	if err != nil || datos.PartidoID != 77 {
		t.Fatalf("payload inesperado: %+v (%v)", datos, err)
	}
}

func TestOnReservaValidadaEsIdempotente(t *testing.T) {
	repo := &repoSagasFalso{partido: Partido{ID: 33, ReservaID: 1, Estado: EstadoActivo}}
	procesador := &procesadorFalso{}
	sagas := NuevoSagas(repo, procesador, &publicadorSagasFalso{})

	evento := eventos.NuevoEvento(eventos.TipoReservaValidadaParaPartido, 1, "reserva-1", "corr", "",
		eventos.ReservaValidadaParaPartido{ReservaID: 1, Nombre: "Amistoso", CupoMaximo: 10})
	if err := sagas.onReservaValidada(context.Background(), evento); err != nil {
		t.Fatalf("onReservaValidada: %v", err)
	}
	if repo.creado != nil {
		t.Fatal("no debía crear otro partido")
	}
	datos, err := eventos.Decodificar[eventos.PartidoCreado](procesador.publicados[0])
	if err != nil || datos.PartidoID != 33 {
		t.Fatalf("payload inesperado: %+v (%v)", datos, err)
	}
}

func TestOnReservaCanceladaCancelaElPartido(t *testing.T) {
	repo := &repoSagasFalso{partido: Partido{ID: 33, ReservaID: 1, Estado: EstadoActivo}}
	procesador := &procesadorFalso{}
	sagas := NuevoSagas(repo, procesador, &publicadorSagasFalso{})

	evento := eventos.NuevoEvento(eventos.TipoReservaCancelada, 1, "reserva-1", "corr", "",
		eventos.ReservaCancelada{ReservaID: 1})
	if err := sagas.onReservaCancelada(context.Background(), evento); err != nil {
		t.Fatalf("onReservaCancelada: %v", err)
	}
	if repo.guardado == nil || repo.guardado.Estado != EstadoCancelado {
		t.Fatalf("el partido no se canceló: %+v", repo.guardado)
	}
	if len(procesador.publicados) != 1 || procesador.publicados[0].Tipo != eventos.TipoPartidoCancelado {
		t.Fatalf("no se publicó PartidoCancelado: %+v", procesador.publicados)
	}
}

func TestOnReservaCanceladaSinPartidoNoHaceNada(t *testing.T) {
	repo := &repoSagasFalso{}
	procesador := &procesadorFalso{}
	sagas := NuevoSagas(repo, procesador, &publicadorSagasFalso{})

	evento := eventos.NuevoEvento(eventos.TipoReservaCancelada, 1, "reserva-1", "corr", "",
		eventos.ReservaCancelada{ReservaID: 1})
	if err := sagas.onReservaCancelada(context.Background(), evento); err != nil {
		t.Fatalf("onReservaCancelada: %v", err)
	}
	if repo.guardado != nil || len(procesador.publicados) != 0 {
		t.Fatalf("no debía haber cambios: %+v / %+v", repo.guardado, procesador.publicados)
	}
}
