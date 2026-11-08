package eventos

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"
)

type repoOutboxFalso struct {
	pendientes  []Evento
	publicados  []string
	fallidos    []string
	ultimoError error
}

func (r *repoOutboxFalso) Guardar(*gorm.DB, *Evento) error { return nil }

func (r *repoOutboxFalso) Pendientes(int) ([]Evento, error) {
	return r.pendientes, nil
}

func (r *repoOutboxFalso) MarcarPublicado(id string) error {
	r.publicados = append(r.publicados, id)
	return nil
}

func (r *repoOutboxFalso) MarcarIntentoFallido(id string, causa error) error {
	r.fallidos = append(r.fallidos, id)
	r.ultimoError = causa
	return nil
}

type busFalso struct {
	err error
}

func (b busFalso) Publicar(context.Context, Evento) error { return b.err }
func (b busFalso) Suscribir(string, Manejador)            {}

func TestDespachadorMarcaPublicadoCuandoElBusAcepta(t *testing.T) {
	repo := &repoOutboxFalso{pendientes: []Evento{
		{ID: "evt-1", Tipo: TipoReservaCreada, Estado: EstadoPendiente},
	}}
	despachador := NuevoDespachador(nil, repo, busFalso{})

	if err := despachador.DespacharPendientes(context.Background()); err != nil {
		t.Fatalf("DespacharPendientes devolvió error: %v", err)
	}
	if len(repo.publicados) != 1 || repo.publicados[0] != "evt-1" {
		t.Fatalf("no se marcó publicado: %+v", repo.publicados)
	}
	if len(repo.fallidos) != 0 {
		t.Fatalf("no debía haber fallos: %+v", repo.fallidos)
	}
}

func TestDespachadorMarcaFalloYAgotaIntentos(t *testing.T) {
	repo := &repoOutboxFalso{pendientes: []Evento{
		{ID: "evt-2", Tipo: TipoReservaCreada, Estado: EstadoPendiente, Intentos: MaxIntentos - 1},
	}}
	despachador := NuevoDespachador(nil, repo, busFalso{err: errors.New("consumidor caído")})

	agotado := false
	despachador.AlAgotarIntentos = func(_ context.Context, _ Evento, _ error) {
		agotado = true
	}

	if err := despachador.DespacharPendientes(context.Background()); err != nil {
		t.Fatalf("DespacharPendientes devolvió error: %v", err)
	}
	if len(repo.fallidos) != 1 || repo.fallidos[0] != "evt-2" {
		t.Fatalf("no se marcó el intento fallido: %+v", repo.fallidos)
	}
	if repo.ultimoError == nil || repo.ultimoError.Error() != "consumidor caído" {
		t.Fatalf("no se guardó la causa: %v", repo.ultimoError)
	}
	if !agotado {
		t.Fatal("se debió avisar que se agotaron los intentos")
	}
}
