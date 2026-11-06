package reservas // el MISMO paquete que manejadores.go

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"canchas-sinteticas/internal/canchas"
	"canchas-sinteticas/internal/sesion"
	"canchas-sinteticas/internal/usuarios"
)

// tokensDePrueba usa un secreto de juguete: no sale de este archivo.
var tokensDePrueba = sesion.NuevosTokens("secreto-solo-de-las-pruebas-reservas")

// vinculosDelToken devuelve un usuario de negocio coherente con el rol
// firmado en el token. Permite probar el orden de las comprobaciones sin
// base de datos.
type vinculosDelToken struct{}

func (vinculosDelToken) UsuarioDe(s sesion.Sesion) (usuarios.Usuario, error) {
	return usuarios.Usuario{ID: 1, Rol: s.Rol}, nil
}

// enrutador arma el servidor de prueba SIN base de datos (DB: nil).
// Sirve porque todas las pruebas de este archivo responden ANTES
// de la línea "desde aquí hace falta la base de datos".
func enrutador() http.Handler {
	r := chi.NewRouter()
	(&Manejador{Servicio: NuevoServicio(nil), Vinculos: vinculosDelToken{}}).Rutas(r, tokensDePrueba)
	return r
}

// conRol firma un token real del proyecto para el rol indicado.
func conRol(t *testing.T, peticion *http.Request, rol string) *http.Request {
	t.Helper()
	texto, err := tokensDePrueba.Emitir(sesion.Cuenta{
		ID: 1, Usuario: "prueba", Rol: rol,
	})
	if err != nil {
		t.Fatalf("no se pudo emitir el token de prueba: %v", err)
	}
	peticion.Header.Set("Authorization", "Bearer "+texto)
	return peticion
}

// TestCrear: una función, varias filas. Cada fila es un caso.
func TestCrear(t *testing.T) {
	casos := []struct {
		nombre string
		cuerpo string
		codigo int
	}{
		{
			nombre: "JSON roto responde 400",
			cuerpo: `{"CanchaID": 1`,
			codigo: http.StatusBadRequest,
		},
		{
			// Regla propia de este negocio (2b): faltan fecha, hora_inicio
			// y hora_fin. Responde antes de tocar m.DB.
			nombre: "faltan datos obligatorios responde 422",
			cuerpo: `{"CanchaID": 1, "ClienteID": 2}`,
			codigo: http.StatusUnprocessableEntity,
		},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			peticion := httptest.NewRequest(http.MethodPost, "/reservas",
				strings.NewReader(caso.cuerpo))
			peticion = conRol(t, peticion, sesion.RolCliente)
			grabadora := httptest.NewRecorder()
			enrutador().ServeHTTP(grabadora, peticion)
			if grabadora.Code != caso.codigo {
				t.Fatalf("se esperaba %d y llegó %d con cuerpo %s",
					caso.codigo, grabadora.Code, grabadora.Body.String())
			}
		})
	}
}

// TestCambiarEstado: la validación del estado pedido responde antes de
// leer la reserva de la base de datos. La ruta es solo del administrador.
func TestCambiarEstado(t *testing.T) {
	casos := []struct {
		nombre string
		cuerpo string
		codigo int
	}{
		{
			nombre: "JSON roto responde 400",
			cuerpo: `{"NuevoEstado": "confirmada"`,
			codigo: http.StatusBadRequest,
		},
		{
			nombre: "estado inventado responde 422",
			cuerpo: `{"NuevoEstado": "volando"}`,
			codigo: http.StatusUnprocessableEntity,
		},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			peticion := httptest.NewRequest(http.MethodPatch, "/reservas/1/estado",
				strings.NewReader(caso.cuerpo))
			peticion = conRol(t, peticion, sesion.RolAdministrador)
			grabadora := httptest.NewRecorder()
			enrutador().ServeHTTP(grabadora, peticion)
			if grabadora.Code != caso.codigo {
				t.Fatalf("se esperaba %d y llegó %d con cuerpo %s",
					caso.codigo, grabadora.Code, grabadora.Body.String())
			}
		})
	}
}

