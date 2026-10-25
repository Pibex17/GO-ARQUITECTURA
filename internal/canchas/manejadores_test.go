package canchas

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"canchas-sinteticas/internal/sesion"
	"canchas-sinteticas/internal/usuarios"
)

var tokensDePrueba = sesion.NuevosTokens("secreto-solo-de-las-pruebas-canchas")

// enrutador arma el servidor de prueba SIN base de datos (DB: nil).
// Las pruebas de autenticación y rol responden antes de tocar m.DB.
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

// TestRutasSinTokenResponden401: listar, crear y editar piden sesión.
func TestRutasSinTokenResponden401(t *testing.T) {
	casos := []struct {
		nombre string
		metodo string
		ruta   string
		cuerpo string
	}{
		{"listar canchas", http.MethodGet, "/canchas", ""},
		{"crear cancha", http.MethodPost, "/canchas", `{}`},
		{"editar cancha", http.MethodPatch, "/canchas/1", `{}`},
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

// TestClienteNoAdministraCanchas: crear y editar son solo del administrador.
func TestClienteNoAdministraCanchas(t *testing.T) {
	casos := []struct {
		nombre string
		metodo string
		ruta   string
	}{
		{"crear cancha", http.MethodPost, "/canchas"},
		{"editar cancha", http.MethodPatch, "/canchas/1"},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			peticion := httptest.NewRequest(caso.metodo, caso.ruta,
				strings.NewReader(`{}`))
			peticion = conRol(t, peticion, sesion.RolCliente)
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

// TestAdministradorLlegaAlManejador: el rol correcto pasa el guardia y
// el manejador valida el cuerpo antes de tocar la base de datos.
func TestAdministradorLlegaAlManejador(t *testing.T) {
	peticion := httptest.NewRequest(http.MethodPost, "/canchas",
		strings.NewReader(`{`))
	peticion = conRol(t, peticion, sesion.RolAdministrador)
	grabadora := httptest.NewRecorder()
	enrutador().ServeHTTP(grabadora, peticion)
	if grabadora.Code != http.StatusBadRequest {
		t.Fatalf("se esperaba 400 del manejador y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
}

// TestAdministradorConIDInvalidoLlegaAlManejador: PATCH valida el id
// antes de consultar la base de datos.
func TestAdministradorConIDInvalidoLlegaAlManejador(t *testing.T) {
	peticion := httptest.NewRequest(http.MethodPatch, "/canchas/abc",
		strings.NewReader(`{}`))
	peticion = conRol(t, peticion, sesion.RolAdministrador)
	grabadora := httptest.NewRecorder()
	enrutador().ServeHTTP(grabadora, peticion)
	if grabadora.Code != http.StatusBadRequest {
		t.Fatalf("se esperaba 400 del manejador y llegó %d con cuerpo %s",
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

type repoCanchasFalso struct {
	canchas    []Cancha
	errCancha  error
	errListar  error
	errCrear   error
	errGuardar error
	filtro     FiltroCanchas
	creada     *Cancha
	guardada   *Cancha
}

func (r *repoCanchasFalso) CanchaPorID(id uint) (Cancha, error) {
	if r.errCancha != nil {
		return Cancha{}, r.errCancha
	}
	for _, cancha := range r.canchas {
		if cancha.ID == id {
			return cancha, nil
		}
	}
	return Cancha{}, gorm.ErrRecordNotFound
}

func (r *repoCanchasFalso) ListarCanchas(filtro FiltroCanchas) ([]Cancha, error) {
	r.filtro = filtro
	if r.errListar != nil {
		return nil, r.errListar
	}
	var lista []Cancha
	for _, cancha := range r.canchas {
		if filtro.Activa != nil && cancha.Activa != *filtro.Activa {
			continue
		}
		if filtro.AdministradorID != nil && cancha.AdministradorID != *filtro.AdministradorID {
			continue
		}
		lista = append(lista, cancha)
	}
	return lista, nil
}

func (r *repoCanchasFalso) CrearCancha(cancha *Cancha) error {
	if r.errCrear != nil {
		return r.errCrear
	}
	copia := *cancha
	if copia.ID == 0 {
		copia.ID = uint(len(r.canchas) + 1)
	}
	*cancha = copia
	r.creada = &copia
	r.canchas = append(r.canchas, copia)
	return nil
}

func (r *repoCanchasFalso) GuardarCancha(cancha *Cancha) error {
	if r.errGuardar != nil {
		return r.errGuardar
	}
	copia := *cancha
	r.guardada = &copia
	for i := range r.canchas {
		if r.canchas[i].ID == cancha.ID {
			r.canchas[i] = copia
			return nil
		}
	}
	r.canchas = append(r.canchas, copia)
	return nil
}

func enrutadorCon(repo Repositorio, vinculos sesion.ResolutorVinculos) http.Handler {
	r := chi.NewRouter()
	(&Manejador{Repo: repo, Vinculos: vinculos}).Rutas(r, tokensDePrueba)
	return r
}

// TestAdministradorCreaCanchaParaSiMismo cubre el caso 6: ignorar el
// AdministradorID del cuerpo y usar el de la sesión.
func TestAdministradorCreaCanchaParaSiMismo(t *testing.T) {
	repo := &repoCanchasFalso{}
	enrutador := enrutadorCon(repo, vinculosFalsos{
		usuario: usuarios.Usuario{ID: 1, Rol: sesion.RolAdministrador},
	})

	peticion := httptest.NewRequest(http.MethodPost, "/canchas",
		strings.NewReader(`{"Nombre":"Central","Tipo":"futbol_5","Direccion":"Calle 1","PrecioHora":25,"AdministradorID":99}`))
	peticion = conRol(t, peticion, sesion.RolAdministrador)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusCreated {
		t.Fatalf("se esperaba 201 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if repo.creada == nil {
		t.Fatal("no se guardó la cancha")
	}
	if repo.creada.AdministradorID != 1 {
		t.Fatalf("la cancha quedó a nombre del administrador %d y debía ser el 1", repo.creada.AdministradorID)
	}
	if !repo.creada.Activa {
		t.Fatal("la cancha debía crearse activa")
	}
}

// TestClienteSoloVeCanchasActivas cubre el catálogo del cliente.
func TestClienteSoloVeCanchasActivas(t *testing.T) {
	repo := &repoCanchasFalso{
		canchas: []Cancha{
			{ID: 1, Nombre: "Activa 1", Activa: true, AdministradorID: 1},
			{ID: 2, Nombre: "Activa 2", Activa: true, AdministradorID: 2},
			{ID: 3, Nombre: "Inactiva", Activa: false, AdministradorID: 1},
		},
	}
	enrutador := enrutadorCon(repo, vinculosFalsos{
		usuario: usuarios.Usuario{ID: 2, Rol: sesion.RolCliente},
	})

	peticion := httptest.NewRequest(http.MethodGet, "/canchas", nil)
	peticion = conRol(t, peticion, sesion.RolCliente)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if repo.filtro.AdministradorID != nil {
		t.Fatal("el cliente no debía fijar un administrador")
	}
	if repo.filtro.Activa == nil || !*repo.filtro.Activa {
		t.Fatal("el cliente debía filtrar solo canchas activas")
	}
}

// TestAdministradorSoloVeSusCanchas cubre el catálogo del administrador.
func TestAdministradorSoloVeSusCanchas(t *testing.T) {
	repo := &repoCanchasFalso{
		canchas: []Cancha{
			{ID: 1, Nombre: "Propia activa", Activa: true, AdministradorID: 1},
			{ID: 2, Nombre: "Propia inactiva", Activa: false, AdministradorID: 1},
			{ID: 3, Nombre: "Ajena", Activa: true, AdministradorID: 9},
		},
	}
	enrutador := enrutadorCon(repo, vinculosFalsos{
		usuario: usuarios.Usuario{ID: 1, Rol: sesion.RolAdministrador},
	})

	peticion := httptest.NewRequest(http.MethodGet, "/canchas?administrador_id=9", nil)
	peticion = conRol(t, peticion, sesion.RolAdministrador)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if repo.filtro.AdministradorID == nil || *repo.filtro.AdministradorID != 1 {
		t.Fatalf("el filtro debía ser el administrador 1 y fue %+v", repo.filtro.AdministradorID)
	}
	if repo.filtro.Activa != nil {
		t.Fatal("el administrador debía ver activas e inactivas")
	}
}

// TestAdministradorNoEditaCanchaAjena cubre el caso 3 para canchas.
func TestAdministradorNoEditaCanchaAjena(t *testing.T) {
	repo := &repoCanchasFalso{
		canchas: []Cancha{{ID: 1, Nombre: "Ajena", Activa: true, AdministradorID: 9}},
	}
	enrutador := enrutadorCon(repo, vinculosFalsos{
		usuario: usuarios.Usuario{ID: 1, Rol: sesion.RolAdministrador},
	})

	peticion := httptest.NewRequest(http.MethodPatch, "/canchas/1",
		strings.NewReader(`{"Nombre":"Cambiada"}`))
	peticion = conRol(t, peticion, sesion.RolAdministrador)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusForbidden {
		t.Fatalf("se esperaba 403 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if repo.guardada != nil {
		t.Fatal("no se debió guardar una cancha ajena")
	}
}

// TestAdministradorEditaSuCancha cubre el caso 4 para canchas.
func TestAdministradorEditaSuCancha(t *testing.T) {
	repo := &repoCanchasFalso{
		canchas: []Cancha{{ID: 1, Nombre: "Propia", Activa: true, AdministradorID: 1}},
	}
	enrutador := enrutadorCon(repo, vinculosFalsos{
		usuario: usuarios.Usuario{ID: 1, Rol: sesion.RolAdministrador},
	})

	peticion := httptest.NewRequest(http.MethodPatch, "/canchas/1",
		strings.NewReader(`{"Nombre":"Cambiada"}`))
	peticion = conRol(t, peticion, sesion.RolAdministrador)
	grabadora := httptest.NewRecorder()
	enrutador.ServeHTTP(grabadora, peticion)

	if grabadora.Code != http.StatusOK {
		t.Fatalf("se esperaba 200 y llegó %d con cuerpo %s",
			grabadora.Code, grabadora.Body.String())
	}
	if repo.guardada == nil || repo.guardada.Nombre != "Cambiada" {
		t.Fatalf("la cancha no quedó actualizada: %+v", repo.guardada)
	}
}

// TestCuentaSinVinculoNoListaCanchas cubre el caso 5 para canchas.
func TestCuentaSinVinculoNoListaCanchas(t *testing.T) {
	repo := &repoCanchasFalso{}
	enrutador := enrutadorCon(repo, vinculosFalsos{err: sesion.ErrSinVinculo})

	peticion := httptest.NewRequest(http.MethodGet, "/canchas", nil)
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
}

// TestCuentaSinVinculoNoCreaNiEditaCanchas cubre el caso 5 en las
// operaciones de escritura del administrador.
func TestCuentaSinVinculoNoCreaNiEditaCanchas(t *testing.T) {
	casos := []struct {
		nombre string
		metodo string
		ruta   string
		cuerpo string
	}{
		{
			nombre: "crear cancha",
			metodo: http.MethodPost,
			ruta:   "/canchas",
			cuerpo: `{"Nombre":"Central","Tipo":"futbol_5","Direccion":"Calle 1","PrecioHora":25}`,
		},
		{
			nombre: "editar cancha",
			metodo: http.MethodPatch,
			ruta:   "/canchas/1",
			cuerpo: `{"Nombre":"Cambiada"}`,
		},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			repo := &repoCanchasFalso{}
			enrutador := enrutadorCon(repo, vinculosFalsos{err: sesion.ErrSinVinculo})

			peticion := httptest.NewRequest(caso.metodo, caso.ruta,
				strings.NewReader(caso.cuerpo))
			peticion = conRol(t, peticion, sesion.RolAdministrador)
			grabadora := httptest.NewRecorder()
			enrutador.ServeHTTP(grabadora, peticion)

			if grabadora.Code != http.StatusForbidden {
				t.Fatalf("se esperaba 403 y llegó %d con cuerpo %s",
					grabadora.Code, grabadora.Body.String())
			}
			if !strings.Contains(grabadora.Body.String(), "cuenta_sin_vinculo") {
				t.Fatalf("falta el código cuenta_sin_vinculo: %s", grabadora.Body.String())
			}
			if repo.creada != nil || repo.guardada != nil {
				t.Fatal("no se debió guardar nada sin vínculo")
			}
		})
	}
}
