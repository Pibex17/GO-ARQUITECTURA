package sesion

import (
	"errors"

	"canchas-sinteticas/internal/usuarios"
)

// RepositorioCuentas es lo único que el inicio de sesión necesita de la
// base de datos. La interfaz vive en la capa de servicio; su implementación
// GORM está en cuentas.go.
type RepositorioCuentas interface {
	PorUsuario(usuario string) (Cuenta, error)
}

// RepositorioVinculos es la persistencia que necesita la resolución de la
// cuenta autenticada a su usuario de negocio. La implementación GORM está en
// vinculo.go.
type RepositorioVinculos interface {
	CuentaPorID(id uint) (Cuenta, error)
	UsuarioPorID(id uint) (usuarios.Usuario, error)
}

// ErrCredencialesInvalidas es el único error que el transporte debe
// traducir a 401. No distingue «no existe» de «contraseña incorrecta».
var ErrCredencialesInvalidas = errors.New("usuario o contraseña incorrectos")

// Servicio contiene el caso de uso de inicio de sesión. No conoce HTTP:
// recibe credenciales y devuelve un token o un error de negocio.
type Servicio struct {
	cuentas RepositorioCuentas
	tokens  *Tokens
}

// NuevoServicio inyecta la persistencia de cuentas y el emisor de tokens.
func NuevoServicio(cuentas RepositorioCuentas, tokens *Tokens) *Servicio {
	return &Servicio{cuentas: cuentas, tokens: tokens}
}

// Login verifica las credenciales y emite el token de la cuenta.
func (s *Servicio) Login(usuario, clave string) (string, error) {
	cuenta, err := s.cuentas.PorUsuario(usuario)
	if errors.Is(err, ErrCuentaNoEncontrada) {
		return "", ErrCredencialesInvalidas
	}
	if err != nil {
		return "", err
	}

	if !ClaveCoincide(cuenta.ClaveHash, clave) {
		return "", ErrCredencialesInvalidas
	}

	return s.tokens.Emitir(cuenta)
}
