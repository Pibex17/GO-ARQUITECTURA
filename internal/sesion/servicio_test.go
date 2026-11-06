package sesion

import (
	"errors"
	"testing"
)

func TestServicioSesionLoginEmiteTokenParaCredencialesValidas(t *testing.T) {
	cuenta := cuentaDePrueba(t)
	servicio := NuevoServicio(
		repositorioFalso{cuenta: cuenta},
		NuevosTokens(secretoDePrueba),
	)

	texto, err := servicio.Login("ana", "ana-2026")
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if texto == "" {
		t.Fatal("el servicio no devolvió un token")
	}

	sesion, err := NuevosTokens(secretoDePrueba).Leer(texto)
	if err != nil {
		t.Fatalf("el token emitido no es válido: %v", err)
	}
	if sesion.CuentaID != cuenta.ID || sesion.Usuario != cuenta.Usuario || sesion.Rol != cuenta.Rol {
		t.Fatalf("el token no identifica la cuenta: %+v", sesion)
	}
}

func TestServicioSesionLoginNoDistingueCuentaInexistenteDeClaveIncorrecta(t *testing.T) {
	casos := []struct {
		nombre string
		repo   repositorioFalso
		clave  string
	}{
		{
			nombre: "clave incorrecta",
			repo:   repositorioFalso{cuenta: cuentaDePrueba(t)},
			clave:  "clave-equivocada",
		},
		{
			nombre: "cuenta inexistente",
			repo:   repositorioFalso{err: ErrCuentaNoEncontrada},
			clave:  "ana-2026",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			servicio := NuevoServicio(caso.repo, NuevosTokens(secretoDePrueba))
			texto, err := servicio.Login("ana", caso.clave)
			if !errors.Is(err, ErrCredencialesInvalidas) {
				t.Fatalf("se esperaba ErrCredencialesInvalidas y llegó %v", err)
			}
			if texto != "" {
				t.Fatal("no se debe emitir un token con credenciales inválidas")
			}
		})
	}
}

func TestServicioSesionLoginPropagaErroresInternos(t *testing.T) {
	fallo := errors.New("fallo de base")
	servicio := NuevoServicio(
		repositorioFalso{err: fallo},
		NuevosTokens(secretoDePrueba),
	)

	texto, err := servicio.Login("ana", "ana-2026")
	if !errors.Is(err, fallo) {
		t.Fatalf("se esperaba el error interno y llegó %v", err)
	}
	if errors.Is(err, ErrCredencialesInvalidas) {
		t.Fatal("un fallo interno no debe convertirse en credenciales inválidas")
	}
	if texto != "" {
		t.Fatal("no se debe emitir un token cuando falla la persistencia")
	}
}
