package canchas

import "gorm.io/gorm"

// Sembrar carga datos de ejemplo solo si la tabla está vacía.
// administradorID es el ID del usuario administrador ya sembrado
// por internal/usuarios (siempre el primero: ID 1 en una base limpia).
func Sembrar(db *gorm.DB, administradorID uint) {
	var total int64
	db.Model(&Cancha{}).Count(&total)
	if total > 0 {
		return
	}
	datos := []Cancha{
		{
			Nombre: "Cancha El Golazo 1", Tipo: "futbol_5",
			Direccion: "Av. Los Shyris y Naciones Unidas", PrecioHora: 25.00,
			Activa: true, AdministradorID: administradorID,
			Horarios: []Horario{
				{DiaSemana: "sabado", HoraInicio: "18:00", HoraFin: "19:00", Activo: true},
				{DiaSemana: "sabado", HoraInicio: "19:00", HoraFin: "20:00", Activo: true},
			},
		},
		{
			Nombre: "Cancha El Golazo 2", Tipo: "futbol_7",
			Direccion: "Av. Los Shyris y Naciones Unidas", PrecioHora: 35.00,
			Activa: true, AdministradorID: administradorID,
			Horarios: []Horario{
				{DiaSemana: "domingo", HoraInicio: "10:00", HoraFin: "11:00", Activo: true},
			},
		},
	}
	db.Create(&datos)
}