// El id de la ruta se valida antes de consultar la base de datos.
func TestCambiarEstadoConIDQueNoEsNumeroResponde400(t *testing.T) {
	peticion := httptest.NewRequest(http.MethodPatch, "/reservas/abc/estado",
		strings.NewReader(`{"NuevoEstado": "confirmada"}`))
	peticion = conRol(t, peticion, sesion.RolAdministrador)
	grabadora := httptest.NewRecorder()
	enrutador().ServeHTTP(grabadora, peticion)
	if grabadora.Code != http.StatusBadRequest {
		t.Fatalf("se esperaba 400 y llegó %d", grabadora.Code)
	}
}

// TestRutasSinTokenResponden401: ninguna ruta de reservas queda abierta.
func TestRutasSinTokenResponden401(t *testing.T) {
	casos := []struct {
		nombre string
		metodo string
		ruta   string
		cuerpo string
	}{
		{"crear reserva", http.MethodPost, "/reservas", `{}`},
		{"listar reservas", http.MethodGet, "/reservas", ""},
		{"ver una reserva", http.MethodGet, "/reservas/1", ""},
		{"cambiar estado", http.MethodPatch, "/reservas/1/estado", `{}`},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			peticion := httptest.NewRequest(caso.metodo, caso.ruta,
				strings.NewReader(caso.cuerpo))
			grabadora := httptest.NewRecorder()
			enrutador().ServeHTTP(grabadora, peticion)
			if grabadora.Code != http.StatusUnauthorized {
				t.Fatalf("se esperaba 401 y llegó %d con cuerpo %s",
					grabadora.Code, grabadora.Body.String())
			}
		})
	}
}

// TestRutasDeUnSoloRolRechazanElOtroRol: el rol incorrecto recibe 403 y
// no llega al manejador de negocio.
func TestRutasDeUnSoloRolRechazanElOtroRol(t *testing.T) {
	casos := []struct {
		nombre string
		metodo string
		ruta   string
		cuerpo string
		rol    string
	}{
		{
			nombre: "administrador no crea reservas",
			metodo: http.MethodPost,
			ruta:   "/reservas",
			cuerpo: `{}`,
			rol:    sesion.RolAdministrador,
		},
		{
			nombre: "cliente no cambia el estado",
			metodo: http.MethodPatch,
			ruta:   "/reservas/1/estado",
			cuerpo: `{}`,
			rol:    sesion.RolCliente,
		},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			peticion := httptest.NewRequest(caso.metodo, caso.ruta,
				strings.NewReader(caso.cuerpo))
			peticion = conRol(t, peticion, caso.rol)
			grabadora := httptest.NewRecorder()
			enrutador().ServeHTTP(grabadora, peticion)
			if grabadora.Code != http.StatusForbidden {
				t.Fatalf("se esperaba 403 y llegó %d con cuerpo %s",
					grabadora.Code, grabadora.Body.String())
			}
			if !strings.Contains(grabadora.Body.String(), "permiso_denegado") {
				t.Fatalf("falta el código permiso_denegado: %s", grabadora.Body.String())
			}
		})
	}
}

// TestClienteNoAlcanzaLaValidacionDeTransicion: aunque el cuerpo pida un
// estado inexistente o una transición prohibida, el rol cliente se rechaza
// con 403 antes de evaluar la regla de estados (nunca 422).
func TestClienteNoAlcanzaLaValidacionDeTransicion(t *testing.T) {
	casos := []struct {
		nombre string
		cuerpo string
	}{
		{"cuerpo con transición válida", `{"NuevoEstado":"jugada"}`},
		{"estado inexistente", `{"NuevoEstado":"volando"}`},
		{"JSON roto", `{"NuevoEstado":`},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			peticion := httptest.NewRequest(http.MethodPatch, "/reservas/1/estado",
				strings.NewReader(caso.cuerpo))
			peticion = conRol(t, peticion, sesion.RolCliente)
			grabadora := httptest.NewRecorder()
			enrutador().ServeHTTP(grabadora, peticion)

			if grabadora.Code != http.StatusForbidden {
				t.Fatalf("se esperaba 403 y llegó %d con cuerpo %s",
					grabadora.Code, grabadora.Body.String())
			}
			if codigo := codigoDeError(t, grabadora.Body.Bytes()); codigo != "permiso_denegado" {
				t.Fatalf("se esperaba permiso_denegado y llegó %q", codigo)
			}
		})
	}
}

