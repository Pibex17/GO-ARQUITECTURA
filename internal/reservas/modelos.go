// Package reservas trae Reserva (la entidad con máquina de estados) y Pago.
package reservas

// Reserva es el turno que un cliente pide en una cancha.
type Reserva struct {
	ID         uint
	CanchaID   uint
	ClienteID  uint   // ID de un Usuario con rol cliente
	Fecha      string // "2026-10-21"
	HoraInicio string // "HH:MM"
	HoraFin    string // "HH:MM"
	Estado     string // pendiente | pago_por_verificar | confirmada | jugada | cancelada
	Pagos      []Pago
}

// Pago es un abono (seña o saldo) asociado a una Reserva.
type Pago struct {
	ID             uint
	ReservaID      uint
	Tipo           string  // sena | saldo
	Monto          float64 `gorm:"check:monto > 0"`
	Metodo         string  // transferencia | efectivo | tarjeta
	ComprobanteURL string
	Estado         string // pendiente | verificado | rechazado
}

// estadosValidos son los únicos valores que puede tomar Reserva.Estado.
var estadosValidos = map[string]bool{
	"pendiente":          true,
	"pago_por_verificar": true,
	"confirmada":         true,
	"jugada":             true,
	"cancelada":          true,
}

// transicionesValidas dice, para cada estado, a qué estados se puede pasar.
// Es la tabla de la Sección 5 de la ficha del negocio, ahora en código:
// de "jugada" no sale ninguna transición (estado terminal).
var transicionesValidas = map[string][]string{
	"pendiente":          {"pago_por_verificar", "cancelada"},
	"pago_por_verificar": {"confirmada", "cancelada"},
	"confirmada":         {"jugada", "cancelada"},
	"jugada":             {},
	"cancelada":          {},
}

// transicionPermitida informa si se puede pasar de "de" a "a".
func transicionPermitida(de, a string) bool {
	for _, siguiente := range transicionesValidas[de] {
		if siguiente == a {
			return true
		}
	}
	return false
}

// porcentajeSena es la regla de negocio de la ficha: la seña es el 30 %
// del precio por hora de la cancha.
const porcentajeSena = 0.30

// NuevoPagoSena construye el pago que se crea al pasar una reserva a
// "pago_por_verificar". La regla del 30 % vive aquí, no en el handler HTTP.
func NuevoPagoSena(reservaID uint, precioHora float64, metodo, comprobanteURL string) Pago {
	return Pago{
		ReservaID:      reservaID,
		Tipo:           "sena",
		Monto:          precioHora * porcentajeSena,
		Metodo:         metodo,
		ComprobanteURL: comprobanteURL,
		Estado:         "pendiente",
	}
}
