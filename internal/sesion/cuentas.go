// Package sesion responde a una sola pregunta: ¿quién hace esta petición?
// Guarda las cuentas que pueden iniciar sesión, emite y revisa los tokens.
//
// Las cuentas viven en su propia tabla ("cuentas") y no se mezclan con la
// tabla "usuarios" del negocio. El campo UsuarioID es el vínculo opcional
// con el usuario de negocio que corresponde a la cuenta.
package sesion

import "gorm.io/gorm"

// Los dos roles del negocio, con el mismo texto que usa internal/usuarios.
const (
	RolCliente       = "cliente"
	RolAdministrador = "administrador"
)

// Cuenta es una persona que puede iniciar sesión.
// De la contraseña se guarda un resumen bcrypt, nunca la contraseña.
type Cuenta struct {
	ID        uint   `json:"id"`
	Usuario   string `json:"usuario" gorm:"uniqueIndex;not null"`
	Rol       string `json:"rol"`
	ClaveHash string `json:"-"`
	UsuarioID *uint  `json:"usuario_id,omitempty" gorm:"index"` // usuario de negocio, si corresponde
}

// TableName mantiene la sesión separada de la tabla "usuarios" del negocio.
func (Cuenta) TableName() string {
	return "cuentas"
}

// RepositorioCuentas es lo único que el inicio de sesión necesita de la
// base de datos. Una interfaz permite probar el manejador sin PostgreSQL.
type RepositorioCuentas interface {
	PorUsuario(usuario string) (Cuenta, error)
}

// RepositorioCuentasGORM implementa RepositorioCuentas con GORM.
type RepositorioCuentasGORM struct {
	DB *gorm.DB
}

// PorUsuario busca una cuenta por su nombre de inicio de sesión.
// Si no existe, devuelve gorm.ErrRecordNotFound.
func (r RepositorioCuentasGORM) PorUsuario(usuario string) (Cuenta, error) {
	var c Cuenta
	err := r.DB.Where("usuario = ?", usuario).First(&c).Error
	return c, err
}
