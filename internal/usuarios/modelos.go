// Package usuarios trae la entidad Usuario. Todavía no tiene rutas
// propias (autenticación queda para más adelante en el hilo servidor);
// por ahora Cancha y Reserva solo guardan su ID y validan que exista
// antes de guardar.
package usuarios

// Usuario es un cliente o un administrador de canchas.
type Usuario struct {
	ID     uint
	Nombre string
	Email  string
	Rol    string // cliente | administrador
}

// rolesValidos son los únicos valores que puede tomar Rol.
var rolesValidos = map[string]bool{
	"cliente":       true,
	"administrador": true,
}

// RolValido informa si r es uno de los roles reconocidos.
func RolValido(r string) bool {
	return rolesValidos[r]
}
