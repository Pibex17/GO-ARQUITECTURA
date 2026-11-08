//go:build integration

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
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

// TestAPIExtremoAExtremo recorre el ciclo completo de la Saga Coreografía
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

	tokenAdmin := iniciarSesion(t, servidor.URL, "luis", "luis-2026")
	tokenCliente := iniciarSesion(t, servidor.URL, "ana", "ana-2026")

	// 0. Preparación: una reserva nueva para el ciclo de pago.
	reservaID := crearReserva(t, servidor.URL, tokenCliente)
	rutaReserva := fmt.Sprintf("/reservas/%d", reservaID)
	rutaEstado := fmt.Sprintf("/reservas/%d/estado", reservaID)

	// 1. Ruta protegida sin token.
	codigo, cuerpo := pedir(t, servidor.URL, http.MethodGet, "/reservas", "", "")
	exigirCodigo(t, "1. GET /reservas sin token", codigo, http.StatusUnauthorized, cuerpo)
	exigirError(t, cuerpo, "falta_token")

	// 2. Listado del cliente: solo ve sus reservas.
	codigo, cuerpo = pedir(t, servidor.URL, http.MethodGet, "/reservas", tokenCliente, "")
	exigirCodigo(t, "2. GET /reservas como cliente", codigo, http.StatusOK, cuerpo)
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

	// 3. GET /canchas sigue funcionando para ambos roles.
	codigo, cuerpo = pedir(t, servidor.URL, http.MethodGet, "/canchas", tokenAdmin, "")
	exigirCodigo(t, "3. GET /canchas como administrador", codigo, http.StatusOK, cuerpo)
	var canchas []struct {
		ID              uint
		AdministradorID uint
	}
	datosDe(t, cuerpo, &canchas)
	if !contieneCancha(canchas, 1) {
		t.Fatalf("el administrador no ve la cancha sembrada 1: %s", cuerpo)
	}

	// 4. Transición prohibida desde "pendiente".
	codigo, cuerpo = pedir(t, servidor.URL, http.MethodPatch, rutaEstado,
		tokenAdmin, `{"NuevoEstado":"confirmada"}`)
	exigirCodigo(t, "4. PATCH /reservas/{id}/estado prohibido", codigo,
		http.StatusUnprocessableEntity, cuerpo)
	exigirError(t, cuerpo, "transicion_invalida")

	// 5. Transición válida a pago_por_verificar: 202 + saga.
	codigo, cuerpo = pedir(t, servidor.URL, http.MethodPatch, rutaEstado,
		tokenAdmin, `{"NuevoEstado":"pago_por_verificar",`+
			`"Metodo":"efectivo","ComprobanteURL":"https://ejemplo.invalid/comprobante.jpg"}`)
	exigirCodigo(t, "5. PATCH a pago_por_verificar", codigo, http.StatusAccepted, cuerpo)
	sagaID := sagaIDDe(t, cuerpo)
	esperarSagaPaso(t, servidor.URL, tokenAdmin, sagaID, "pago_por_verificar")

	// 6. La reserva y la seña quedaron como efectos de la saga.
	codigo, cuerpo = pedir(t, servidor.URL, http.MethodGet, rutaReserva, tokenAdmin, "")
	exigirCodigo(t, "6. GET reserva en pago_por_verificar", codigo, http.StatusOK, cuerpo)
	var conPago struct {
		ID     uint
		Estado string
		Pagos  []struct {
			Tipo   string
			Monto  float64
			Estado string
		}
	}
	datosDe(t, cuerpo, &conPago)
	if conPago.ID != reservaID || conPago.Estado != "pago_por_verificar" {
		t.Fatalf("la reserva no quedó en pago_por_verificar: %+v", conPago)
	}
	if len(conPago.Pagos) != 1 ||
		conPago.Pagos[0].Tipo != "sena" ||
		conPago.Pagos[0].Estado != "pendiente" {
		t.Fatalf("no se creó la seña esperada: %+v", conPago.Pagos)
	}
	if math.Abs(conPago.Pagos[0].Monto-7.5) > 1e-9 {
		t.Fatalf("la seña no es el 30 %% de 25: %v", conPago.Pagos[0].Monto)
	}

	// 7. Confirmación: 202, saga y seña verificada.
	codigo, cuerpo = pedir(t, servidor.URL, http.MethodPatch, rutaEstado,
		tokenAdmin, `{"NuevoEstado":"confirmada"}`)
	exigirCodigo(t, "7. PATCH a confirmada", codigo, http.StatusAccepted, cuerpo)
	esperarSagaPaso(t, servidor.URL, tokenAdmin, sagaID, "reserva_confirmada")

	codigo, cuerpo = pedir(t, servidor.URL, http.MethodGet, rutaReserva, tokenAdmin, "")
	exigirCodigo(t, "7b. GET reserva confirmada", codigo, http.StatusOK, cuerpo)
	datosDe(t, cuerpo, &conPago)
	if conPago.Estado != "confirmada" {
		t.Fatalf("la reserva no quedó confirmada: %+v", conPago)
	}
	if len(conPago.Pagos) != 1 || conPago.Pagos[0].Estado != "verificado" {
		t.Fatalf("la seña no quedó verificada: %+v", conPago.Pagos)
	}

	// 8. Jugada: cierra el ciclo de la reserva.
	codigo, cuerpo = pedir(t, servidor.URL, http.MethodPatch, rutaEstado,
		tokenAdmin, `{"NuevoEstado":"jugada"}`)
	exigirCodigo(t, "8. PATCH a jugada", codigo, http.StatusAccepted, cuerpo)
	esperarSagaPaso(t, servidor.URL, tokenAdmin, sagaID, "jugada")

	codigo, cuerpo = pedir(t, servidor.URL, http.MethodGet, rutaReserva, tokenAdmin, "")
	exigirCodigo(t, "8b. GET reserva jugada", codigo, http.StatusOK, cuerpo)
	var final struct {
		Estado string
	}
	datosDe(t, cuerpo, &final)
	if final.Estado != "jugada" {
		t.Fatalf("la reserva no quedó jugada: %+v", final)
	}

	// 9. Un segundo ciclo: reserva confirmada -> solicitud de partido.
	reservaPartido := crearReserva(t, servidor.URL, tokenCliente)
	rutaEstadoPartido := fmt.Sprintf("/reservas/%d/estado", reservaPartido)
	codigo, cuerpo = pedir(t, servidor.URL, http.MethodPatch, rutaEstadoPartido,
		tokenAdmin, `{"NuevoEstado":"pago_por_verificar",`+
			`"Metodo":"transferencia","ComprobanteURL":"https://ejemplo.invalid/2.jpg"}`)
	exigirCodigo(t, "9. PATCH a pago_por_verificar (reserva 2)", codigo, http.StatusAccepted, cuerpo)
	sagaPartido := sagaIDDe(t, cuerpo)
	esperarSagaPaso(t, servidor.URL, tokenAdmin, sagaPartido, "pago_por_verificar")

	codigo, cuerpo = pedir(t, servidor.URL, http.MethodPatch, rutaEstadoPartido,
		tokenAdmin, `{"NuevoEstado":"confirmada"}`)
	exigirCodigo(t, "9b. PATCH a confirmada (reserva 2)", codigo, http.StatusAccepted, cuerpo)
	esperarSagaPaso(t, servidor.URL, tokenAdmin, sagaPartido, "reserva_confirmada")

	codigo, cuerpo = pedir(t, servidor.URL, http.MethodPost, "/partidos",
		tokenCliente, fmt.Sprintf(`{"ReservaID":%d,"Nombre":"Amistoso","CupoMaximo":10}`, reservaPartido))
	exigirCodigo(t, "9c. POST /partidos", codigo, http.StatusAccepted, cuerpo)
	esperarSagaPaso(t, servidor.URL, tokenAdmin, sagaPartido, "partido_creado")

	// 10. El partido creado se puede consultar.
	codigo, cuerpo = pedir(t, servidor.URL, http.MethodGet,
		fmt.Sprintf("/sagas/%s", sagaPartido), tokenAdmin, "")
	exigirCodigo(t, "10. GET saga con partido", codigo, http.StatusOK, cuerpo)
	var saga struct {
		RecursoID uint
		Estado    string
		Paso      string
	}
	datosDe(t, cuerpo, &saga)
	if saga.Estado != "completada" || saga.RecursoID == 0 {
		t.Fatalf("la saga no terminó con el partido: %+v", saga)
	}
	partidoID := saga.RecursoID

	codigo, cuerpo = pedir(t, servidor.URL, http.MethodGet,
		fmt.Sprintf("/partidos/%d", partidoID), tokenAdmin, "")
	exigirCodigo(t, "10b. GET partido creado", codigo, http.StatusOK, cuerpo)
	var detalle struct {
		Partido struct {
			ID     uint
			Estado string
		}
	}
	datosDe(t, cuerpo, &detalle)
	if detalle.Partido.ID != partidoID || detalle.Partido.Estado != "activo" {
		t.Fatalf("el partido no quedó activo: %+v", detalle.Partido)
	}

	// 11. Cancelación: compensa la seña y el partido.
	codigo, cuerpo = pedir(t, servidor.URL, http.MethodPatch, rutaEstadoPartido,
		tokenAdmin, `{"NuevoEstado":"cancelada"}`)
	exigirCodigo(t, "11. PATCH a cancelada", codigo, http.StatusAccepted, cuerpo)

	esperarHasta(t, "la reserva quedó cancelada", func() bool {
		codigo, cuerpo := pedir(t, servidor.URL, http.MethodGet,
			fmt.Sprintf("/reservas/%d", reservaPartido), tokenAdmin, "")
		if codigo != http.StatusOK {
			return false
		}
		var r struct {
			Estado string
		}
		if err := datosDeJSON(cuerpo, &r); err != nil {
			return false
		}
		return r.Estado == "cancelada"
	})

	esperarHasta(t, "la seña quedó anulada", func() bool {
		codigo, cuerpo := pedir(t, servidor.URL, http.MethodGet,
			fmt.Sprintf("/reservas/%d", reservaPartido), tokenAdmin, "")
		if codigo != http.StatusOK {
			return false
		}
		var r struct {
			Pagos []struct {
				Estado string
			}
		}
		if err := datosDeJSON(cuerpo, &r); err != nil {
			return false
		}
		return len(r.Pagos) == 1 && r.Pagos[0].Estado == "anulado"
	})

	esperarHasta(t, "el partido quedó cancelado", func() bool {
		codigo, cuerpo := pedir(t, servidor.URL, http.MethodGet,
			fmt.Sprintf("/partidos/%d", partidoID), tokenAdmin, "")
		if codigo != http.StatusOK {
			return false
		}
		var detalle struct {
			Partido struct {
				Estado string
			}
		}
		if err := datosDeJSON(cuerpo, &detalle); err != nil {
			return false
		}
		return detalle.Partido.Estado == "cancelado"
	})
}

