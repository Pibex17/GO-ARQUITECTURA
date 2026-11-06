package inscripcion

import (
	"errors"
	"testing"
)

func TestServicioInscripcionesValidaCodigoPartidoYCupo(t *testing.T) {
	valido := Partido{ID: 1, CodigoInvitacion: "ABC123", CupoMaximo: 10}

	casos := []struct {
		nombre   string
		repo     *repoInscripcionFalso
		entrada  EntradaCrear
		esperado error
	}{
		{
			nombre:   "código obligatorio",
			repo:     &repoInscripcionFalso{partido: valido},
			entrada:  EntradaCrear{},
			esperado: ErrDatosInvalidos,
		},
		{
			nombre:   "partido inexistente",
			repo:     &repoInscripcionFalso{},
			entrada:  EntradaCrear{CodigoInvitacion: "ABC123"},
			esperado: ErrNoEncontrado,
		},
		{
			nombre:   "código incorrecto",
			repo:     &repoInscripcionFalso{partido: valido},
			entrada:  EntradaCrear{CodigoInvitacion: "OTRO"},
			esperado: ErrCodigoInvalido,
		},
		{
			nombre: "ya inscripto",
			repo: &repoInscripcionFalso{
				partido:   valido,
				existente: &Inscripcion{ID: 5, PartidoID: 1, JugadorID: 2},
			},
			entrada:  EntradaCrear{CodigoInvitacion: "ABC123"},
			esperado: ErrYaInscripto,
		},
		{
			nombre: "partido lleno",
			repo: &repoInscripcionFalso{
				partido: Partido{ID: 1, CodigoInvitacion: "ABC123", CupoMaximo: 2},
				total:   2,
			},
			entrada:  EntradaCrear{CodigoInvitacion: "ABC123"},
			esperado: ErrPartidoLleno,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			_, err := NuevoServicio(caso.repo).Crear(1, 2, caso.entrada)
			if !errors.Is(err, caso.esperado) {
				t.Fatalf("se esperaba %v y llegó %v", caso.esperado, err)
			}
			if caso.repo.creada != nil {
				t.Fatal("no se debió guardar una inscripción inválida")
			}
		})
	}
}

func TestServicioInscripcionesCreaLaInscripcionDelJugador(t *testing.T) {
	repo := &repoInscripcionFalso{
		partido: Partido{ID: 1, CodigoInvitacion: "ABC123", CupoMaximo: 10},
		total:   3,
	}
	inscripcion, err := NuevoServicio(repo).Crear(1, 2, EntradaCrear{CodigoInvitacion: "ABC123"})
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if inscripcion.PartidoID != 1 || inscripcion.JugadorID != 2 {
		t.Fatalf("la inscripción no quedó asociada al partido y jugador: %+v", inscripcion)
	}
	if inscripcion.CreadoEn.IsZero() {
		t.Fatal("la inscripción no tiene fecha de creación")
	}
	if repo.creada == nil {
		t.Fatal("no se guardó la inscripción")
	}
}

func TestServicioInscripcionesPropagaErroresDePersistencia(t *testing.T) {
	fallo := errors.New("fallo de base")
	valido := Partido{ID: 1, CodigoInvitacion: "ABC123", CupoMaximo: 10}

	t.Run("leyendo partido", func(t *testing.T) {
		repo := &repoInscripcionFalso{errPartido: fallo}
		_, err := NuevoServicio(repo).Crear(1, 2, EntradaCrear{CodigoInvitacion: "ABC123"})
		if !errors.Is(err, errLeyendoPartido) {
			t.Fatalf("se esperaba errLeyendoPartido y llegó %v", err)
		}
	})

	t.Run("comprobando inscripción", func(t *testing.T) {
		repo := &repoInscripcionFalso{partido: valido, errExistente: fallo}
		_, err := NuevoServicio(repo).Crear(1, 2, EntradaCrear{CodigoInvitacion: "ABC123"})
		if !errors.Is(err, errComprobandoInscripcion) {
			t.Fatalf("se esperaba errComprobandoInscripcion y llegó %v", err)
		}
	})

	t.Run("contando inscriptos", func(t *testing.T) {
		repo := &repoInscripcionFalso{partido: valido, errTotal: fallo}
		_, err := NuevoServicio(repo).Crear(1, 2, EntradaCrear{CodigoInvitacion: "ABC123"})
		if !errors.Is(err, errContandoInscripciones) {
			t.Fatalf("se esperaba errContandoInscripciones y llegó %v", err)
		}
	})

	t.Run("guardando inscripción", func(t *testing.T) {
		repo := &repoInscripcionFalso{partido: valido, errCrear: fallo}
		_, err := NuevoServicio(repo).Crear(1, 2, EntradaCrear{CodigoInvitacion: "ABC123"})
		if !errors.Is(err, errGuardandoInscripcion) {
			t.Fatalf("se esperaba errGuardandoInscripcion y llegó %v", err)
		}
	})
}
