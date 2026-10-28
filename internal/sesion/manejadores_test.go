package sesion

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

type repositorioFalso struct {
	cuenta Cuenta
	err    error
}

func (r repositorioFalso) PorUsuario(string) (Cuenta, error) {
	return r.cuenta, r.err
}

type respuestaDePrueba struct {
	OK    bool `json:"ok"`
	Datos struct {
		Token string `json:"token"`
	} `json:"datos"`
	Error *struct {
		Codigo  string `json:"codigo"`
		Mensaje string `json:"mensaje"`
	} `json:"error"`
}

func enrutadorDePrueba(repo RepositorioCuentas) http.Handler {
	r := chi.NewRouter()
	(&Manejador{Cuentas: repo, Tokens: NuevosTokens(secretoDePrueba)}).Rutas(r)
	return r
}

func cuentaDePrueba(t *testing.T) Cuenta {
	t.Helper()
	resumen, err := HashClave("ana-2026")
	if err != nil {
		t.Fatalf("no se pudo calcular el resumen: %v", err)
	}
	return Cuenta{ID: 1, Usuario: "ana", Rol: RolCliente, ClaveHash: resumen}
}

func TestLoginExitosoDevuelveTokenValido(t *testing.T) {
	cuenta := cuentaDePrueba(t)
	peticion := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"usuario": "ana", "clave": "ana-2026"}`))
	grabadora := httptest.NewRecorder()

	enrutadorDePrueba(repositorioFalso{cuenta: cuenta}).ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}

	var respuesta respuestaDePrueba
	if err := json.Unmarshal(grabadora.Body.Bytes(), &respuesta); err != nil {
		t.Fatalf("la respuesta no es JSON: %v", err)
	}
	if !respuesta.OK || respuesta.Datos.Token == "" {
		t.Fatalf("respuesta sin token: %s", grabadora.Body.String())
	}
	if strings.Contains(grabadora.Body.String(), cuenta.ClaveHash) {
		t.Fatal("la respuesta expone el hash de la contraseña")
	}

	sesion, err := NuevosTokens(secretoDePrueba).Leer(respuesta.Datos.Token)
	if err != nil {
		t.Fatalf("el token devuelto no es válido: %v", err)
	}
	if sesion.Usuario != cuenta.Usuario || sesion.Rol != cuenta.Rol {
		t.Fatalf("el token no identifica la cuenta: %+v", sesion)
	}
}

// TestLoginSoloDevuelveElToken: la respuesta no incluye datos de la cuenta,
// ni la contraseña, ni el hash. El token es el único dato público.
func TestLoginSoloDevuelveElToken(t *testing.T) {
	cuenta := cuentaDePrueba(t)
	peticion := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"usuario": "ana", "clave": "ana-2026"}`))
	grabadora := httptest.NewRecorder()

	enrutadorDePrueba(repositorioFalso{cuenta: cuenta}).ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}

	var envoltura struct {
		OK    bool           `json:"ok"`
		Datos map[string]any `json:"datos"`
	}
	if err := json.Unmarshal(grabadora.Body.Bytes(), &envoltura); err != nil {
		t.Fatalf("la respuesta no es JSON: %v", err)
	}
	if !envoltura.OK {
		t.Fatalf("la respuesta no es de éxito: %s", grabadora.Body.String())
	}
	if len(envoltura.Datos) != 1 {
		t.Fatalf("la respuesta debería traer solo el token y trae %v", envoltura.Datos)
	}
	if _, existe := envoltura.Datos["token"]; !existe {
		t.Fatalf("falta el token en la respuesta: %s", grabadora.Body.String())
	}

	cuerpo := grabadora.Body.String()
	if strings.Contains(cuerpo, cuenta.ClaveHash) {
		t.Fatal("la respuesta expone el hash de la contraseña")
	}
	if strings.Contains(cuerpo, "ana-2026") {
		t.Fatal("la respuesta expone la contraseña en claro")
	}
}

