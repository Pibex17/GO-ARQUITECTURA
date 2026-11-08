// Package reservas trae Reserva (la entidad con máquina de estados).
// El pago vive en internal/pagos; aquí solo se conserva la relación de
// lectura para las consultas HTTP.
package reservas

import "canchas-sinteticas/internal/pagos"

// Reserva es el turno que un cliente pide en una cancha.
type Reserva struct {
	ID         uint
	CanchaID   uint
	ClienteID  uint   // ID de un Usuario con rol cliente
	Fecha      string // "2026-10-21"
	HoraInicio string // "HH:MM"
	HoraFin    string // "HH:MM"
	Estado     string // pendiente | pago_por_verificar | confirmada | jugada | cancelada
	Pagos      []pagos.Pago
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
// Es la tabla de la máquina de estados del negocio.
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