// TestCambiarEstadoRechazaElRolAntesDeLaTransicion comprueba el orden
// dentro del manejador, sin el guardia SoloRol de la ruta: el cliente debe
// recibir 403 antes de que se valide el cuerpo o la transición.
func TestCambiarEstadoRechazaElRolAntesDeLaTransicion(t *testing.T) {
	manejador := &Manejador{Servicio: NuevoServicio(nil), Vinculos: vinculosDelToken{}}
	r := chi.NewRouter()
	r.With(tokensDePrueba.Autenticar).
		Patch("/reservas/{id}/estado", manejador.cambiarEstado)

	casos := []struct {
		nombre string
		rol    string
		cuerpo string
		codigo int
		error  string
	}{
		{
			nombre: "cliente con estado inexistente",
			rol:    sesion.RolCliente,
			cuerpo: `{"NuevoEstado":"volando"}`,
			codigo: http.StatusForbidden,
			error:  "permiso_denegado",
		},
		{
			nombre: "administrador con estado inexistente",
			rol:    sesion.RolAdministrador,
			cuerpo: `{"NuevoEstado":"volando"}`,
			codigo: http.StatusUnprocessableEntity,
			error:  "estado_invalido",
		},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			peticion := httptest.NewRequest(http.MethodPatch, "/reservas/1/estado",
				strings.NewReader(caso.cuerpo))
			peticion = conRol(t, peticion, caso.rol)
			grabadora := httptest.NewRecorder()
			r.ServeHTTP(grabadora, peticion)

			if grabadora.Code != caso.codigo {
				t.Fatalf("se esperaba %d y llegó %d con cuerpo %s",
					caso.codigo, grabadora.Code, grabadora.Body.String())
			}
			if codigo := codigoDeError(t, grabadora.Body.Bytes()); codigo != caso.error {
				t.Fatalf("se esperaba %q y llegó %q", caso.error, codigo)
			}
		})
	}
}

// --- Dobles para probar la propiedad sin PostgreSQL ---

type vinculosFalsos struct {
	usuario usuarios.Usuario
	err     error
}

func (v vinculosFalsos) UsuarioDe(sesion.Sesion) (usuarios.Usuario, error) {
	return v.usuario, v.err
}

type repoReservasFalso struct {
	canchas      map[uint]canchas.Cancha
	reservas     []Reserva
	conflicto    bool
	errCancha    error
	errConflicto error
	errCrear     error
	errListar    error
	errReserva   error
	errGuardar   error
	errPago      error
	errVerificar error

	filtro         FiltroReservas
	creada         *Reserva
	guardada       *Reserva
	pagoCreado     *Pago
	pagoVerificado bool
}

func (r *repoReservasFalso) CanchaPorID(id uint) (canchas.Cancha, error) {
	if r.errCancha != nil {
		return canchas.Cancha{}, r.errCancha
	}
	cancha, ok := r.canchas[id]
	if !ok {
		return canchas.Cancha{}, canchas.ErrNoEncontrada
	}
	return cancha, nil
}

func (r *repoReservasFalso) HayConflicto(uint, string, string) (bool, error) {
	return r.conflicto, r.errConflicto
}

func (r *repoReservasFalso) CrearReserva(reserva *Reserva) error {
	if r.errCrear != nil {
		return r.errCrear
	}
	copia := *reserva
	if copia.ID == 0 {
		copia.ID = uint(len(r.reservas) + 1)
	}
	*reserva = copia
	r.creada = &copia
	r.reservas = append(r.reservas, copia)
	return nil
}

func (r *repoReservasFalso) ListarReservas(filtro FiltroReservas) ([]Reserva, error) {
	r.filtro = filtro
	if r.errListar != nil {
		return nil, r.errListar
	}
	var lista []Reserva
	for _, reserva := range r.reservas {
		if filtro.Estado != "" && reserva.Estado != filtro.Estado {
			continue
		}
		if filtro.PropietarioClienteID != nil && reserva.ClienteID != *filtro.PropietarioClienteID {
			continue
		}
		if filtro.AdministradorID != nil {
			cancha, ok := r.canchas[reserva.CanchaID]
			if !ok || cancha.AdministradorID != *filtro.AdministradorID {
				continue
			}
		}
		if filtro.ClienteID != nil && reserva.ClienteID != *filtro.ClienteID {
			continue
		}
		lista = append(lista, reserva)
	}
	return lista, nil
}

