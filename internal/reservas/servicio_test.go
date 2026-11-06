package reservas

import (
	"errors"
	"math"
	"testing"

	"canchas-sinteticas/internal/canchas"
)

func TestServicioReservasCrearValidaCanchaYHorario(t *testing.T) {
	t.Run("faltan datos obligatorios", func(t *testing.T) {
		_, err := NuevoServicio(&repoReservasFalso{}).Crear(2, EntradaCrear{CanchaID: 1})
		if !errors.Is(err, ErrDatosInvalidos) {
			t.Fatalf("se esperaba ErrDatosInvalidos y llegó %v", err)
		}
	})

	t.Run("cancha inexistente", func(t *testing.T) {
		_, err := NuevoServicio(&repoReservasFalso{}).Crear(2, EntradaCrear{
			CanchaID: 99, Fecha: "2026-10-21", HoraInicio: "18:00", HoraFin: "19:00",
		})
		if !errors.Is(err, ErrCanchaInvalida) {
			t.Fatalf("se esperaba ErrCanchaInvalida y llegó %v", err)
		}
	})

	t.Run("cancha inactiva", func(t *testing.T) {
		repo := &repoReservasFalso{canchas: map[uint]canchas.Cancha{
			1: {ID: 1, Activa: false},
		}}
		_, err := NuevoServicio(repo).Crear(2, EntradaCrear{
			CanchaID: 1, Fecha: "2026-10-21", HoraInicio: "18:00", HoraFin: "19:00",
		})
		if !errors.Is(err, ErrCanchaInvalida) {
			t.Fatalf("se esperaba ErrCanchaInvalida y llegó %v", err)
		}
	})

	t.Run("horario ocupado", func(t *testing.T) {
		repo := &repoReservasFalso{
			canchas:   map[uint]canchas.Cancha{1: {ID: 1, Activa: true}},
			conflicto: true,
		}
		_, err := NuevoServicio(repo).Crear(2, EntradaCrear{
			CanchaID: 1, Fecha: "2026-10-21", HoraInicio: "18:00", HoraFin: "19:00",
		})
		if !errors.Is(err, ErrHorarioOcupado) {
			t.Fatalf("se esperaba ErrHorarioOcupado y llegó %v", err)
		}
		if repo.creada != nil {
			t.Fatal("no se debió guardar una reserva en un horario ocupado")
		}
	})
}

func TestServicioReservasCrearAsignaEstadoYDueño(t *testing.T) {
	repo := &repoReservasFalso{canchas: map[uint]canchas.Cancha{
		1: {ID: 1, Activa: true, PrecioHora: 25},
	}}
	reserva, err := NuevoServicio(repo).Crear(2, EntradaCrear{
		CanchaID: 1, Fecha: "2026-10-21", HoraInicio: "18:00", HoraFin: "19:00",
	})
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if reserva.Estado != "pendiente" || reserva.ClienteID != 2 {
		t.Fatalf("la reserva no quedó pendiente ni con su dueño: %+v", reserva)
	}
	if repo.creada == nil || repo.creada.HoraInicio != "18:00" {
		t.Fatalf("no se guardó la reserva esperada: %+v", repo.creada)
	}
}

func TestServicioReservasPropagaErroresAlCrear(t *testing.T) {
	fallo := errors.New("fallo de base")

	t.Run("leyendo cancha", func(t *testing.T) {
		repo := &repoReservasFalso{errCancha: fallo}
		_, err := NuevoServicio(repo).Crear(2, EntradaCrear{
			CanchaID: 1, Fecha: "2026-10-21", HoraInicio: "18:00", HoraFin: "19:00",
		})
		if !errors.Is(err, errLeyendoCancha) {
			t.Fatalf("se esperaba errLeyendoCancha y llegó %v", err)
		}
	})

	t.Run("comprobando horario", func(t *testing.T) {
		repo := &repoReservasFalso{
			canchas:      map[uint]canchas.Cancha{1: {ID: 1, Activa: true}},
			errConflicto: fallo,
		}
		_, err := NuevoServicio(repo).Crear(2, EntradaCrear{
			CanchaID: 1, Fecha: "2026-10-21", HoraInicio: "18:00", HoraFin: "19:00",
		})
		if !errors.Is(err, errComprobandoHorario) {
			t.Fatalf("se esperaba errComprobandoHorario y llegó %v", err)
		}
	})

	t.Run("guardando reserva", func(t *testing.T) {
		repo := &repoReservasFalso{
			canchas:  map[uint]canchas.Cancha{1: {ID: 1, Activa: true}},
			errCrear: fallo,
		}
		_, err := NuevoServicio(repo).Crear(2, EntradaCrear{
			CanchaID: 1, Fecha: "2026-10-21", HoraInicio: "18:00", HoraFin: "19:00",
		})
		if !errors.Is(err, errGuardandoReserva) {
			t.Fatalf("se esperaba errGuardandoReserva y llegó %v", err)
		}
	})
}

