//go:build integration

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"canchas-sinteticas/internal/sesion"
)

// secretoDeIntegracion es de juguete: solo firma los tokens de esta prueba.
const secretoDeIntegracion = "secreto-de-integracion-de-al-menos-32-bytes"

// TestAPIExtremoAExtremo recorre el guion de demostración del Hito 2
// contra un servidor HTTP real y PostgreSQL real. Necesita TEST_DATABASE_URL
// apuntando a una base DESECHABLE (prepararBase la borra y la vuelve a sembrar).
//
// Ejecución:
//
//	TEST_DATABASE_URL="host=localhost user=postgres password=postgres dbname=canchas_test port=5432 sslmode=disable" \
//	  go test -tags=integration . -run TestAPIExtremoAExtremo -v -count=1
func TestAPIExtremoAExtremo(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("defina TEST_DATABASE_URL para ejecutar la prueba de integración")
	}

	db, err := gorm.Open(postgres.Open(dsn),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("no se pudo conectar a TEST_DATABASE_URL: %v", err)
	}
	if err := prepararBase(db, true); err != nil {
		t.Fatalf("no se pudo preparar la base de prueba: %v", err)
	}

	servidor := httptest.NewServer(nuevoEnrutador(db, sesion.NuevosTokens(secretoDeIntegracion)))
	defer servidor.Close()

	// Preparación 0: login del administrador.
	tokenAdmin := iniciarSesion(t, servidor.URL, "luis", "luis-2026")

	// 1. Ruta protegida sin token.
	codigo, cuerpo := pedir(t, servidor.URL, http.MethodGet, "/reservas", "", "")
	exigirCodigo(t, "1. GET /reservas sin token", codigo, http.StatusUnauthorized, cuerpo)
	exigirError(t, cuerpo, "falta_token")

	// 2. Login del cliente.
	tokenCliente := iniciarSesion(t, servidor.URL, "ana", "ana-2026")

	// 3. Listado de reservas del cliente.
	codigo, cuerpo = pedir(t, servidor.URL, http.MethodGet, "/reservas", tokenCliente, "")
	exigirCodigo(t, "3. GET /reservas como cliente", codigo, http.StatusOK, cuerpo)
	var reservas []struct {
		ID        uint
		ClienteID uint
		Estado    string
	}
	datosDe(t, cuerpo, &reservas)
	if len(reservas) == 0 {
		t.Fatal("el cliente no recibió ninguna reserva sembrada")
	}
	for _, reserva := range reservas {
		if reserva.ClienteID != 2 {
			t.Fatalf("el cliente recibió una reserva ajena: %+v", reserva)
		}
	}

	// N5: GET /canchas sigue funcionando para ambos roles.
	codigo, cuerpo = pedir(t, servidor.URL, http.MethodGet, "/canchas", tokenAdmin, "")
	exigirCodigo(t, "GET /canchas como administrador", codigo, http.StatusOK, cuerpo)
	var canchas []struct {
		ID              uint
		AdministradorID uint
	}
	datosDe(t, cuerpo, &canchas)
	if !contieneCancha(canchas, 1) {
		t.Fatalf("el administrador no ve la cancha sembrada 1: %s", cuerpo)
	}

	codigo, cuerpo = pedir(t, servidor.URL, http.MethodGet, "/canchas", tokenCliente, "")
	exigirCodigo(t, "GET /canchas como cliente", codigo, http.StatusOK, cuerpo)
	datosDe(t, cuerpo, &canchas)
	if len(canchas) == 0 {
		t.Fatal("el cliente no ve canchas activas")
	}

	// 4. El cliente no puede editar una cancha.
	codigo, cuerpo = pedir(t, servidor.URL, http.MethodPatch, "/canchas/1", tokenCliente, `{}`)
	exigirCodigo(t, "4. PATCH /canchas/1 como cliente", codigo, http.StatusForbidden, cuerpo)
	exigirError(t, cuerpo, "permiso_denegado")

	// 5. El administrador sí puede; un cuerpo vacío no cambia campos.
	codigo, cuerpo = pedir(t, servidor.URL, http.MethodPatch, "/canchas/1", tokenAdmin, `{}`)
	exigirCodigo(t, "5. PATCH /canchas/1 como administrador", codigo, http.StatusOK, cuerpo)

	// 6. Transición prohibida desde "pendiente" (la reserva 1 está pendiente).
	codigo, cuerpo = pedir(t, servidor.URL, http.MethodPatch, "/reservas/1/estado",
		tokenAdmin, `{"NuevoEstado":"confirmada"}`)
	exigirCodigo(t, "6. PATCH /reservas/1/estado prohibido", codigo,
		http.StatusUnprocessableEntity, cuerpo)
	exigirError(t, cuerpo, "transicion_invalida")

	// El 422 no debe haber guardado nada: la reserva sigue pendiente.
	codigo, cuerpo = pedir(t, servidor.URL, http.MethodGet, "/reservas/1", tokenAdmin, "")
	exigirCodigo(t, "GET /reservas/1 tras el 422", codigo, http.StatusOK, cuerpo)
	var reserva struct {
		ID     uint
		Estado string
	}
	datosDe(t, cuerpo, &reserva)
	if reserva.ID != 1 || reserva.Estado != "pendiente" {
		t.Fatalf("la reserva cambió tras una transición inválida: %+v", reserva)
	}
}

