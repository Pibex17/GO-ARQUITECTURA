package partidos

import (
	"time"

	"gorm.io/gorm"
)

// Sembrar carga datos de ejemplo solo si la tabla está vacía.
// reservaID es la reserva confirmada (en una base limpia: Reserva 2).
func Sembrar(db *gorm.DB, reservaID uint) {
	var total int64
	db.Model(&Partido{}).Count(&total)
	if total > 0 {
		return
	}
	datos := []Partido{
		{
			ReservaID:        reservaID,
			Nombre:           "Picadito del sábado",
			CupoMaximo:       10,
			CodigoInvitacion: "ABC123",
			CreadoEn:         time.Now().UTC(),
			Estado:           EstadoActivo,
		},
	}
	db.Create(&datos)
}
