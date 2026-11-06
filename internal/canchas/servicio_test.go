package canchas

import (
	"errors"
	"testing"
)

// TestServicioCanchasCrearValidaLasReglasDeNegocio comprueba que las
// validaciones viven en el servicio y no dependen del transporte HTTP.
func TestServicioCanchasCrearValidaLasReglasDeNegocio(t *testing.T) {
	casos := []struct {
		nombre   string
		entrada  EntradaCrear
		esperado error
	}{
		{
			nombre:   "sin nombre",
			entrada:  EntradaCrear{Direccion: "Calle 1", Tipo: "futbol_5", PrecioHora: 10},
			esperado: ErrFaltanDatos,
		},
		{
			nombre:   "sin dirección",
			entrada:  EntradaCrear{Nombre: "Central", Tipo: "futbol_5", PrecioHora: 10},
			esperado: ErrFaltanDatos,
		},
		{
			nombre:   "precio cero",
			entrada:  EntradaCrear{Nombre: "Central", Direccion: "Calle 1", Tipo: "futbol_5"},
			esperado: ErrPrecioInvalido,
		},
		{
			nombre:   "tipo inexistente",
			entrada:  EntradaCrear{Nombre: "Central", Direccion: "Calle 1", Tipo: "futbol_9", PrecioHora: 10},
			esperado: ErrTipoInvalido,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			repo := &repoCanchasFalso{}
			_, err := NuevoServicio(repo).Crear(1, caso.entrada)
			if !errors.Is(err, caso.esperado) {
				t.Fatalf("se esperaba %v y llegó %v", caso.esperado, err)
			}
			if repo.creada != nil {
				t.Fatal("no se debió guardar una cancha inválida")
			}
		})
	}
}

func TestServicioCanchasCrearAsignaDueñoYEstadoInicial(t *testing.T) {
	repo := &repoCanchasFalso{}
	cancha, err := NuevoServicio(repo).Crear(7, EntradaCrear{
		Nombre: "Central", Tipo: "futbol_7", Direccion: "Calle 1", PrecioHora: 30,
	})
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if cancha.AdministradorID != 7 || !cancha.Activa {
		t.Fatalf("la cancha no quedó activa ni con su dueño: %+v", cancha)
	}
	if repo.creada == nil || repo.creada.Nombre != "Central" {
		t.Fatalf("no se guardó la cancha esperada: %+v", repo.creada)
	}
}

func TestServicioCanchasListarAplicaLasReglasDeVisibilidad(t *testing.T) {
	repo := &repoCanchasFalso{
		canchas: []Cancha{
			{ID: 1, Activa: true, AdministradorID: 1},
			{ID: 2, Activa: false, AdministradorID: 1},
			{ID: 3, Activa: true, AdministradorID: 9},
		},
	}
	servicio := NuevoServicio(repo)

	// El cliente solo ve activas y puede filtrar por administrador.
	administradorID := uint(9)
	lista, err := servicio.Listar("cliente", 2, &administradorID)
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if len(lista) != 1 || lista[0].ID != 3 {
		t.Fatalf("el cliente debía ver solo la cancha activa del administrador 9: %+v", lista)
	}
	if repo.filtro.Activa == nil || !*repo.filtro.Activa {
		t.Fatal("el cliente debía filtrar solo canchas activas")
	}

	// El administrador solo ve las suyas, sin importar el query recibido.
	repo.filtro = FiltroCanchas{}
	lista, err = servicio.Listar("administrador", 1, &administradorID)
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if len(lista) != 2 {
		t.Fatalf("el administrador debía ver sus dos canchas: %+v", lista)
	}
	if repo.filtro.AdministradorID == nil || *repo.filtro.AdministradorID != 1 {
		t.Fatalf("el filtro debía fijarse al administrador de la sesión: %+v", repo.filtro.AdministradorID)
	}
	if repo.filtro.Activa != nil {
		t.Fatal("el administrador debía ver activas e inactivas")
	}

	if _, err := servicio.Listar("desconocido", 1, nil); !errors.Is(err, ErrRolNoPermitido) {
		t.Fatalf("se esperaba ErrRolNoPermitido y llegó %v", err)
	}
}

