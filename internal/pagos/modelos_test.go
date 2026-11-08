package pagos

import (
	"math"
	"testing"
)

// TestNuevoPagoSenaFijaLaReglaDelTreintaPorCiento comprueba que la seña
// se calcula en el contexto de pagos, sin depender del handler HTTP.
func TestNuevoPagoSenaFijaLaReglaDelTreintaPorCiento(t *testing.T) {
	pago := NuevoPagoSena(3, 25, "efectivo", "https://ejemplo.invalid/comprobante.jpg")

	if pago.ReservaID != 3 {
		t.Errorf("ReservaID = %d; se esperaba 3", pago.ReservaID)
	}
	if pago.Tipo != "sena" {
		t.Errorf("Tipo = %q; se esperaba sena", pago.Tipo)
	}
	if math.Abs(pago.Monto-7.5) > 1e-9 {
		t.Errorf("Monto = %v; se esperaba 7.5 (30 %% de 25)", pago.Monto)
	}
	if pago.Metodo != "efectivo" || pago.ComprobanteURL == "" {
		t.Errorf("no se copiaron los datos del pago: %+v", pago)
	}
	if pago.Estado != EstadoPendiente {
		t.Errorf("Estado = %q; se esperaba pendiente", pago.Estado)
	}
}