func crearReserva(t *testing.T, base, token string) uint {
	t.Helper()
	fecha := time.Now().Format("2006-01-02")
	horaInicio := time.Now().Format("15:04:05.000000000")
	cuerpoReserva := fmt.Sprintf(
		`{"CanchaID":1,"Fecha":%q,"HoraInicio":%q,"HoraFin":"23:59:59"}`,
		fecha, horaInicio)
	codigo, cuerpo := pedir(t, base, http.MethodPost, "/reservas", token, cuerpoReserva)
	exigirCodigo(t, "POST /reservas (reserva nueva)", codigo, http.StatusCreated, cuerpo)

	var reserva struct {
		ID       uint
		CanchaID uint
		Estado   string
	}
	datosDe(t, cuerpo, &reserva)
	if reserva.ID == 0 || reserva.CanchaID != 1 || reserva.Estado != "pendiente" {
		t.Fatalf("la reserva de preparación no quedó utilizable: %+v", reserva)
	}
	return reserva.ID
}

func sagaIDDe(t *testing.T, cuerpo []byte) string {
	t.Helper()
	var datos struct {
		SagaID string `json:"saga_id"`
	}
	datosDe(t, cuerpo, &datos)
	if datos.SagaID == "" {
		t.Fatalf("la respuesta 202 no incluyó saga_id: %s", cuerpo)
	}
	return datos.SagaID
}

