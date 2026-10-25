package sesion

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestAutenticarRechazaTokenAusenteOMalFormado comprueba el primer nivel:
// si no hay una sesión válida, el manejador de negocio no se ejecuta.
func TestAutenticarRechazaTokenAusenteOMalFormado(t *testing.T) {
	tokens := NuevosTokens(secretoDePrueba)

	emisorAjeno := NuevosTokens("secreto-de-otra-aplicacion")
	tokenOtraFirma, err := emisorAjeno.Emitir(Cuenta{
		ID: 1, Usuario: "ana", Rol: RolCliente,
	})
	if err != nil {
		t.Fatalf("no se pudo emitir el token de otra firma: %v", err)
	}

	tokenVencido := firmarDePrueba(t, jwt.SigningMethodHS256, secretoDePrueba,
		time.Now().Add(-time.Minute))
	tokenOtroAlgoritmo := firmarDePrueba(t, jwt.SigningMethodHS384, secretoDePrueba,
		time.Now().Add(time.Hour))

	casos := []struct {
		nombre      string
		cabecera    string
		codigo      int
		codigoError string
	}{
		{
			nombre:      "sin encabezado",
			cabecera:    "",
			codigo:      http.StatusUnauthorized,
			codigoError: "falta_token",
		},
		{
			nombre:      "esquema distinto de Bearer",
			cabecera:    "Basic dXN1YXJpbzpjbGF2ZQ==",
			codigo:      http.StatusUnauthorized,
			codigoError: "falta_token",
		},
		{
			nombre:      "Bearer sin token",
			cabecera:    "Bearer ",
			codigo:      http.StatusUnauthorized,
			codigoError: "falta_token",
		},
		{
			nombre:      "token mal formado",
			cabecera:    "Bearer esto-no-es-un-jwt",
			codigo:      http.StatusUnauthorized,
			codigoError: "token_invalido",
		},
		{
			nombre:      "firma de otro secreto",
			cabecera:    "Bearer " + tokenOtraFirma,
			codigo:      http.StatusUnauthorized,
			codigoError: "token_invalido",
		},
		{
			nombre:      "token vencido",
			cabecera:    "Bearer " + tokenVencido,
			codigo:      http.StatusUnauthorized,
			codigoError: "token_vencido",
		},
		{
			nombre:      "algoritmo distinto de HS256",
			cabecera:    "Bearer " + tokenOtroAlgoritmo,
			codigo:      http.StatusUnauthorized,
			codigoError: "token_invalido",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			llegoAlManejador := false
			guardia := tokens.Autenticar(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				llegoAlManejador = true
				w.WriteHeader(http.StatusNoContent)
			}))

			peticion := httptest.NewRequest(http.MethodGet, "/protegida", nil)
			if caso.cabecera != "" {
				peticion.Header.Set("Authorization", caso.cabecera)
			}
			grabadora := httptest.NewRecorder()
			guardia.ServeHTTP(grabadora, peticion)

			if llegoAlManejador {
				t.Fatal("el manejador se ejecutó con una sesión inválida")
			}
			verificarErrorDeGuardia(t, grabadora, caso.codigo, caso.codigoError)
		})
	}
}

// TestAutenticarAceptaTokenValidoYGuardaSesion comprueba el camino feliz:
// el token real del proyecto pasa y la identidad queda en el contexto.
func TestAutenticarAceptaTokenValidoYGuardaSesion(t *testing.T) {
	tokens := NuevosTokens(secretoDePrueba)
	cuenta := Cuenta{ID: 7, Usuario: "ana", Rol: RolCliente}
	texto, err := tokens.Emitir(cuenta)
	if err != nil {
		t.Fatalf("no se pudo emitir el token: %v", err)
	}

	var vista Sesion
	guardia := tokens.Autenticar(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := De(r)
		if !ok {
			t.Fatal("el guardia no dejó la sesión en el contexto")
		}
		vista = s
		w.WriteHeader(http.StatusNoContent)
	}))

	peticion := httptest.NewRequest(http.MethodGet, "/protegida", nil)
	peticion.Header.Set("Authorization", "Bearer "+texto)
	grabadora := httptest.NewRecorder()
	guardia.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusNoContent {
		t.Fatalf("se esperaba 204 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	esperada := Sesion{CuentaID: cuenta.ID, Usuario: cuenta.Usuario, Rol: cuenta.Rol}
	if vista != esperada {
		t.Fatalf("la sesión del contexto no coincide: %+v", vista)
	}
}

// TestSoloRolConRolCorrectoDejaPasar comprueba que el rol firmado, no el
// que manda el cliente, es el que autoriza.
func TestSoloRolConRolCorrectoDejaPasar(t *testing.T) {
	tokens := NuevosTokens(secretoDePrueba)
	texto, err := tokens.Emitir(Cuenta{ID: 2, Usuario: "admin", Rol: RolAdministrador})
	if err != nil {
		t.Fatalf("no se pudo emitir el token: %v", err)
	}

	llegoAlManejador := false
	guardia := tokens.Autenticar(SoloRol(RolAdministrador)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			llegoAlManejador = true
			w.WriteHeader(http.StatusNoContent)
		})))

	peticion := httptest.NewRequest(http.MethodPost, "/solo-admin", nil)
	peticion.Header.Set("Authorization", "Bearer "+texto)
	grabadora := httptest.NewRecorder()
	guardia.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusNoContent {
		t.Fatalf("se esperaba 204 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if !llegoAlManejador {
		t.Fatal("el rol autorizado no llegó al manejador")
	}
}

// TestSoloRolConRolIncorrectoResponde403 comprueba el segundo nivel:
// hay sesión, pero el rol no alcanza para esta operación.
func TestSoloRolConRolIncorrectoResponde403(t *testing.T) {
	tokens := NuevosTokens(secretoDePrueba)
	texto, err := tokens.Emitir(Cuenta{ID: 3, Usuario: "ana", Rol: RolCliente})
	if err != nil {
		t.Fatalf("no se pudo emitir el token: %v", err)
	}

	llegoAlManejador := false
	guardia := tokens.Autenticar(SoloRol(RolAdministrador)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			llegoAlManejador = true
			w.WriteHeader(http.StatusNoContent)
		})))

	peticion := httptest.NewRequest(http.MethodPost, "/solo-admin", nil)
	peticion.Header.Set("Authorization", "Bearer "+texto)
	grabadora := httptest.NewRecorder()
	guardia.ServeHTTP(grabadora, peticion)

	if llegoAlManejador {
		t.Fatal("un rol sin permiso llegó al manejador")
	}
	verificarErrorDeGuardia(t, grabadora, http.StatusForbidden, "permiso_denegado")
}

// TestSoloRolSinSesionResponde401 protege el caso de usar SoloRol sin
// Autenticar: nunca debe convertirse en un pase libre.
func TestSoloRolSinSesionResponde401(t *testing.T) {
	guardia := SoloRol(RolCliente)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("el manejador se ejecutó sin sesión")
	}))

	peticion := httptest.NewRequest(http.MethodGet, "/protegida", nil)
	grabadora := httptest.NewRecorder()
	guardia.ServeHTTP(grabadora, peticion)

	verificarErrorDeGuardia(t, grabadora, http.StatusUnauthorized, "falta_token")
}

func verificarErrorDeGuardia(t *testing.T, grabadora *httptest.ResponseRecorder,
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
}
