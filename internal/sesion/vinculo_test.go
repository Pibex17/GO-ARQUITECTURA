package sesion

import (
	"errors"
	"testing"

	"canchas-sinteticas/internal/usuarios"
)

type repoVinculosFalso struct {
	cuenta        Cuenta
	usuario       usuarios.Usuario
	errorCuenta   error
	errorUsuario  error
	pedidaCuenta  uint
	pedidoUsuario uint
}

func (r *repoVinculosFalso) CuentaPorID(id uint) (Cuenta, error) {
	r.pedidaCuenta = id
	return r.cuenta, r.errorCuenta
}

func (r *repoVinculosFalso) UsuarioPorID(id uint) (usuarios.Usuario, error) {
	r.pedidoUsuario = id
	return r.usuario, r.errorUsuario
}

func TestVinculosUsaElIDDeCuentaYNoElDeUsuario(t *testing.T) {
	usuarioID := uint(7)
	repo := &repoVinculosFalso{
		cuenta:  Cuenta{ID: 42, Rol: RolCliente, UsuarioID: &usuarioID},
		usuario: usuarios.Usuario{ID: usuarioID, Rol: RolCliente},
	}

	u, err := (Vinculos{Repo: repo}).UsuarioDe(Sesion{CuentaID: 42, Rol: RolCliente})
	if err != nil {
		t.Fatalf("se esperaba un vínculo válido y llegó %v", err)
	}
	if u.ID != usuarioID {
		t.Fatalf("se esperaba el usuario %d y llegó %d", usuarioID, u.ID)
	}
	if repo.pedidaCuenta != 42 {
		t.Fatalf("se debió buscar la cuenta 42 y se buscó %d", repo.pedidaCuenta)
	}
	if repo.pedidoUsuario != usuarioID {
		t.Fatalf("se debió buscar el usuario %d y se buscó %d", usuarioID, repo.pedidoUsuario)
	}
}

func TestVinculosRechazaCuentaSinUsuarioID(t *testing.T) {
	repo := &repoVinculosFalso{
		cuenta: Cuenta{ID: 42, Rol: RolCliente, UsuarioID: nil},
	}

	_, err := (Vinculos{Repo: repo}).UsuarioDe(Sesion{CuentaID: 42, Rol: RolCliente})
	if !errors.Is(err, ErrSinVinculo) {
		t.Fatalf("se esperaba ErrSinVinculo y llegó %v", err)
	}
	if repo.pedidoUsuario != 0 {
		t.Fatal("no se debe buscar un usuario cuando la cuenta no tiene vínculo")
	}
}

func TestVinculosRechazaRolIncoherente(t *testing.T) {
	usuarioID := uint(7)
	casos := []struct {
		nombre  string
		cuenta  Cuenta
		usuario usuarios.Usuario
		sesion  Sesion
	}{
		{
			nombre:  "rol de la cuenta distinto al del token",
			cuenta:  Cuenta{ID: 42, Rol: RolAdministrador, UsuarioID: &usuarioID},
			usuario: usuarios.Usuario{ID: usuarioID, Rol: RolCliente},
			sesion:  Sesion{CuentaID: 42, Rol: RolCliente},
		},
		{
			nombre:  "rol del usuario distinto al del token",
			cuenta:  Cuenta{ID: 42, Rol: RolCliente, UsuarioID: &usuarioID},
			usuario: usuarios.Usuario{ID: usuarioID, Rol: RolAdministrador},
			sesion:  Sesion{CuentaID: 42, Rol: RolCliente},
		},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			repo := &repoVinculosFalso{cuenta: caso.cuenta, usuario: caso.usuario}
			_, err := (Vinculos{Repo: repo}).UsuarioDe(caso.sesion)
			if !errors.Is(err, ErrVinculoInvalido) {
				t.Fatalf("se esperaba ErrVinculoInvalido y llegó %v", err)
			}
		})
	}
}

func TestVinculosRechazaUsuarioInexistente(t *testing.T) {
	usuarioID := uint(7)
	repo := &repoVinculosFalso{
		cuenta:       Cuenta{ID: 42, Rol: RolCliente, UsuarioID: &usuarioID},
		errorUsuario: errors.New("no encontrado"),
	}

	_, err := (Vinculos{Repo: repo}).UsuarioDe(Sesion{CuentaID: 42, Rol: RolCliente})
	if !errors.Is(err, ErrVinculoInvalido) {
		t.Fatalf("se esperaba ErrVinculoInvalido y llegó %v", err)
	}
}