func TestServicioReservasListarAplicaLasReglasDeVisibilidad(t *testing.T) {
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
	servicio := NuevoServicio(repo)

	lista, err := servicio.Listar("cliente", 2, "", nil)
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if len(lista) != 1 || lista[0].ID != 1 {
		t.Fatalf("el cliente debía ver solo su reserva: %+v", lista)
	}
	if repo.filtro.PropietarioClienteID == nil || *repo.filtro.PropietarioClienteID != 2 {
		t.Fatalf("el filtro de propietario debía ser 2: %+v", repo.filtro.PropietarioClienteID)
	}

	clienteID := uint(3)
	lista, err = servicio.Listar("administrador", 1, "", &clienteID)
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if len(lista) != 1 || lista[0].ID != 3 {
		t.Fatalf("el administrador debía ver solo la reserva 3: %+v", lista)
	}
	if repo.filtro.AdministradorID == nil || *repo.filtro.AdministradorID != 1 {
		t.Fatalf("el filtro de administrador debía ser 1: %+v", repo.filtro.AdministradorID)
	}
	if repo.filtro.ClienteID == nil || *repo.filtro.ClienteID != 3 {
		t.Fatalf("el filtro de cliente debía ser 3: %+v", repo.filtro.ClienteID)
	}

	if _, err := servicio.Listar("desconocido", 1, "", nil); !errors.Is(err, ErrRolNoPermitido) {
		t.Fatalf("se esperaba ErrRolNoPermitido y llegó %v", err)
	}
}

func TestServicioReservasListarPropagaErrorDePersistencia(t *testing.T) {
	repo := &repoReservasFalso{errListar: errors.New("fallo de base")}
	_, err := NuevoServicio(repo).Listar("cliente", 2, "", nil)
	if !errors.Is(err, errListandoReservas) {
		t.Fatalf("se esperaba errListandoReservas y llegó %v", err)
	}
}

func TestServicioReservasObtenerRespetaPropiedad(t *testing.T) {
	casos := []struct {
		nombre   string
		rol      string
		usuario  uint
		canchas  map[uint]canchas.Cancha
		reservas []Reserva
		esperado error
	}{
		{
			nombre:   "cliente dueño",
			rol:      "cliente",
			usuario:  2,
			canchas:  map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 1}},
			reservas: []Reserva{{ID: 1, CanchaID: 1, ClienteID: 2}},
		},
		{
			nombre:   "cliente ajeno",
			rol:      "cliente",
			usuario:  2,
			canchas:  map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 1}},
			reservas: []Reserva{{ID: 1, CanchaID: 1, ClienteID: 3}},
			esperado: ErrPermisoDenegado,
		},
		{
			nombre:   "administrador de la cancha",
			rol:      "administrador",
			usuario:  1,
			canchas:  map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 1}},
			reservas: []Reserva{{ID: 1, CanchaID: 1, ClienteID: 2}},
		},
		{
			nombre:   "administrador de otra cancha",
			rol:      "administrador",
			usuario:  1,
			canchas:  map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 9}},
			reservas: []Reserva{{ID: 1, CanchaID: 1, ClienteID: 2}},
			esperado: ErrPermisoDenegado,
		},
		{
			nombre:   "reserva inexistente",
			rol:      "cliente",
			usuario:  2,
			esperado: ErrNoEncontrada,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			repo := &repoReservasFalso{canchas: caso.canchas, reservas: caso.reservas}
			_, err := NuevoServicio(repo).Obtener(1, caso.rol, caso.usuario)
			if caso.esperado == nil {
				if err != nil {
					t.Fatalf("no se esperaba error: %v", err)
				}
				return
			}
			if !errors.Is(err, caso.esperado) {
				t.Fatalf("se esperaba %v y llegó %v", caso.esperado, err)
			}
		})
	}
}

