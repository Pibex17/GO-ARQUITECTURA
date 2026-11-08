// Package pagos es el contexto dueño de la seña de una reserva.
// No conoce la máquina de estados de la reserva: solo reacciona a eventos.
package pagos

// Pago es un abono (seña o saldo) asociado a una Reserva.
type Pago struct {
	ID             uint
	ReservaID      uint
	Tipo           string // sena | saldo
	Monto          float64
	Metodo         string // transferencia | efectivo | tarjeta
	ComprobanteURL string
	Estado         string // pendiente | verificado | anulado
}

// TableName fija el nombre de la tabla de pagos.
func (Pago) TableName() string { return "pagos" }

// Estados válidos de un pago.
const (
	EstadoPendiente  = "pendiente"
	EstadoVerificado = "verificado"
	EstadoAnulado    = "anulado"
)

// porcentajeSena es la regla de negocio: la seña es el 30 % del precio
// por hora de la cancha.
const porcentajeSena = 0.30

// NuevoPagoSena construye el pago que crea el contexto de pagos al recibir
// el evento SenaDeclarada.
func NuevoPagoSena(reservaID uint, precioHora float64, metodo, comprobanteURL string) Pago {
	return Pago{
		ReservaID:      reservaID,
		Tipo:           "sena",
		Monto:          precioHora * porcentajeSena,
		Metodo:         metodo,
		ComprobanteURL: comprobanteURL,
		Estado:         EstadoPendiente,
	}
}