func TestServicioCanchasActualizarRespetaPropiedadYCamposPermitidos(t *testing.T) {
	t.Run("cancha inexistente", func(t *testing.T) {
		_, err := NuevoServicio(&repoCanchasFalso{}).Actualizar(99, 1, EntradaActualizar{})
		if !errors.Is(err, ErrNoEncontrada) {
			t.Fatalf("se esperaba ErrNoEncontrada y llegó %v", err)
		}
	})

	t.Run("cancha ajena", func(t *testing.T) {
		repo := &repoCanchasFalso{canchas: []Cancha{{ID: 1, AdministradorID: 9}}}
		_, err := NuevoServicio(repo).Actualizar(1, 1, EntradaActualizar{Nombre: "Cambiada"})
		if !errors.Is(err, ErrNoEsSuya) {
			t.Fatalf("se esperaba ErrNoEsSuya y llegó %v", err)
		}
		if repo.guardada != nil {
			t.Fatal("no se debió guardar una cancha ajena")
		}
	})

	t.Run("tipo inválido", func(t *testing.T) {
		repo := &repoCanchasFalso{canchas: []Cancha{{ID: 1, AdministradorID: 1}}}
		_, err := NuevoServicio(repo).Actualizar(1, 1, EntradaActualizar{Tipo: "futbol_9"})
		if !errors.Is(err, ErrTipoInvalido) {
			t.Fatalf("se esperaba ErrTipoInvalido y llegó %v", err)
		}
	})

	t.Run("actualización parcial", func(t *testing.T) {
		repo := &repoCanchasFalso{canchas: []Cancha{{
			ID: 1, Nombre: "Original", Direccion: "Calle 1",
			Tipo: "futbol_5", PrecioHora: 10, Activa: true, AdministradorID: 1,
		}}}
		activa := false
		actualizada, err := NuevoServicio(repo).Actualizar(1, 1, EntradaActualizar{
			Nombre: "Nueva", Activa: &activa,
		})
		if err != nil {
			t.Fatalf("no se esperaba error: %v", err)
		}
		if actualizada.Nombre != "Nueva" || actualizada.Activa {
			t.Fatalf("no se aplicaron los cambios: %+v", actualizada)
		}
		if actualizada.Direccion != "Calle 1" || actualizada.Tipo != "futbol_5" || actualizada.PrecioHora != 10 {
			t.Fatalf("se perdieron campos no enviados: %+v", actualizada)
		}
		if repo.guardada == nil {
			t.Fatal("no se guardó la cancha actualizada")
		}
	})
}

// TestServicioCanchasPropagaErroresDePersistencia comprueba que el servicio
// envuelve el error del repositorio para que el transporte responda 500.
func TestServicioCanchasPropagaErroresDePersistencia(t *testing.T) {
	fallo := errors.New("fallo de base")

	t.Run("crear", func(t *testing.T) {
		repo := &repoCanchasFalso{errCrear: fallo}
		_, err := NuevoServicio(repo).Crear(1, EntradaCrear{
			Nombre: "Central", Direccion: "Calle 1", Tipo: "futbol_5", PrecioHora: 10,
		})
		if !errors.Is(err, errCreando) {
			t.Fatalf("se esperaba errCreando y llegó %v", err)
		}
	})

	t.Run("listar", func(t *testing.T) {
		repo := &repoCanchasFalso{errListar: fallo}
		_, err := NuevoServicio(repo).Listar("cliente", 1, nil)
		if !errors.Is(err, errListando) {
			t.Fatalf("se esperaba errListando y llegó %v", err)
		}
	})

	t.Run("leer para actualizar", func(t *testing.T) {
		repo := &repoCanchasFalso{errCancha: fallo}
		_, err := NuevoServicio(repo).Actualizar(1, 1, EntradaActualizar{})
		if !errors.Is(err, errLeyendo) {
			t.Fatalf("se esperaba errLeyendo y llegó %v", err)
		}
	})

	t.Run("guardar", func(t *testing.T) {
		repo := &repoCanchasFalso{
			canchas:    []Cancha{{ID: 1, AdministradorID: 1}},
			errGuardar: fallo,
		}
		_, err := NuevoServicio(repo).Actualizar(1, 1, EntradaActualizar{Nombre: "Nueva"})
		if !errors.Is(err, errEditando) {
			t.Fatalf("se esperaba errEditando y llegó %v", err)
		}
	})
}
