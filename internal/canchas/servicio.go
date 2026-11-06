package canchas

import (
	"errors"
	"fmt"
)

// Errores de negocio de canchas. El transporte los traduce a códigos HTTP;
// el servicio no conoce net/http ni GORM.
var (
	ErrNoEncontrada   = errors.New("no existe una cancha con ese id")
	ErrFaltanDatos    = errors.New("nombre y dirección son obligatorios")
	ErrPrecioInvalido = errors.New("el precio por hora debe ser mayor que cero")
	ErrTipoInvalido   = errors.New("el tipo de cancha no es válido")
	ErrNoEsSuya       = errors.New("la cancha no es suya")
	ErrRolNoPermitido = errors.New("su rol no tiene permiso para esta acción")

	errListando = errors.New("no se pudo listar las canchas")
	errLeyendo  = errors.New("no se pudo leer la cancha")
	errCreando  = errors.New("no se pudo guardar la cancha")
	errEditando = errors.New("no se pudo actualizar la cancha")
)

// Repositorio es la persistencia que necesita el servicio. La interfaz vive
// en la capa de servicio; la implementación GORM está en repositorio.go.
type Repositorio interface {
	CanchaPorID(id uint) (Cancha, error)
	ListarCanchas(filtro FiltroCanchas) ([]Cancha, error)
	CrearCancha(cancha *Cancha) error
	GuardarCancha(cancha *Cancha) error
}

// EntradaCrear son los datos de negocio para crear una cancha. El dueño no
// viene del cuerpo: lo aporta el transporte desde la sesión autenticada.
type EntradaCrear struct {
	Nombre     string
	Tipo       string
	Direccion  string
	PrecioHora float64
}

// EntradaActualizar son los campos modificables de una cancha. El puntero
// Activa distingue "no lo mandaron" de "lo mandaron en false".
type EntradaActualizar struct {
	Nombre     string
	Tipo       string
	Direccion  string
	PrecioHora float64
	Activa     *bool
}

// Servicio contiene los casos de uso del catálogo de canchas.
type Servicio struct {
	repo Repositorio
}

// NuevoServicio inyecta la persistencia que necesita el caso de uso.
func NuevoServicio(repo Repositorio) *Servicio {
	return &Servicio{repo: repo}
}

// Crear valida y construye una cancha activa para el administrador indicado.
func (s *Servicio) Crear(administradorID uint, entrada EntradaCrear) (Cancha, error) {
	if entrada.Nombre == "" || entrada.Direccion == "" {
		return Cancha{}, ErrFaltanDatos
	}
	if entrada.PrecioHora <= 0 {
		return Cancha{}, ErrPrecioInvalido
	}
	if !tiposValidos[entrada.Tipo] {
		return Cancha{}, ErrTipoInvalido
	}

	cancha := Cancha{
		Nombre:          entrada.Nombre,
		Tipo:            entrada.Tipo,
		Direccion:       entrada.Direccion,
		PrecioHora:      entrada.PrecioHora,
		Activa:          true,
		AdministradorID: administradorID,
	}
	if err := s.repo.CrearCancha(&cancha); err != nil {
		return Cancha{}, fmt.Errorf("%w: %v", errCreando, err)
	}
	return cancha, nil
}

// Listar aplica la visibilidad del catálogo según el rol. El cliente solo
// ve activas y puede filtrar por administrador; el administrador solo ve
// las suyas, activas e inactivas, sin ampliar el alcance por query.
func (s *Servicio) Listar(rol string, usuarioID uint, administradorID *uint) ([]Cancha, error) {
	filtro := FiltroCanchas{}
	switch rol {
	case "cliente":
		activa := true
		filtro.Activa = &activa
		filtro.AdministradorID = administradorID
	case "administrador":
		propio := usuarioID
		filtro.AdministradorID = &propio
	default:
		return nil, ErrRolNoPermitido
	}

	lista, err := s.repo.ListarCanchas(filtro)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errListando, err)
	}
	return lista, nil
}

// Actualizar aplica los cambios permitidos sobre una cancha del administrador.
func (s *Servicio) Actualizar(id, administradorID uint, entrada EntradaActualizar) (Cancha, error) {
	existente, err := s.repo.CanchaPorID(id)
	if err != nil {
		if errors.Is(err, ErrNoEncontrada) {
			return Cancha{}, ErrNoEncontrada
		}
		return Cancha{}, fmt.Errorf("%w: %v", errLeyendo, err)
	}
	if existente.AdministradorID != administradorID {
		return Cancha{}, ErrNoEsSuya
	}
	if entrada.Tipo != "" && !tiposValidos[entrada.Tipo] {
		return Cancha{}, ErrTipoInvalido
	}

	// Solo estos campos se pueden cambiar: el AdministradorID no se toca
	// desde este endpoint.
	if entrada.Nombre != "" {
		existente.Nombre = entrada.Nombre
	}
	if entrada.Tipo != "" {
		existente.Tipo = entrada.Tipo
	}
	if entrada.Direccion != "" {
		existente.Direccion = entrada.Direccion
	}
	if entrada.PrecioHora > 0 {
		existente.PrecioHora = entrada.PrecioHora
	}
	if entrada.Activa != nil {
		existente.Activa = *entrada.Activa
	}

	if err := s.repo.GuardarCancha(&existente); err != nil {
		return Cancha{}, fmt.Errorf("%w: %v", errEditando, err)
	}
	return existente, nil
}
