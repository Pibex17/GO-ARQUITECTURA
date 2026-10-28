package reservas

import "testing"

// TestTransicionPermitida fija la tabla de la Sección 5 de la ficha.
// Esta regla vive en modelos.go, sin importar net/http ni la base de datos.
func TestTransicionPermitida(t *testing.T) {
	casos := []struct {
		de        string
		a         string
		permitida bool
	}{
		{"pendiente", "pago_por_verificar", true},
		{"pendiente", "cancelada", true},
		{"pendiente", "confirmada", false},
		{"pendiente", "jugada", false},

		{"pago_por_verificar", "confirmada", true},
		{"pago_por_verificar", "cancelada", true},
		{"pago_por_verificar", "pendiente", false},
		{"pago_por_verificar", "jugada", false},

		{"confirmada", "jugada", true},
		{"confirmada", "cancelada", true},
		{"confirmada", "pendiente", false},
		{"confirmada", "pago_por_verificar", false},

		// Estados terminales: no tienen salida.
		{"jugada", "cancelada", false},
		{"cancelada", "pendiente", false},

		// Un estado que no existe nunca se puede alcanzar.
		{"desconocido", "cancelada", false},
	}

	for _, caso := range casos {
		if obtenida := transicionPermitida(caso.de, caso.a); obtenida != caso.permitida {
			t.Errorf("transicionPermitida(%q, %q) = %v; se esperaba %v",
				caso.de, caso.a, obtenida, caso.permitida)
		}
	}
}

// TestCadaEstadoTieneFilaEnLaTabla evita que un estado válido quede sin
// definir y que una transición termine permitida por omisión.
func TestCadaEstadoTieneFilaEnLaTabla(t *testing.T) {
	for estado := range estadosValidos {
		if _, existe := transicionesValidas[estado]; !existe {
			t.Errorf("el estado %q no tiene fila en transicionesValidas", estado)
		}
	}
}
