package reservas

// FiltroReservas describe qué reservas puede ver una sesión. El servicio
// decide los campos de propiedad; el repositorio solo arma la consulta.
type FiltroReservas struct {
	Estado               string
	ClienteID            *uint // filtro adicional del administrador
	PropietarioClienteID *uint // reservas de este cliente (rol cliente)
	AdministradorID      *uint // reservas de las canchas de este administrador
}
