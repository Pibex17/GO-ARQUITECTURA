package usuarios

import "gorm.io/gorm"

// Sembrar carga datos de ejemplo solo si la tabla está vacía.
func Sembrar(db *gorm.DB) {
	var total int64
	db.Model(&Usuario{}).Count(&total)
	if total > 0 {
		return
	}
	datos := []Usuario{
		{Nombre: "El Golazo (administrador)", Email: "admin@elgolazo.ec", Rol: "administrador"},
		{Nombre: "María Pérez", Email: "maria@gmail.com", Rol: "cliente"},
		{Nombre: "Juan Torres", Email: "juan@gmail.com", Rol: "cliente"},
	}
	db.Create(&datos)
}
