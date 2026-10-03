package inscripcion

import "gorm.io/gorm"

// Sembrar carga datos de ejemplo solo si la tabla está vacía.
// partidoID es el partido sembrado por internal/partidos (siempre el primero: ID 1).
func Sembrar(db *gorm.DB, partidoID uint) {
	var total int64
	db.Model(&Inscripcion{}).Count(&total)
	if total > 0 {
		return
	}
	datos := []Inscripcion{
		{PartidoID: partidoID, JugadorID: 2}, // María Pérez
		{PartidoID: partidoID, JugadorID: 3}, // Juan Torres
	}
	db.Create(&datos)
}