func (r *repoReservasFalso) ReservaPorID(id uint) (Reserva, error) {
	if r.errReserva != nil {
		return Reserva{}, r.errReserva
	}
	for _, reserva := range r.reservas {
		if reserva.ID == id {
			return reserva, nil
		}
	}
	return Reserva{}, ErrNoEncontrada
}

// AplicarCambioEstado reproduce en memoria la transacción del repositorio
// GORM: guarda el pago, verifica la seña y actualiza la reserva.
func (r *repoReservasFalso) AplicarCambioEstado(
	reserva *Reserva, pago *Pago, verificarPago bool,
) error {
	if pago != nil {
		if r.errPago != nil {
			return r.errPago
		}
		copia := *pago
		r.pagoCreado = &copia
	}
	if verificarPago {
		if r.errVerificar != nil {
			return r.errVerificar
		}
		r.pagoVerificado = true
	}
	if r.errGuardar != nil {
		return r.errGuardar
	}
	copia := *reserva
	r.guardada = &copia
	for i := range r.reservas {
		if r.reservas[i].ID == reserva.ID {
			r.reservas[i] = copia
			return nil
		}
	}
	r.reservas = append(r.reservas, copia)
	return nil
}

func enrutadorCon(repo Repositorio, vinculos sesion.ResolutorVinculos) http.Handler {
	r := chi.NewRouter()
	(&Manejador{Servicio: NuevoServicio(repo), Vinculos: vinculos}).Rutas(r, tokensDePrueba)
	return r
}

func codigoDeError(t *testing.T, cuerpo []byte) string {
	t.Helper()
	var envoltura struct {
		Error struct {
			Codigo string `json:"codigo"`
		} `json:"error"`
	}
	if err := json.Unmarshal(cuerpo, &envoltura); err != nil {
		t.Fatalf("la respuesta no es JSON: %v", err)
	}
	return envoltura.Error.Codigo
}

func datosDe(t *testing.T, cuerpo []byte, destino any) {
	t.Helper()
	var envoltura struct {
		OK    bool            `json:"ok"`
		Datos json.RawMessage `json:"datos"`
	}
	if err := json.Unmarshal(cuerpo, &envoltura); err != nil {
		t.Fatalf("la respuesta no es JSON: %v", err)
	}
	if err := json.Unmarshal(envoltura.Datos, destino); err != nil {
		t.Fatalf("no se pudieron leer los datos: %v", err)
	}
}

