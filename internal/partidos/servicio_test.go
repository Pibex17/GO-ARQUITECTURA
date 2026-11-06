package partidos

import (
	"errors"
	"strings"
	"testing"

	"canchas-sinteticas/internal/reservas"
)

func TestServicioPartidosCrearValidaLosDatosDeNegocio(t *testing.T) {
	casos := []struct {
		nombre  string
		entrada EntradaCrear
	}{
		{"sin reserva", EntradaCrear{Nombre: "Amistoso", CupoMaximo: 10}},
		{"sin nombre", EntradaCrear{ReservaID: 1, CupoMaximo: 10}},
		{"cupo cero", EntradaCrear{ReservaID: 1, Nombre: "Amistoso"}},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			repo := &repoPartidosFalso{}
			_, err := NuevoServicio(repo).Crear(2, caso.entrada)
			if !errors.Is(err, ErrDatosInvalidos) {
				t.Fatalf("se esperaba ErrDatosInvalidos y llegó %v", err)
			}
			if repo.creado != nil {
				t.Fatal("no se debió guardar un partido inválido")
			}
		})
	}
}

func TestServicioPartidosCrearExigeReservaPropiaYConfirmada(t *testing.T) {
	t.Run("reserva inexistente", func(t *testing.T) {
		repo := &repoPartidosFalso{}
		_, err := NuevoServicio(repo).Crear(2, EntradaCrear{
			ReservaID: 99, Nombre: "Amistoso", CupoMaximo: 10,
		})
		if !errors.Is(err, ErrReservaNoEncontrada) {
			t.Fatalf("se esperaba ErrReservaNoEncontrada y llegó %v", err)
		}
	})

	t.Run("reserva ajena", func(t *testing.T) {
		repo := &repoPartidosFalso{
			reserva: reservas.Reserva{ID: 1, ClienteID: 3, Estado: "confirmada"},
		}
		_, err := NuevoServicio(repo).Crear(2, EntradaCrear{
			ReservaID: 1, Nombre: "Amistoso", CupoMaximo: 10,
		})
		if !errors.Is(err, ErrReservaAjena) {
			t.Fatalf("se esperaba ErrReservaAjena y llegó %v", err)
		}
	})

	t.Run("reserva no confirmada", func(t *testing.T) {
		repo := &repoPartidosFalso{
			reserva: reservas.Reserva{ID: 1, ClienteID: 2, Estado: "pendiente"},
		}
		_, err := NuevoServicio(repo).Crear(2, EntradaCrear{
			ReservaID: 1, Nombre: "Amistoso", CupoMaximo: 10,
		})
		if !errors.Is(err, ErrReservaNoConfirmada) {
			t.Fatalf("se esperaba ErrReservaNoConfirmada y llegó %v", err)
		}
	})
}

func TestServicioPartidosCrearEvitaDuplicados(t *testing.T) {
	repo := &repoPartidosFalso{
		reserva:   reservas.Reserva{ID: 1, ClienteID: 2, Estado: "confirmada"},
		existente: &Partido{ID: 5, ReservaID: 1},
	}
	_, err := NuevoServicio(repo).Crear(2, EntradaCrear{
		ReservaID: 1, Nombre: "Amistoso", CupoMaximo: 10,
	})
	if !errors.Is(err, ErrPartidoExistente) {
		t.Fatalf("se esperaba ErrPartidoExistente y llegó %v", err)
	}
	if repo.creado != nil {
		t.Fatal("no se debió guardar un partido duplicado")
	}
}

func TestServicioPartidosCrearGeneraElCodigoDeInvitacion(t *testing.T) {
	repo := &repoPartidosFalso{
		reserva: reservas.Reserva{ID: 1, ClienteID: 2, Estado: "confirmada"},
	}
	partido, err := NuevoServicio(repo).Crear(2, EntradaCrear{
		ReservaID: 1, Nombre: "Amistoso", CupoMaximo: 10,
	})
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if len(partido.CodigoInvitacion) != 6 {
		t.Fatalf("el código debía tener 6 caracteres y tiene %d: %q",
			len(partido.CodigoInvitacion), partido.CodigoInvitacion)
	}
	const permitidos = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	for _, caracter := range partido.CodigoInvitacion {
		if !strings.ContainsRune(permitidos, caracter) {
			t.Fatalf("el código contiene un carácter no permitido: %q", partido.CodigoInvitacion)
		}
	}
	if repo.creado == nil || repo.creado.ReservaID != 1 {
		t.Fatalf("no se guardó el partido esperado: %+v", repo.creado)
	}
}