func esperarSagaPaso(t *testing.T, base, token, sagaID, paso string) {
	t.Helper()
	esperarHasta(t, "la saga llegó al paso "+paso, func() bool {
		codigo, cuerpo := pedir(t, base, http.MethodGet, "/sagas/"+sagaID, token, "")
		if codigo != http.StatusOK {
			return false
		}
		var saga struct {
			Paso string `json:"paso"`
		}
		if err := datosDeJSON(cuerpo, &saga); err != nil {
			return false
		}
		return saga.Paso == paso
	})
}

func esperarHasta(t *testing.T, descripcion string, condicion func() bool) {
	t.Helper()
	limite := time.Now().Add(15 * time.Second)
	for time.Now().Before(limite) {
		if condicion() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("tiempo de espera agotado: %s", descripcion)
}

type envoltura struct {
	OK    bool            `json:"ok"`
	Datos json.RawMessage `json:"datos"`
	Error *struct {
		Codigo  string `json:"codigo"`
		Mensaje string `json:"mensaje"`
	} `json:"error"`
}

func datosDeJSON(cuerpo []byte, destino any) error {
	var respuesta envoltura
	if err := json.Unmarshal(cuerpo, &respuesta); err != nil {
		return err
	}
	if !respuesta.OK {
		return fmt.Errorf("respuesta no exitosa: %s", cuerpo)
	}
	return json.Unmarshal(respuesta.Datos, destino)
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
	if err := datosDeJSON(cuerpo, destino); err != nil {
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