// TestClienteCreaReservaParaSiMismoEIgnoraClienteIDDelCuerpo cubre el caso 1
// y el caso 6: un ClienteID del cuerpo no cambia al dueño.
func TestClienteCreaReservaParaSiMismoEIgnoraClienteIDDelCuerpo(t *testing.T) {
	repo := &repoReservasFalso{
		canchas: map[uint]canchas.Cancha{
			1: {ID: 1, Activa: true, PrecioHora: 100},
		},
	}
	vinculos := vinculosFalsos{usuario: usuarios.Usuario{ID: 2, Rol: sesion.RolCliente}}
	enrutador := enrutadorCon(repo, vinculos)

	peticion := httptest.NewRequest(http.MethodPost, "/reservas",
		strings.NewReader(`{"CanchaID":1,"ClienteID":99,"Fecha":"2026-10-21","HoraInicio":"18:00","HoraFin":"19:00"}`))
	peticion = conRol(t, peticion, sesion.RolCliente)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusCreated {
		t.Fatalf("se esperaba 201 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if repo.creada == nil {
		t.Fatal("no se guardó la reserva")
	}
	if repo.creada.ClienteID != 2 {
		t.Fatalf("la reserva quedó a nombre del usuario %d y debía ser el 2", repo.creada.ClienteID)
	}
}

// TestCrearReservaConHorarioOcupadoResponde409 cubre el conflicto de
// horario con un doble determinista, sin necesitar PostgreSQL.
func TestCrearReservaConHorarioOcupadoResponde409(t *testing.T) {
	repo := &repoReservasFalso{
		canchas: map[uint]canchas.Cancha{
			1: {ID: 1, Activa: true, PrecioHora: 100},
		},
		conflicto: true,
	}
	enrutador := enrutadorCon(repo, vinculosFalsos{
		usuario: usuarios.Usuario{ID: 2, Rol: sesion.RolCliente},
	})

	peticion := httptest.NewRequest(http.MethodPost, "/reservas",
		strings.NewReader(`{"CanchaID":1,"Fecha":"2026-10-21","HoraInicio":"18:00","HoraFin":"19:00"}`))
	peticion = conRol(t, peticion, sesion.RolCliente)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusConflict {
		t.Fatalf("se esperaba 409 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if codigo := codigoDeError(t, grabadora.Body.Bytes()); codigo != "horario_ocupado" {
		t.Fatalf("se esperaba horario_ocupado y llegó %q", codigo)
	}
	if repo.creada != nil {
		t.Fatal("no se debió guardar una reserva en un horario ocupado")
	}
}

// TestCuentaSinVinculoNoCreaReserva cubre el caso 5.
func TestCuentaSinVinculoNoCreaReserva(t *testing.T) {
	repo := &repoReservasFalso{}
	enrutador := enrutadorCon(repo, vinculosFalsos{err: sesion.ErrSinVinculo})

	peticion := httptest.NewRequest(http.MethodPost, "/reservas",
		strings.NewReader(`{"CanchaID":1,"Fecha":"2026-10-21","HoraInicio":"18:00","HoraFin":"19:00"}`))
	peticion = conRol(t, peticion, sesion.RolCliente)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusForbidden {
		t.Fatalf("se esperaba 403 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if codigo := codigoDeError(t, grabadora.Body.Bytes()); codigo != "cuenta_sin_vinculo" {
		t.Fatalf("se esperaba cuenta_sin_vinculo y llegó %q", codigo)
	}
	if repo.creada != nil {
		t.Fatal("no se debió guardar una reserva sin vínculo")
	}
}

// TestListarClienteIgnoraClienteIDDeLaQuery cubre los casos 2 y 6.
func TestListarClienteIgnoraClienteIDDeLaQuery(t *testing.T) {
	repo := &repoReservasFalso{
		canchas: map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 1}},
		reservas: []Reserva{
			{ID: 1, CanchaID: 1, ClienteID: 2},
			{ID: 2, CanchaID: 1, ClienteID: 3},
		},
	}
	vinculos := vinculosFalsos{usuario: usuarios.Usuario{ID: 2, Rol: sesion.RolCliente}}
	enrutador := enrutadorCon(repo, vinculos)

	peticion := httptest.NewRequest(http.MethodGet, "/reservas?cliente_id=3", nil)
	peticion = conRol(t, peticion, sesion.RolCliente)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	var lista []Reserva
	datosDe(t, grabadora.Body.Bytes(), &lista)
	if len(lista) != 1 || lista[0].ID != 1 {
		t.Fatalf("el cliente debía ver solo su reserva 1 y llegó %+v", lista)
	}
	if repo.filtro.PropietarioClienteID == nil || *repo.filtro.PropietarioClienteID != 2 {
		t.Fatalf("el filtro de propietario debía ser 2 y fue %+v", repo.filtro.PropietarioClienteID)
	}
	if repo.filtro.ClienteID != nil {
		t.Fatalf("la query no debía fijar ClienteID y quedó %d", *repo.filtro.ClienteID)
	}
}

// TestListarAdministradorSoloVeReservasDeSusCanchas cubre el caso 4 y el 6.
func TestListarAdministradorSoloVeReservasDeSusCanchas(t *testing.T) {
	repo := &repoReservasFalso{
		canchas: map[uint]canchas.Cancha{
			1: {ID: 1, AdministradorID: 1},
			2: {ID: 2, AdministradorID: 9},
		},
		reservas: []Reserva{
			{ID: 1, CanchaID: 1, ClienteID: 2},
			{ID: 2, CanchaID: 2, ClienteID: 3},
			{ID: 3, CanchaID: 1, ClienteID: 3},
		},
	}
	vinculos := vinculosFalsos{usuario: usuarios.Usuario{ID: 1, Rol: sesion.RolAdministrador}}
	enrutador := enrutadorCon(repo, vinculos)

	peticion := httptest.NewRequest(http.MethodGet, "/reservas?cliente_id=3", nil)
	peticion = conRol(t, peticion, sesion.RolAdministrador)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	var lista []Reserva
	datosDe(t, grabadora.Body.Bytes(), &lista)
	if len(lista) != 1 || lista[0].ID != 3 {
		t.Fatalf("el administrador debía ver solo la reserva 3 y llegó %+v", lista)
	}
	if repo.filtro.AdministradorID == nil || *repo.filtro.AdministradorID != 1 {
		t.Fatalf("el filtro de administrador debía ser 1 y fue %+v", repo.filtro.AdministradorID)
	}
	if repo.filtro.ClienteID == nil || *repo.filtro.ClienteID != 3 {
		t.Fatalf("el filtro de cliente debía ser 3 y fue %+v", repo.filtro.ClienteID)
	}
}

// TestObtenerReservaRespetaPropiedad cubre los casos 2, 3 y 4.
func TestObtenerReservaRespetaPropiedad(t *testing.T) {
	casos := []struct {
		nombre    string
		rol       string
		usuario   usuarios.Usuario
		canchas   map[uint]canchas.Cancha
		reservas  []Reserva
		codigo    int
		errorCode string
	}{
		{
			nombre:   "cliente dueño",
			rol:      sesion.RolCliente,
			usuario:  usuarios.Usuario{ID: 2, Rol: sesion.RolCliente},
			canchas:  map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 1}},
			reservas: []Reserva{{ID: 1, CanchaID: 1, ClienteID: 2}},
			codigo:   http.StatusOK,
		},
		{
			nombre:    "cliente ajeno",
			rol:       sesion.RolCliente,
			usuario:   usuarios.Usuario{ID: 2, Rol: sesion.RolCliente},
			canchas:   map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 1}},
			reservas:  []Reserva{{ID: 1, CanchaID: 1, ClienteID: 3}},
			codigo:    http.StatusForbidden,
			errorCode: "permiso_denegado",
		},
		{
			nombre:   "administrador de la cancha",
			rol:      sesion.RolAdministrador,
			usuario:  usuarios.Usuario{ID: 1, Rol: sesion.RolAdministrador},
			canchas:  map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 1}},
			reservas: []Reserva{{ID: 1, CanchaID: 1, ClienteID: 2}},
			codigo:   http.StatusOK,
		},
		{
			nombre:    "administrador de otra cancha",
			rol:       sesion.RolAdministrador,
			usuario:   usuarios.Usuario{ID: 1, Rol: sesion.RolAdministrador},
			canchas:   map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 9}},
			reservas:  []Reserva{{ID: 1, CanchaID: 1, ClienteID: 2}},
			codigo:    http.StatusForbidden,
			errorCode: "permiso_denegado",
		},
		{
			nombre:    "reserva inexistente",
			rol:       sesion.RolCliente,
			usuario:   usuarios.Usuario{ID: 2, Rol: sesion.RolCliente},
			canchas:   map[uint]canchas.Cancha{},
			codigo:    http.StatusNotFound,
			errorCode: "no_encontrado",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			repo := &repoReservasFalso{canchas: caso.canchas, reservas: caso.reservas}
			enrutador := enrutadorCon(repo, vinculosFalsos{usuario: caso.usuario})

			peticion := httptest.NewRequest(http.MethodGet, "/reservas/1", nil)
			peticion = conRol(t, peticion, caso.rol)
			grabadora := httptest.NewRecorder()
			enrutador.ServeHTTP(grabadora, peticion)

			if grabadora.Code != caso.codigo {
				t.Fatalf("se esperaba %d y llegó %d con cuerpo %s",
					caso.codigo, grabadora.Code, grabadora.Body.String())
			}
			if caso.errorCode != "" {
				if codigo := codigoDeError(t, grabadora.Body.Bytes()); codigo != caso.errorCode {
					t.Fatalf("se esperaba %q y llegó %q", caso.errorCode, codigo)
				}
			}
		})
	}
}

