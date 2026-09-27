// Package canchas trae Cancha y Horario, y el CRUD del catálogo de canchas.
package canchas

// Cancha es una cancha sintética que un administrador ofrece para reservar.
type Cancha struct {
	ID              uint
	Nombre          string
	Tipo            string // futbol_5 | futbol_7 | futbol_11
	Direccion       string
	PrecioHora      float64
	Activa          bool
	AdministradorID uint // ID de un Usuario con rol administrador
	Horarios        []Horario
}

// Horario es un bloque semanal recurrente de disponibilidad de una Cancha.
type Horario struct {
	ID         uint
	CanchaID   uint
	DiaSemana  string // lunes | martes | miercoles | jueves | viernes | sabado | domingo
	HoraInicio string // "HH:MM"
	HoraFin    string // "HH:MM"
	Activo     bool
}

// tiposValidos son los únicos valores que puede tomar Tipo.
var tiposValidos = map[string]bool{
	"futbol_5":  true,
	"futbol_7":  true,
	"futbol_11": true,
}
