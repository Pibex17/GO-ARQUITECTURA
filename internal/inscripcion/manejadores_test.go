package inscripcion

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"canchas-sinteticas/internal/sesion"
	"canchas-sinteticas/internal/usuarios"
)

var tokensDePrueba = sesion.NuevosTokens("secreto-solo-de-las-pruebas-inscripcion")

// enrutador arma el servidor de prueba sin repositorio. Sirve para las
// pruebas que responden antes de llegar al servicio (401, 403, 400).
func enrutador() http.Handler {
	r := chi.NewRouter()
	(&Manejador{
		Servicio: NuevoServicio(nil),
		Vinculos: vinculosFalsos{usuario: usuarios.Usuario{ID: 2, Rol: sesion.RolCliente}},
	}).Rutas(r, tokensDePrueba)
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

// TestInscripcionSinTokenResponde401: inscribirse exige sesión.
func TestInscripcionSinTokenResponde401(t *testing.T) {
	peticion := httptest.NewRequest(http.MethodPost,
		"/partidos/1/inscripciones", strings.NewReader(`{}`))
	grabadora := httptest.NewRecorder()
	enrutador().ServeHTTP(grabadora, peticion)
	if grabadora.Code != http.StatusUnauthorized {
		t.Fatalf("se esperaba 401 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
}

// TestInscripcionConAdministradorResponde403: la tabla de roles de la
// ficha (sección 6) dice que el administrador no se inscribe.
func TestInscripcionConAdministradorResponde403(t *testing.T) {
	peticion := httptest.NewRequest(http.MethodPost,
		"/partidos/1/inscripciones", strings.NewReader(`{}`))
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

// TestInscripcionConClienteLlegaAlManejador: el rol autorizado pasa y el
// manejador valida los datos obligatorios antes de tocar la base.
func TestInscripcionConClienteLlegaAlManejador(t *testing.T) {
	casos := []struct {
		nombre string
		cuerpo string
		codigo int
	}{
		{"JSON roto responde 400", `{`, http.StatusBadRequest},
		{"faltan datos responde 422", `{}`, http.StatusUnprocessableEntity},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			peticion := httptest.NewRequest(http.MethodPost,
				"/partidos/1/inscripciones", strings.NewReader(caso.cuerpo))
			peticion = conRol(t, peticion, sesion.RolCliente)
			grabadora := httptest.NewRecorder()
			enrutador().ServeHTTP(grabadora, peticion)
			if grabadora.Code != caso.codigo {
				t.Fatalf("se esperaba %d del manejador y llegó %d con cuerpo %s",
					caso.codigo, grabadora.Code, grabadora.Body.String())
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

type repoInscripcionFalso struct {
	partido      Partido
	errPartido   error
	existente    *Inscripcion
	errExistente error
	total        int64
	errTotal     error
	creada       *Inscripcion
	errCrear     error
}

func (r *repoInscripcionFalso) PartidoPorID(uint) (Partido, error) {
	if r.errPartido != nil {
		return Partido{}, r.errPartido
	}
	if r.partido.ID == 0 {
		return Partido{}, ErrNoEncontrado
	}
	return r.partido, nil
}

func (r *repoInscripcionFalso) InscripcionPorPartidoYJugador(uint, uint) (Inscripcion, error) {
	if r.errExistente != nil {
		return Inscripcion{}, r.errExistente
	}
	if r.existente != nil {
		return *r.existente, nil
	}
	return Inscripcion{}, ErrNoEncontrado
}

func (r *repoInscripcionFalso) ContarInscripciones(uint) (int64, error) {
	return r.total, r.errTotal
}

func (r *repoInscripcionFalso) CrearInscripcion(inscripcion *Inscripcion) error {
	if r.errCrear != nil {
		return r.errCrear
	}
	copia := *inscripcion
	r.creada = &copia
	return nil
}

func enrutadorCon(repo Repositorio, vinculos sesion.ResolutorVinculos) http.Handler {
	r := chi.NewRouter()
	(&Manejador{Servicio: NuevoServicio(repo), Vinculos: vinculos}).Rutas(r, tokensDePrueba)
	return r
}

// TestClienteSeInscribeConSuPropioUsuario cubre los casos 1 y 6: un
// JugadorID del cuerpo no permite inscribir a otra persona.
func TestClienteSeInscribeConSuPropioUsuario(t *testing.T) {
	repo := &repoInscripcionFalso{
		partido: Partido{ID: 1, CodigoInvitacion: "ABC123", CupoMaximo: 10},
	}
	enrutador := enrutadorCon(repo, vinculosFalsos{
		usuario: usuarios.Usuario{ID: 2, Rol: sesion.RolCliente},
	})

	peticion := httptest.NewRequest(http.MethodPost, "/partidos/1/inscripciones",
		strings.NewReader(`{"CodigoInvitacion":"ABC123","JugadorID":99}`))
	peticion = conRol(t, peticion, sesion.RolCliente)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusCreated {
		t.Fatalf("se esperaba 201 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if repo.creada == nil {
		t.Fatal("no se guardó la inscripción")
	}
	if repo.creada.JugadorID != 2 {
		t.Fatalf("la inscripción quedó a nombre del jugador %d y debía ser el 2", repo.creada.JugadorID)
	}
}

// TestCuentaSinVinculoNoSeInscribe cubre el caso 5 para inscripciones.
func TestCuentaSinVinculoNoSeInscribe(t *testing.T) {
	repo := &repoInscripcionFalso{}
	enrutador := enrutadorCon(repo, vinculosFalsos{err: sesion.ErrSinVinculo})

	peticion := httptest.NewRequest(http.MethodPost, "/partidos/1/inscripciones",
		strings.NewReader(`{"CodigoInvitacion":"ABC123"}`))
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
	if repo.creada != nil {
		t.Fatal("no se debió guardar una inscripción sin vínculo")
	}
}
