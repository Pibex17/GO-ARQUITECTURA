package reservas

import "gorm.io/gorm"

// Sembrar carga datos de ejemplo solo si la tabla está vacía.
// canchaID y clienteID son los IDs ya sembrados por internal/canchas
// y internal/usuarios. La seña de la reserva confirmada la siembra
// internal/pagos.
func Sembrar(db *gorm.DB, canchaID, clienteID uint) {
	var total int64
	db.Model(&Reserva{}).Count(&total)
	if total > 0 {
		return
	}
	datos := []Reserva{
		{
			CanchaID: canchaID, ClienteID: clienteID,
			Fecha: "2026-10-03", HoraInicio: "18:00", HoraFin: "19:00",
			Estado: "pendiente",
		},
		{
			CanchaID: canchaID, ClienteID: clienteID,
			Fecha: "2026-10-04", HoraInicio: "19:00", HoraFin: "20:00",
			Estado: "confirmada",
		},
	}
	db.Create(&datos)
}
