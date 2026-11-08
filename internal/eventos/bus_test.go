package eventos

import (
	"context"
	"errors"
	"testing"
)

func TestBusEntregaAlTipoYAlGlobal(t *testing.T) {
	bus := NuevoBusEnMemoria()
	var recibidos []string

	bus.Suscribir(TipoReservaCreada, func(_ context.Context, e Evento) error {
		recibidos = append(recibidos, "tipo:"+e.Tipo)
		return nil
	})
	bus.Suscribir("*", func(_ context.Context, e Evento) error {
		recibidos = append(recibidos, "global:"+e.Tipo)
		return nil
	})

	evento := NuevoEvento(TipoReservaCreada, 1, "reserva-1", "corr", "", ReservaCreada{ReservaID: 1})
	if err := bus.Publicar(context.Background(), evento); err != nil {
		t.Fatalf("Publicar devolvió error: %v", err)
	}

	if len(recibidos) != 2 {
		t.Fatalf("se esperaban 2 entregas y llegaron %v", recibidos)
	}
	if recibidos[0] != "tipo:"+TipoReservaCreada || recibidos[1] != "global:"+TipoReservaCreada {
		t.Fatalf("orden de entregas inesperado: %v", recibidos)
	}
}

func TestBusAgrupaErroresDeConsumidores(t *testing.T) {
	bus := NuevoBusEnMemoria()
	bus.Suscribir(TipoReservaCreada, func(context.Context, Evento) error {
		return errors.New("falló el primero")
	})
	bus.Suscribir(TipoReservaCreada, func(context.Context, Evento) error {
		return errors.New("falló el segundo")
	})

	err := bus.Publicar(context.Background(), NuevoEvento(TipoReservaCreada, 1, "", "", "", ReservaCreada{}))
	if err == nil {
		t.Fatal("se esperaba un error agregado")
	}
	if texto := err.Error(); texto != "falló el primero\nfalló el segundo" {
		t.Fatalf("texto de error inesperado: %q", texto)
	}
}

func TestBusRecuperaPanicos(t *testing.T) {
	bus := NuevoBusEnMemoria()
	bus.Suscribir(TipoReservaCreada, func(context.Context, Evento) error {
		panic("explotó")
	})

	err := bus.Publicar(context.Background(), NuevoEvento(TipoReservaCreada, 1, "", "", "", ReservaCreada{}))
	if err == nil {
		t.Fatal("un pánico debe convertirse en error")
	}
}

func TestDerivadoConservaSagaYCorrelacion(t *testing.T) {
	causa := NuevoEvento(TipoSenaDeclarada, 7, "reserva-7", "corr-7", "", SenaDeclarada{ReservaID: 7})
	derivado := Derivado(causa, TipoSenaRegistrada, 7, SenaRegistrada{ReservaID: 7, PagoID: 3})

	if derivado.SagaID != causa.SagaID || derivado.CorrelationID != causa.CorrelationID {
		t.Fatalf("no se conservó la trazabilidad: %+v", derivado)
	}
	if derivado.CausaID != causa.ID {
		t.Fatalf("CausaID = %q; se esperaba %q", derivado.CausaID, causa.ID)
	}
	if derivado.ID == causa.ID {
		t.Fatal("el evento derivado debe tener id propio")
	}
}

func TestDerivadoConIDEsDeterminista(t *testing.T) {
	causa := NuevoEvento(TipoSenaRegistrada, 7, "reserva-7", "corr-7", "", SenaRegistrada{ReservaID: 7})
	primero := DerivadoConID("comp-1", causa, TipoReservaNoActualizada, 7, ReservaNoActualizada{ReservaID: 7})
	segundo := DerivadoConID("comp-1", causa, TipoReservaNoActualizada, 7, ReservaNoActualizada{ReservaID: 7})
	if primero.ID != segundo.ID || primero.ID != "comp-1" {
		t.Fatalf("los ids deterministas no coinciden: %q y %q", primero.ID, segundo.ID)
	}
}