// TestCuentaSinVinculoNoListaNiConsulta cubre el caso 5.
func TestCuentaSinVinculoNoListaNiConsulta(t *testing.T) {
	casos := []struct {
		metodo string
		ruta   string
	}{
		{http.MethodGet, "/reservas"},
		{http.MethodGet, "/reservas/1"},
	}
	for _, caso := range casos {
		t.Run(caso.ruta, func(t *testing.T) {
			repo := &repoReservasFalso{}
			enrutador := enrutadorCon(repo, vinculosFalsos{err: sesion.ErrSinVinculo})

			peticion := httptest.NewRequest(caso.metodo, caso.ruta, nil)
			peticion = conRol(t, peticion, sesion.RolCliente)
			grabadora := httptest.NewRecorder()
			enrutador.ServeHTTP(grabadora, peticion)

			if grabadora.Code != http.StatusForbidden {
				t.Fatalf("se esperaba 403 y llegó %d con cuerpo %s",
					grabadora.Code, grabadora.Body.String())
			}
			if codigo := codigoDeError(t, grabadora.Body.Bytes()); codigo != "cuenta_sin_vinculo" {
				t.Fatalf("se esperaba cuenta_sin_vinculo y llegó %q", codigo)
			}
		})
	}
}

// TestAdministradorNoCambiaReservaDeCanchaAjena cubre el caso 3 para PATCH.
func TestAdministradorNoCambiaReservaDeCanchaAjena(t *testing.T) {
	repo := &repoReservasFalso{
		canchas:  map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 9}},
		reservas: []Reserva{{ID: 1, CanchaID: 1, ClienteID: 2, Estado: "confirmada"}},
	}
	enrutador := enrutadorCon(repo, vinculosFalsos{
		usuario: usuarios.Usuario{ID: 1, Rol: sesion.RolAdministrador},
	})

	peticion := httptest.NewRequest(http.MethodPatch, "/reservas/1/estado",
		strings.NewReader(`{"NuevoEstado":"cancelada"}`))
	peticion = conRol(t, peticion, sesion.RolAdministrador)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusForbidden {
		t.Fatalf("se esperaba 403 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if repo.guardada != nil {
		t.Fatal("no se debió guardar un cambio sobre una reserva ajena")
	}
}