type envoltura struct {
	OK    bool            `json:"ok"`
	Datos json.RawMessage `json:"datos"`
	Error *struct {
		Codigo  string `json:"codigo"`
		Mensaje string `json:"mensaje"`
	} `json:"error"`
}

func iniciarSesion(t *testing.T, base, usuario, clave string) string {
	t.Helper()
	cuerpo := `{"usuario":"` + usuario + `","clave":"` + clave + `"}`
	codigo, respuesta := pedir(t, base, http.MethodPost, "/login", "", cuerpo)
	exigirCodigo(t, "POST /login de "+usuario, codigo, http.StatusOK, respuesta)

	var datos struct {
		Token string `json:"token"`
	}
	datosDe(t, respuesta, &datos)
	if datos.Token == "" {
		t.Fatalf("el login de %s no devolvió token: %s", usuario, respuesta)
	}
	return datos.Token
}

func pedir(t *testing.T, base, metodo, ruta, token, cuerpo string) (int, []byte) {
	t.Helper()

	var lector io.Reader
	if cuerpo != "" {
		lector = strings.NewReader(cuerpo)
	}
	peticion, err := http.NewRequest(metodo, base+ruta, lector)
	if err != nil {
		t.Fatalf("no se pudo armar %s %s: %v", metodo, ruta, err)
	}
	if cuerpo != "" {
		peticion.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		peticion.Header.Set("Authorization", "Bearer "+token)
	}

	cliente := &http.Client{Timeout: 10 * time.Second}
	respuesta, err := cliente.Do(peticion)
	if err != nil {
		t.Fatalf("%s %s falló: %v", metodo, ruta, err)
	}
	defer respuesta.Body.Close()

	datos, err := io.ReadAll(respuesta.Body)
	if err != nil {
		t.Fatalf("no se pudo leer la respuesta de %s %s: %v", metodo, ruta, err)
	}
	return respuesta.StatusCode, datos
}

func exigirCodigo(t *testing.T, caso string, obtenido, esperado int, cuerpo []byte) {
	t.Helper()
	if obtenido != esperado {
		t.Fatalf("%s: se esperaba %d y llegó %d con cuerpo %s",
			caso, esperado, obtenido, cuerpo)
	}
}

func exigirError(t *testing.T, cuerpo []byte, codigoEsperado string) {
	t.Helper()

	var respuesta envoltura
	if err := json.Unmarshal(cuerpo, &respuesta); err != nil {
		t.Fatalf("la respuesta no es JSON: %v (%s)", err, cuerpo)
	}
	if respuesta.Error == nil || respuesta.Error.Codigo != codigoEsperado {
		t.Fatalf("se esperaba el error %q y llegó %s", codigoEsperado, cuerpo)
	}
}

func datosDe(t *testing.T, cuerpo []byte, destino any) {
	t.Helper()

	var respuesta envoltura
	if err := json.Unmarshal(cuerpo, &respuesta); err != nil {
		t.Fatalf("la respuesta no es JSON: %v (%s)", err, cuerpo)
	}
	if !respuesta.OK {
		t.Fatalf("la respuesta no es de éxito: %s", cuerpo)
	}
	if err := json.Unmarshal(respuesta.Datos, destino); err != nil {
		t.Fatalf("no se pudo leer datos: %v (%s)", err, cuerpo)
	}
}

func contieneCancha(canchas []struct {
	ID              uint
	AdministradorID uint
}, id uint) bool {
	for _, cancha := range canchas {
		if cancha.ID == id {
			return true
		}
	}
	return false
}
