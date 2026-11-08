package eventos

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Manejador procesa un evento. Si devuelve error, el despachador vuelve a
// intentar la publicación y los consumidores idempotentes ignoran lo ya
// procesado.
type Manejador func(context.Context, Evento) error

// Bus entrega eventos a los suscriptores del tipo correspondiente.
// La implementación en memoria es síncrona: el despachador la invoca desde
// su propia goroutine, de modo que la petición HTTP no espera a la saga.
type Bus interface {
	Publicar(ctx context.Context, evento Evento) error
	Suscribir(tipo string, manejador Manejador)
}

// BusEnMemoria es la implementación por defecto. El tipo "*" recibe todos
// los eventos y se usa para la proyección de sagas.
type BusEnMemoria struct {
	mu           sync.RWMutex
	suscriptores map[string][]Manejador
}

// NuevoBusEnMemoria crea un bus vacío.
func NuevoBusEnMemoria() *BusEnMemoria {
	return &BusEnMemoria{suscriptores: make(map[string][]Manejador)}
}

// Suscribir registra un manejador para un tipo de evento.
func (b *BusEnMemoria) Suscribir(tipo string, manejador Manejador) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.suscriptores[tipo] = append(b.suscriptores[tipo], manejador)
}

// Publicar entrega el evento a todos los suscriptores del tipo y a los
// suscriptores globales. Un pánico en un consumidor no tumba el proceso:
// se convierte en error para que el outbox reintente.
func (b *BusEnMemoria) Publicar(ctx context.Context, evento Evento) error {
	b.mu.RLock()
	manejadores := append([]Manejador{}, b.suscriptores[evento.Tipo]...)
	manejadores = append(manejadores, b.suscriptores["*"]...)
	b.mu.RUnlock()

	var errores []error
	for _, manejador := range manejadores {
		if err := invocar(ctx, manejador, evento); err != nil {
			errores = append(errores, err)
		}
	}
	return errors.Join(errores...)
}

func invocar(ctx context.Context, manejador Manejador, evento Evento) (err error) {
	defer func() {
		if recuperado := recover(); recuperado != nil {
			err = fmt.Errorf("pánico al procesar %s: %v", evento.Tipo, recuperado)
		}
	}()
	return manejador(ctx, evento)
}