// TestTransicionInvalidaResponde422 cubre el caso 7.
func TestTransicionInvalidaResponde422(t *testing.T) {
	repo := &repoReservasFalso{
		canchas:  map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 1}},
		reservas: []Reserva{{ID: 1, CanchaID: 1, ClienteID: 2, Estado: "jugada"}},
	}
	enrutador := enrutadorCon(repo, vinculosFalsos{
		usuario: usuarios.Usuario{ID: 1, Rol: sesion.RolAdministrador},
	})

	peticion := httptest.NewRequest(http.MethodPatch, "/reservas/1/estado",
		strings.NewReader(`{"NuevoEstado":"cancelada"}`))
	peticion = conRol(t, peticion, sesion.RolAdministrador)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusUnprocessableEntity {
		t.Fatalf("se esperaba 422 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if codigo := codigoDeError(t, grabadora.Body.Bytes()); codigo != "transicion_invalida" {
		t.Fatalf("se esperaba transicion_invalida y llegó %q", codigo)
	}
	if repo.guardada != nil {
		t.Fatal("no se debió guardar una transición inválida")
	}
}

// TestAdministradorCambiaReservaDeSuCancha cubre el caso 4.
func TestAdministradorCambiaReservaDeSuCancha(t *testing.T) {
	repo := &repoReservasFalso{
		canchas:  map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 1}},
		reservas: []Reserva{{ID: 1, CanchaID: 1, ClienteID: 2, Estado: "confirmada"}},
	}
	enrutador := enrutadorCon(repo, vinculosFalsos{
		usuario: usuarios.Usuario{ID: 1, Rol: sesion.RolAdministrador},
	})

	peticion := httptest.NewRequest(http.MethodPatch, "/reservas/1/estado",
		strings.NewReader(`{"NuevoEstado":"jugada"}`))
	peticion = conRol(t, peticion, sesion.RolAdministrador)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if repo.guardada == nil || repo.guardada.Estado != "jugada" {
		t.Fatalf("la reserva no quedó jugada: %+v", repo.guardada)
	}
}

// TestAdministradorSinVinculoNoCambiaEstado cubre el caso 5 en PATCH.
func TestAdministradorSinVinculoNoCambiaEstado(t *testing.T) {
	repo := &repoReservasFalso{}
	enrutador := enrutadorCon(repo, vinculosFalsos{err: sesion.ErrSinVinculo})

	peticion := httptest.NewRequest(http.MethodPatch, "/reservas/1/estado",
		strings.NewReader(`{"NuevoEstado":"cancelada"}`))
	peticion = conRol(t, peticion, sesion.RolAdministrador)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusForbidden {
		t.Fatalf("se esperaba 403 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if codigo := codigoDeError(t, grabadora.Body.Bytes()); codigo != "cuenta_sin_vinculo" {
		t.Fatalf("se esperaba cuenta_sin_vinculo y llegó %q", codigo)
	}
}