// TestCuentaNoSerializaClaveHash: aunque alguien devuelva una Cuenta
// completa, el hash de la contraseña queda fuera del JSON.
func TestCuentaNoSerializaClaveHash(t *testing.T) {
	cuenta := cuentaDePrueba(t)

	datos, err := json.Marshal(cuenta)
	if err != nil {
		t.Fatalf("no se pudo serializar la cuenta: %v", err)
	}
	if strings.Contains(string(datos), "ClaveHash") {
		t.Fatalf("el JSON incluye el campo ClaveHash: %s", datos)
	}
	if strings.Contains(string(datos), cuenta.ClaveHash) {
		t.Fatalf("el JSON incluye el hash de la contraseña: %s", datos)
	}
}

// TestLoginSigueSiendoPublico: el login se registra sin el guardia de
// sesión; se puede llamar sin Authorization y entrega el token.
func TestLoginSigueSiendoPublico(t *testing.T) {
	cuenta := cuentaDePrueba(t)
	peticion := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"usuario": "ana", "clave": "ana-2026"}`))
	if peticion.Header.Get("Authorization") != "" {
		t.Fatal("la petición de login no debe llevar token")
	}
	grabadora := httptest.NewRecorder()

	enrutadorDePrueba(repositorioFalso{cuenta: cuenta}).ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
}

func TestLoginConClaveIncorrectaResponde401(t *testing.T) {
	cuenta := cuentaDePrueba(t)
	peticion := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"usuario": "ana", "clave": "clave-equivocada"}`))
	grabadora := httptest.NewRecorder()

	enrutadorDePrueba(repositorioFalso{cuenta: cuenta}).ServeHTTP(grabadora, peticion)

	verificarErrorDeLogin(t, grabadora, http.StatusUnauthorized, "credenciales_invalidas")
}

func TestLoginConCuentaInexistenteResponde401(t *testing.T) {
	peticion := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"usuario": "nadie", "clave": "ana-2026"}`))
	grabadora := httptest.NewRecorder()

	enrutadorDePrueba(repositorioFalso{err: gorm.ErrRecordNotFound}).
		ServeHTTP(grabadora, peticion)

	verificarErrorDeLogin(t, grabadora, http.StatusUnauthorized, "credenciales_invalidas")
}

func TestLoginConJSONInvalidoResponde400(t *testing.T) {
	peticion := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"usuario": "ana", "clave":`))
	grabadora := httptest.NewRecorder()

	enrutadorDePrueba(repositorioFalso{}).ServeHTTP(grabadora, peticion)

	verificarErrorDeLogin(t, grabadora, http.StatusBadRequest, "cuerpo_malformado")
}

func TestLoginConErrorInternoResponde500SinDetalles(t *testing.T) {
	const detalleInterno = "fallo interno de la base"
	peticion := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"usuario": "ana", "clave": "ana-2026"}`))
	grabadora := httptest.NewRecorder()

	enrutadorDePrueba(repositorioFalso{err: errors.New(detalleInterno)}).
		ServeHTTP(grabadora, peticion)

	verificarErrorDeLogin(t, grabadora, http.StatusInternalServerError, "error_interno")
	if strings.Contains(grabadora.Body.String(), detalleInterno) {
		t.Fatal("la respuesta interna expone detalles de la base")
	}
}

func verificarErrorDeLogin(t *testing.T, grabadora *httptest.ResponseRecorder,
	codigo int, codigoError string) {
	t.Helper()
	if grabadora.Code != codigo {
		t.Fatalf("se esperaba %d y llegó %d con cuerpo %s",
			codigo, grabadora.Code, grabadora.Body.String())
	}
	var respuesta respuestaDePrueba
	if err := json.Unmarshal(grabadora.Body.Bytes(), &respuesta); err != nil {
		t.Fatalf("la respuesta no es JSON: %v", err)
	}
	if respuesta.OK || respuesta.Error == nil {
		t.Fatalf("se esperaba una envoltura de error: %s", grabadora.Body.String())
	}
	if respuesta.Error.Codigo != codigoError {
		t.Fatalf("se esperaba el código %q y llegó %q",
			codigoError, respuesta.Error.Codigo)
	}
	if respuesta.Datos.Token != "" {
		t.Fatal("una respuesta de error incluyó un token")
	}
}