func TestServicioReservasObtenerPropagaErroresDePersistencia(t *testing.T) {
	t.Run("leyendo reserva", func(t *testing.T) {
		repo := &repoReservasFalso{errReserva: errors.New("fallo de base")}
		_, err := NuevoServicio(repo).Obtener(1, "cliente", 2)
		if !errors.Is(err, errLeyendoReserva) {
			t.Fatalf("se esperaba errLeyendoReserva y llegó %v", err)
		}
	})

	t.Run("comprobando propiedad", func(t *testing.T) {
		repo := &repoReservasFalso{
			canchas:   nil,
			reservas:  []Reserva{{ID: 1, CanchaID: 1, ClienteID: 2}},
			errCancha: errors.New("fallo de base"),
		}
		_, err := NuevoServicio(repo).Obtener(1, "administrador", 1)
		if !errors.Is(err, errComprobandoPropiedad) {
			t.Fatalf("se esperaba errComprobandoPropiedad y llegó %v", err)
		}
	})
}

func TestServicioReservasCambiarEstadoValidaEstadoYPropiedad(t *testing.T) {
	t.Run("estado inexistente", func(t *testing.T) {
		repo := &repoReservasFalso{}
		_, err := NuevoServicio(repo).CambiarEstado(1, 1, "volando", "", "")
		if !errors.Is(err, ErrEstadoInvalido) {
			t.Fatalf("se esperaba ErrEstadoInvalido y llegó %v", err)
		}
	})

	t.Run("reserva inexistente", func(t *testing.T) {
		repo := &repoReservasFalso{}
		_, err := NuevoServicio(repo).CambiarEstado(1, 1, "cancelada", "", "")
		if !errors.Is(err, ErrNoEncontrada) {
			t.Fatalf("se esperaba ErrNoEncontrada y llegó %v", err)
		}
	})

	t.Run("cancha inexistente", func(t *testing.T) {
		repo := &repoReservasFalso{
			reservas: []Reserva{{ID: 1, CanchaID: 99, Estado: "pendiente"}},
		}
		_, err := NuevoServicio(repo).CambiarEstado(1, 1, "cancelada", "", "")
		if !errors.Is(err, ErrPermisoDenegado) {
			t.Fatalf("se esperaba ErrPermisoDenegado y llegó %v", err)
		}
	})

	t.Run("cancha de otro administrador", func(t *testing.T) {
		repo := &repoReservasFalso{
			canchas:  map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 9}},
			reservas: []Reserva{{ID: 1, CanchaID: 1, Estado: "pendiente"}},
		}
		_, err := NuevoServicio(repo).CambiarEstado(1, 1, "cancelada", "", "")
		if !errors.Is(err, ErrPermisoDenegado) {
			t.Fatalf("se esperaba ErrPermisoDenegado y llegó %v", err)
		}
		if repo.guardada != nil {
			t.Fatal("no se debió guardar un cambio sobre una cancha ajena")
		}
	})

	t.Run("transición inválida", func(t *testing.T) {
		repo := &repoReservasFalso{
			canchas:  map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 1}},
			reservas: []Reserva{{ID: 1, CanchaID: 1, Estado: "jugada"}},
		}
		_, err := NuevoServicio(repo).CambiarEstado(1, 1, "cancelada", "", "")
		var transicion ErrorTransicion
		if !errors.As(err, &transicion) {
			t.Fatalf("se esperaba ErrorTransicion y llegó %v", err)
		}
		if transicion.De != "jugada" || transicion.A != "cancelada" {
			t.Fatalf("la transición reportada no coincide: %+v", transicion)
		}
		if repo.guardada != nil {
			t.Fatal("no se debió guardar una transición inválida")
		}
	})
}

func TestServicioReservasCambiarEstadoCreaLaSenaEnLaTransicion(t *testing.T) {
	repo := &repoReservasFalso{
		canchas:  map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 1, PrecioHora: 100}},
		reservas: []Reserva{{ID: 1, CanchaID: 1, ClienteID: 2, Estado: "pendiente"}},
	}
	actualizada, err := NuevoServicio(repo).CambiarEstado(
		1, 1, "pago_por_verificar", "transferencia", "https://ejemplo.invalid/comprobante.jpg")
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if actualizada.Estado != "pago_por_verificar" {
		t.Fatalf("la reserva no cambió de estado: %+v", actualizada)
	}
	if repo.pagoCreado == nil {
		t.Fatal("no se creó el pago de la seña")
	}
	if repo.pagoCreado.Tipo != "sena" || repo.pagoCreado.Estado != "pendiente" {
		t.Fatalf("el pago creado no es una seña pendiente: %+v", repo.pagoCreado)
	}
	if math.Abs(repo.pagoCreado.Monto-30) > 1e-9 {
		t.Fatalf("la seña debía ser el 30 %% de 100 y fue %v", repo.pagoCreado.Monto)
	}
	if repo.pagoCreado.Metodo != "transferencia" || repo.pagoCreado.ComprobanteURL == "" {
		t.Fatalf("no se copiaron los datos del pago: %+v", repo.pagoCreado)
	}
	if repo.guardada == nil || repo.guardada.Estado != "pago_por_verificar" {
		t.Fatalf("no se guardó la reserva actualizada: %+v", repo.guardada)
	}
}

