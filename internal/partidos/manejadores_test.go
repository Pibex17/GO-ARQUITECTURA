package partidos

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"canchas-sinteticas/internal/reservas"
	"canchas-sinteticas/internal/sesion"
	"canchas-sinteticas/internal/usuarios"
)

var tokensDePrueba = sesion.NuevosTokens("secreto-solo-de-las-pruebas-partidos")

// enrutador arma el servidor de prueba SIN base de datos (DB: nil).
// Sirve porque todas las pruebas de este archivo responden ANTES
// de tocar la base de datos.
func enrutador() http.Handler {
	r := chi.NewRouter()
	(&Manejador{DB: nil}).Rutas(r, tokensDePrueba)
	return r
}

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
			cuerpo: `{"ReservaID": 1`,
			codigo: http.StatusBadRequest,
		},
		{
			nombre: "faltan datos obligatorios responde 422",
			cuerpo: `{"ReservaID": 1}`,
			codigo: http.StatusUnprocessableEntity,
		},
		{
			nombre: "cupo_maximo cero responde 422",
			cuerpo: `{"ReservaID": 1, "Nombre": "Partido", "CupoMaximo": 0}`,
			codigo: http.StatusUnprocessableEntity,
		},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			peticion := httptest.NewRequest(http.MethodPost, "/partidos",
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

// TestObtenerConIDQueNoEsNumeroResponde400: el id de la ruta se valida
// antes de consultar la base de datos. El detalle lo ve cualquier sesión.
func TestObtenerConIDQueNoEsNumeroResponde400(t *testing.T) {
	peticion := httptest.NewRequest(http.MethodGet, "/partidos/abc", nil)
	peticion = conRol(t, peticion, sesion.RolAdministrador)
	grabadora := httptest.NewRecorder()
	enrutador().ServeHTTP(grabadora, peticion)
	if grabadora.Code != http.StatusBadRequest {
		t.Fatalf("se esperaba 400 y llegó %d", grabadora.Code)
	}
}

// TestRutasSinTokenResponden401: crear y ver un partido piden sesión.
func TestRutasSinTokenResponden401(t *testing.T) {
	casos := []struct {
		nombre string
		metodo string
		ruta   string
		cuerpo string
	}{
		{"crear partido", http.MethodPost, "/partidos", `{}`},
		{"ver partido", http.MethodGet, "/partidos/1", ""},
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

// TestCrearConAdministradorResponde403: crear un partido es del cliente.
func TestCrearConAdministradorResponde403(t *testing.T) {
	peticion := httptest.NewRequest(http.MethodPost, "/partidos",
		strings.NewReader(`{}`))
	peticion = conRol(t, peticion, sesion.RolAdministrador)
	grabadora := httptest.NewRecorder()
	enrutador().ServeHTTP(grabadora, peticion)
	if grabadora.Code != http.StatusForbidden {
		t.Fatalf("se esperaba 403 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if !strings.Contains(grabadora.Body.String(), "permiso_denegado") {
		t.Fatalf("falta el código permiso_denegado: %s", grabadora.Body.String())
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

type repoPartidosFalso struct {
	reserva      reservas.Reserva
	errReserva   error
	existente    *Partido
	errExistente error
	creado       *Partido
	errCrear     error
}

func (r *repoPartidosFalso) ReservaPorID(uint) (reservas.Reserva, error) {
	if r.errReserva != nil {
		return reservas.Reserva{}, r.errReserva
	}
	if r.reserva.ID == 0 {
		return reservas.Reserva{}, gorm.ErrRecordNotFound
	}
	return r.reserva, nil
}

func (r *repoPartidosFalso) PartidoPorReservaID(uint) (Partido, error) {
	if r.errExistente != nil {
		return Partido{}, r.errExistente
	}
	if r.existente != nil {
		return *r.existente, nil
	}
	return Partido{}, gorm.ErrRecordNotFound
}

func (r *repoPartidosFalso) CrearPartido(partido *Partido) error {
	if r.errCrear != nil {
		return r.errCrear
	}
	copia := *partido
	r.creado = &copia
	return nil
}

func enrutadorCon(repo Repositorio, vinculos sesion.ResolutorVinculos) http.Handler {
	r := chi.NewRouter()
	(&Manejador{Repo: repo, Vinculos: vinculos}).Rutas(r, tokensDePrueba)
	return r
}

// TestClienteCreaPartidoDeSuReserva cubre el caso 1 para partidos.
func TestClienteCreaPartidoDeSuReserva(t *testing.T) {
	repo := &repoPartidosFalso{
		reserva: reservas.Reserva{ID: 1, ClienteID: 2, Estado: "confirmada"},
	}
	enrutador := enrutadorCon(repo, vinculosFalsos{
		usuario: usuarios.Usuario{ID: 2, Rol: sesion.RolCliente},
	})

	peticion := httptest.NewRequest(http.MethodPost, "/partidos",
		strings.NewReader(`{"ReservaID":1,"Nombre":"Amistoso","CupoMaximo":10}`))
	peticion = conRol(t, peticion, sesion.RolCliente)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusCreated {
		t.Fatalf("se esperaba 201 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if repo.creado == nil || repo.creado.ReservaID != 1 {
		t.Fatalf("no se creó el partido esperado: %+v", repo.creado)
	}
}

// TestClienteNoCreaPartidoDeReservaAjena cubre el caso 3 para partidos.
func TestClienteNoCreaPartidoDeReservaAjena(t *testing.T) {
	repo := &repoPartidosFalso{
		reserva: reservas.Reserva{ID: 1, ClienteID: 3, Estado: "confirmada"},
	}
	enrutador := enrutadorCon(repo, vinculosFalsos{
		usuario: usuarios.Usuario{ID: 2, Rol: sesion.RolCliente},
	})

	peticion := httptest.NewRequest(http.MethodPost, "/partidos",
		strings.NewReader(`{"ReservaID":1,"Nombre":"Amistoso","CupoMaximo":10}`))
	peticion = conRol(t, peticion, sesion.RolCliente)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusForbidden {
		t.Fatalf("se esperaba 403 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if repo.creado != nil {
		t.Fatal("no se debió crear un partido sobre una reserva ajena")
	}
}

// TestCuentaSinVinculoNoCreaPartido cubre el caso 5 para partidos.
func TestCuentaSinVinculoNoCreaPartido(t *testing.T) {
	repo := &repoPartidosFalso{}
	enrutador := enrutadorCon(repo, vinculosFalsos{err: sesion.ErrSinVinculo})

	peticion := httptest.NewRequest(http.MethodPost, "/partidos",
		strings.NewReader(`{"ReservaID":1,"Nombre":"Amistoso","CupoMaximo":10}`))
	peticion = conRol(t, peticion, sesion.RolCliente)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusForbidden {
		t.Fatalf("se esperaba 403 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if !strings.Contains(grabadora.Body.String(), "cuenta_sin_vinculo") {
		t.Fatalf("falta el código cuenta_sin_vinculo: %s", grabadora.Body.String())
	}
	if repo.creado != nil {
		t.Fatal("no se debió crear un partido sin vínculo")
	}
}
