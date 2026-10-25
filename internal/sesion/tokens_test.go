package sesion

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const secretoDePrueba = "secreto-solo-de-las-pruebas"

func TestEmitirYLeerTokenValido(t *testing.T) {
	tokens := NuevosTokens(secretoDePrueba)
	cuenta := Cuenta{ID: 7, Usuario: "ana", Rol: RolCliente}

	texto, err := tokens.Emitir(cuenta)
	if err != nil {
		t.Fatalf("no se pudo emitir el token: %v", err)
	}
	if texto == "" {
		t.Fatal("el token quedó vacío")
	}

	sesion, err := tokens.Leer(texto)
	if err != nil {
		t.Fatalf("un token válido fue rechazado: %v", err)
	}
	if sesion.CuentaID != cuenta.ID || sesion.Usuario != cuenta.Usuario || sesion.Rol != cuenta.Rol {
		t.Fatalf("la sesión no coincide: %+v", sesion)
	}
}

func TestLeerTokenVencidoFalla(t *testing.T) {
	tokens := NuevosTokens(secretoDePrueba)
	texto := firmarDePrueba(t, jwt.SigningMethodHS256, secretoDePrueba,
		time.Now().Add(-time.Minute))

	if _, err := tokens.Leer(texto); !errors.Is(err, ErrVencido) {
		t.Fatalf("se esperaba ErrVencido y llegó %v", err)
	}
}

func TestLeerTokenConFirmaInvalidaFalla(t *testing.T) {
	emisor := NuevosTokens("secreto-del-emisor")
	texto, err := emisor.Emitir(Cuenta{ID: 1, Usuario: "ana", Rol: RolCliente})
	if err != nil {
		t.Fatalf("no se pudo emitir el token: %v", err)
	}

	lector := NuevosTokens("secreto-distinto")
	if _, err := lector.Leer(texto); err == nil {
		t.Fatal("un token con otra firma fue aceptado")
	}
}

func TestLeerTokenConAlgoritmoDistintoDeHS256Falla(t *testing.T) {
	tokens := NuevosTokens(secretoDePrueba)
	texto := firmarDePrueba(t, jwt.SigningMethodHS384, secretoDePrueba,
		time.Now().Add(time.Hour))

	if _, err := tokens.Leer(texto); err == nil {
		t.Fatal("un token HS384 fue aceptado")
	}
}

func TestLeerTokenSinExpiracionFalla(t *testing.T) {
	tokens := NuevosTokens(secretoDePrueba)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, datos{
		Usuario: "ana",
		Rol:     RolCliente,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  "1",
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	})
	texto, err := token.SignedString([]byte(secretoDePrueba))
	if err != nil {
		t.Fatalf("no se pudo firmar el token de prueba: %v", err)
	}

	if _, err := tokens.Leer(texto); err == nil {
		t.Fatal("un token sin expiración fue aceptado")
	}
}

// firmarDePrueba crea un token con los reclamos de la sesión y la fecha de
// vencimiento indicada. Sirve para probar casos que Emitir no produce.
func firmarDePrueba(t *testing.T, metodo jwt.SigningMethod, secreto string,
	vencimiento time.Time) string {
	t.Helper()
	token := jwt.NewWithClaims(metodo, datos{
		Usuario: "ana",
		Rol:     RolCliente,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "1",
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(vencimiento),
		},
	})
	texto, err := token.SignedString([]byte(secreto))
	if err != nil {
		t.Fatalf("no se pudo firmar el token de prueba: %v", err)
	}
	return texto
}
