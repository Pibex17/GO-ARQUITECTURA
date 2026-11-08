package partidos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"canchas-sinteticas/internal/eventos"
	"canchas-sinteticas/internal/reservas"
	"canchas-sinteticas/internal/sagas"
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

// TestObtenerDetallePartidoUsaRepositorio: el handler no consulta la base
// directamente; obtiene el partido y el conteo a través del repositorio.
func TestObtenerDetallePartidoUsaRepositorio(t *testing.T) {
	repo := &repoPartidosFalso{
		partido:       &Partido{ID: 1, ReservaID: 2, Nombre: "Picadito", CupoMaximo: 10},
		inscripciones: 3,
	}
	enrutador := enrutadorCon(repo, vinculosFalsos{})

	peticion := httptest.NewRequest(http.MethodGet, "/partidos/1", nil)
	peticion = conRol(t, peticion, sesion.RolCliente)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}

	var respuesta struct {
		OK    bool `json:"ok"`
		Datos struct {
			Partido        Partido `json:"partido"`
			Inscriptos     int64   `json:"inscriptos"`
			CuposRestantes int     `json:"cupos_restantes"`
		} `json:"datos"`
	}
	if err := json.Unmarshal(grabadora.Body.Bytes(), &respuesta); err != nil {
		t.Fatalf("la respuesta no es JSON: %v", err)
	}
	if !respuesta.OK || respuesta.Datos.Partido.ID != 1 {
		t.Fatalf("no se devolvió el partido esperado: %s", grabadora.Body.String())
	}
	if respuesta.Datos.Inscriptos != 3 || respuesta.Datos.CuposRestantes != 7 {
		t.Fatalf("conteo incorrecto: %s", grabadora.Body.String())
	}
}

// TestObtenerPartidoInexistenteResponde404: un error del repositorio se
// traduce al mismo 404 de antes.
func TestObtenerPartidoInexistenteResponde404(t *testing.T) {
	enrutador := enrutadorCon(&repoPartidosFalso{}, vinculosFalsos{})

	peticion := httptest.NewRequest(http.MethodGet, "/partidos/99", nil)
	peticion = conRol(t, peticion, sesion.RolCliente)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusNotFound {
		t.Fatalf("se esperaba 404 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
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
	reserva          reservas.Reserva
	errReserva       error
	existente        *Partido
	errExistente     error
	partido          *Partido
	errPartido       error
	inscripciones    int64
	errInscripciones error
	creado           *Partido
	errCrear         error
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

func (r *repoPartidosFalso) PartidoPorID(uint) (Partido, error) {
	if r.errPartido != nil {
		return Partido{}, r.errPartido
	}
	if r.partido != nil {
		return *r.partido, nil
	}
	return Partido{}, gorm.ErrRecordNotFound
}

func (r *repoPartidosFalso) ContarInscripciones(uint) (int64, error) {
	return r.inscripciones, r.errInscripciones
}

func (r *repoPartidosFalso) CrearPartidoEnTx(_ *gorm.DB, partido *Partido) error {
	if r.errCrear != nil {
		return r.errCrear
	}
	copia := *partido
	r.creado = &copia
	return nil
}

func (r *repoPartidosFalso) PartidoPorReservaIDEnTx(_ *gorm.DB, reservaID uint) (Partido, error) {
	return r.PartidoPorReservaID(reservaID)
}

func (r *repoPartidosFalso) GuardarPartidoEnTx(_ *gorm.DB, partido *Partido) error {
	copia := *partido
	r.partido = &copia
	return nil
}

// publicadorFalso registra los eventos publicados por el manejador.
type publicadorFalso struct {
	eventos []eventos.Evento
	err     error
}

func (p *publicadorFalso) Publicar(_ context.Context, evento eventos.Evento) error {
	if p.err != nil {
		return p.err
	}
	p.eventos = append(p.eventos, evento)
	return nil
}

// sagasFalsas devuelve una proyección mínima sin base de datos.
type sagasFalsas struct {
	saga sagas.Saga
	err  error
}

func (s sagasFalsas) Asegurar(_ context.Context, tipo string, agregadoID uint) (sagas.Saga, error) {
	if s.err != nil {
		return sagas.Saga{}, s.err
	}
	if s.saga.ID != "" {
		return s.saga, nil
	}
	return sagas.Saga{
		ID:         sagas.IDDeReserva(agregadoID),
		Tipo:       tipo,
		AgregadoID: agregadoID,
		Estado:     sagas.EstadoIniciada,
		Paso:       "iniciada",
	}, nil
}

func (s sagasFalsas) Obtener(_ context.Context, id string) (sagas.Saga, error) {
	return s.saga, s.err
}

func enrutadorCon(repo Repositorio, vinculos sesion.ResolutorVinculos) http.Handler {
	return enrutadorConDependencias(repo, vinculos, &publicadorFalso{}, sagasFalsas{})
}

func enrutadorConDependencias(
	repo Repositorio,
	vinculos sesion.ResolutorVinculos,
	publicador eventos.Publicador,
	registro sagas.Registro,
) http.Handler {
	r := chi.NewRouter()
	(&Manejador{
		Repo:     repo,
		Vinculos: vinculos,
		Eventos:  publicador,
		Sagas:    registro,
	}).Rutas(r, tokensDePrueba)
	return r
}

// TestClienteCreaPartidoDeSuReserva comprueba que el manejador acepta la
// solicitud con 202 y publica el evento; el partido lo crea la saga.
func TestClienteCreaPartidoDeSuReserva(t *testing.T) {
	repo := &repoPartidosFalso{
		reserva: reservas.Reserva{ID: 1, ClienteID: 2, Estado: "confirmada"},
	}
	publicador := &publicadorFalso{}
	enrutador := enrutadorConDependencias(repo, vinculosFalsos{
		usuario: usuarios.Usuario{ID: 2, Rol: sesion.RolCliente},
	}, publicador, sagasFalsas{})

	peticion := httptest.NewRequest(http.MethodPost, "/partidos",
		strings.NewReader(`{"ReservaID":1,"Nombre":"Amistoso","CupoMaximo":10}`))
	peticion = conRol(t, peticion, sesion.RolCliente)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusAccepted {
		t.Fatalf("se esperaba 202 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if len(publicador.eventos) != 1 || publicador.eventos[0].Tipo != eventos.TipoSolicitudPartidoRecibida {
		t.Fatalf("no se publicó la solicitud esperada: %+v", publicador.eventos)
	}
	if repo.creado != nil {
		t.Fatal("el manejador no debe crear el partido directamente")
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
