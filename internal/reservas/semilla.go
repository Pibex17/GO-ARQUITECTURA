package reservas

import "gorm.io/gorm"

// Sembrar carga datos de ejemplo solo si la tabla está vacía.
// canchaID y clienteID son los IDs ya sembrados por internal/canchas
// y internal/usuarios (en una base limpia: Cancha 1 y Usuario cliente 2).
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
			Pagos: []Pago{
				{Tipo: "sena", Monto: 7.50, Metodo: "transferencia", Estado: "verificado"},
			},
		},
	}
	db.Create(&datos)
}
