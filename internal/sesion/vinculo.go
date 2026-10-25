package sesion

import (
	"errors"

	"gorm.io/gorm"

	"canchas-sinteticas/internal/usuarios"
)

// ErrSinVinculo indica que la cuenta existe, pero no está asociada a un
// usuario de negocio. No se debe inventar una identidad en ese caso.
var ErrSinVinculo = errors.New("la cuenta no está vinculada a un usuario de negocio")

// ErrVinculoInvalido indica que el vínculo existe, pero los datos no son
// coherentes (usuario inexistente o rol distinto al de la cuenta).
var ErrVinculoInvalido = errors.New("el vínculo de la cuenta no es válido")

// ResolutorVinculos es lo que necesitan los manejadores para pasar de una
// sesión (cuenta) al usuario de negocio que realmente posee los recursos.
type ResolutorVinculos interface {
	UsuarioDe(s Sesion) (usuarios.Usuario, error)
}

// RepositorioVinculos es la parte de la base que hace falta para resolver
// el vínculo. Una interfaz permite probarlo con dobles.
type RepositorioVinculos interface {
	CuentaPorID(id uint) (Cuenta, error)
	UsuarioPorID(id uint) (usuarios.Usuario, error)
}

// Vinculos resuelve la cuenta del token contra la tabla cuentas y luego
// contra la tabla usuarios. El sub del token es CuentaID, nunca el ID del
// usuario de negocio.
type Vinculos struct {
	Repo RepositorioVinculos
}

// UsuarioDe devuelve el usuario de negocio vinculado a la cuenta.
// Exige que la cuenta tenga UsuarioID y que el rol de la cuenta, el del
// usuario y el firmado en el token coincidan.
func (v Vinculos) UsuarioDe(s Sesion) (usuarios.Usuario, error) {
	if v.Repo == nil {
		return usuarios.Usuario{}, ErrSinVinculo
	}

	c, err := v.Repo.CuentaPorID(s.CuentaID)
	if err != nil {
		return usuarios.Usuario{}, ErrSinVinculo
	}
	if c.UsuarioID == nil {
		return usuarios.Usuario{}, ErrSinVinculo
	}
	if c.Rol != s.Rol {
		return usuarios.Usuario{}, ErrVinculoInvalido
	}

	u, err := v.Repo.UsuarioPorID(*c.UsuarioID)
	if err != nil {
		return usuarios.Usuario{}, ErrVinculoInvalido
	}
	if u.Rol != s.Rol {
		return usuarios.Usuario{}, ErrVinculoInvalido
	}
	return u, nil
}

// RepositorioVinculosGORM implementa RepositorioVinculos con GORM.
type RepositorioVinculosGORM struct {
	DB *gorm.DB
}

func (r RepositorioVinculosGORM) CuentaPorID(id uint) (Cuenta, error) {
	var c Cuenta
	err := r.DB.First(&c, id).Error
	return c, err
}

func (r RepositorioVinculosGORM) UsuarioPorID(id uint) (usuarios.Usuario, error) {
	var u usuarios.Usuario
	err := r.DB.First(&u, id).Error
	return u, err
}