func TestServicioReservasCambiarEstadoVerificaLaSenaAlConfirmar(t *testing.T) {
	repo := &repoReservasFalso{
		canchas:  map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 1}},
		reservas: []Reserva{{ID: 1, CanchaID: 1, Estado: "pago_por_verificar"}},
	}
	_, err := NuevoServicio(repo).CambiarEstado(1, 1, "confirmada", "", "")
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if !repo.pagoVerificado {
		t.Fatal("no se verificó la seña al confirmar")
	}
	if repo.pagoCreado != nil {
		t.Fatal("confirmar no debe crear un pago nuevo")
	}
}

func TestServicioReservasCambiarEstadoSinPagoEnTransicionesSinEfecto(t *testing.T) {
	repo := &repoReservasFalso{
		canchas:  map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 1}},
		reservas: []Reserva{{ID: 1, CanchaID: 1, Estado: "confirmada"}},
	}
	_, err := NuevoServicio(repo).CambiarEstado(1, 1, "jugada", "", "")
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if repo.pagoCreado != nil || repo.pagoVerificado {
		t.Fatal("la transición a jugada no debe crear ni verificar pagos")
	}
}

func TestServicioReservasCambiarEstadoPropagaErroresDePersistencia(t *testing.T) {
	base := func() *repoReservasFalso {
		return &repoReservasFalso{
			canchas:  map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 1, PrecioHora: 100}},
			reservas: []Reserva{{ID: 1, CanchaID: 1, Estado: "pendiente"}},
		}
	}

	t.Run("registrando pago", func(t *testing.T) {
		repo := base()
		repo.errPago = ErrPagoNoRegistrado
		_, err := NuevoServicio(repo).CambiarEstado(1, 1, "pago_por_verificar", "efectivo", "")
		if !errors.Is(err, ErrPagoNoRegistrado) {
			t.Fatalf("se esperaba ErrPagoNoRegistrado y llegó %v", err)
		}
	})

	t.Run("verificando pago", func(t *testing.T) {
		repo := &repoReservasFalso{
			canchas:      map[uint]canchas.Cancha{1: {ID: 1, AdministradorID: 1}},
			reservas:     []Reserva{{ID: 1, CanchaID: 1, Estado: "pago_por_verificar"}},
			errVerificar: ErrPagoNoVerificado,
		}
		_, err := NuevoServicio(repo).CambiarEstado(1, 1, "confirmada", "", "")
		if !errors.Is(err, ErrPagoNoVerificado) {
			t.Fatalf("se esperaba ErrPagoNoVerificado y llegó %v", err)
		}
	})

	t.Run("guardando reserva", func(t *testing.T) {
		repo := base()
		repo.errGuardar = ErrReservaNoGuardada
		_, err := NuevoServicio(repo).CambiarEstado(1, 1, "cancelada", "", "")
		if !errors.Is(err, ErrReservaNoGuardada) {
			t.Fatalf("se esperaba ErrReservaNoGuardada y llegó %v", err)
		}
	})

	t.Run("releyendo reserva actualizada", func(t *testing.T) {
		repo := &repoReservasFallaAlReleer{repoReservasFalso: base()}
		_, err := NuevoServicio(repo).CambiarEstado(1, 1, "cancelada", "", "")
		if !errors.Is(err, errLeyendoActualizada) {
			t.Fatalf("se esperaba errLeyendoActualizada y llegó %v", err)
		}
	})
}

// repoReservasFallaAlReleer falla solo en la segunda lectura, que es la que
// hace el servicio después de aplicar el cambio en la transacción.
type repoReservasFallaAlReleer struct {
	*repoReservasFalso
	llamadas int
}

func (r *repoReservasFallaAlReleer) ReservaPorID(id uint) (Reserva, error) {
	r.llamadas++
	if r.llamadas == 2 {
		return Reserva{}, errors.New("fallo al releer")
	}
	return r.repoReservasFalso.ReservaPorID(id)
}