func TestServicioPartidosCrearPropagaErroresDePersistencia(t *testing.T) {
	fallo := errors.New("fallo de base")

	t.Run("leyendo reserva", func(t *testing.T) {
		repo := &repoPartidosFalso{errReserva: fallo}
		_, err := NuevoServicio(repo).Crear(2, EntradaCrear{
			ReservaID: 1, Nombre: "Amistoso", CupoMaximo: 10,
		})
		if !errors.Is(err, errLeyendoReserva) {
			t.Fatalf("se esperaba errLeyendoReserva y llegó %v", err)
		}
	})

	t.Run("comprobando partido existente", func(t *testing.T) {
		repo := &repoPartidosFalso{
			reserva:      reservas.Reserva{ID: 1, ClienteID: 2, Estado: "confirmada"},
			errExistente: fallo,
		}
		_, err := NuevoServicio(repo).Crear(2, EntradaCrear{
			ReservaID: 1, Nombre: "Amistoso", CupoMaximo: 10,
		})
		if !errors.Is(err, errComprobandoPartido) {
			t.Fatalf("se esperaba errComprobandoPartido y llegó %v", err)
		}
	})

	t.Run("generando código", func(t *testing.T) {
		original := leerAleatorio
		leerAleatorio = func([]byte) (int, error) {
			return 0, errors.New("sin entropía")
		}
		defer func() { leerAleatorio = original }()

		repo := &repoPartidosFalso{
			reserva: reservas.Reserva{ID: 1, ClienteID: 2, Estado: "confirmada"},
		}
		_, err := NuevoServicio(repo).Crear(2, EntradaCrear{
			ReservaID: 1, Nombre: "Amistoso", CupoMaximo: 10,
		})
		if !errors.Is(err, errGenerandoCodigo) {
			t.Fatalf("se esperaba errGenerandoCodigo y llegó %v", err)
		}
	})

	t.Run("guardando partido", func(t *testing.T) {
		repo := &repoPartidosFalso{
			reserva:  reservas.Reserva{ID: 1, ClienteID: 2, Estado: "confirmada"},
			errCrear: fallo,
		}
		_, err := NuevoServicio(repo).Crear(2, EntradaCrear{
			ReservaID: 1, Nombre: "Amistoso", CupoMaximo: 10,
		})
		if !errors.Is(err, errGuardandoPartido) {
			t.Fatalf("se esperaba errGuardandoPartido y llegó %v", err)
		}
	})
}

func TestServicioPartidosObtenerCalculaLaOcupacion(t *testing.T) {
	repo := &repoPartidosFalso{
		partido:       &Partido{ID: 1, Nombre: "Amistoso", CupoMaximo: 10},
		inscripciones: 3,
	}
	detalle, err := NuevoServicio(repo).Obtener(1)
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if detalle.Inscriptos != 3 || detalle.CuposRestantes != 7 {
		t.Fatalf("la ocupación no coincide: %+v", detalle)
	}
}

func TestServicioPartidosObtenerDistingueNoEncontradoDeFalloInterno(t *testing.T) {
	t.Run("no encontrado", func(t *testing.T) {
		_, err := NuevoServicio(&repoPartidosFalso{}).Obtener(99)
		if !errors.Is(err, ErrNoEncontrado) {
			t.Fatalf("se esperaba ErrNoEncontrado y llegó %v", err)
		}
	})

	t.Run("error leyendo partido", func(t *testing.T) {
		repo := &repoPartidosFalso{errPartido: errors.New("fallo de base")}
		_, err := NuevoServicio(repo).Obtener(1)
		if !errors.Is(err, errLeyendoPartido) || errors.Is(err, ErrNoEncontrado) {
			t.Fatalf("se esperaba errLeyendoPartido y llegó %v", err)
		}
	})

	// Este es el caso que antes se convertía indebidamente en 404.
	t.Run("error contando inscriptos", func(t *testing.T) {
		repo := &repoPartidosFalso{
			partido:          &Partido{ID: 1, CupoMaximo: 10},
			errInscripciones: errors.New("fallo de base"),
		}
		_, err := NuevoServicio(repo).Obtener(1)
		if !errors.Is(err, errContandoInscripcion) || errors.Is(err, ErrNoEncontrado) {
			t.Fatalf("se esperaba errContandoInscripcion y llegó %v", err)
		}
	})
}
