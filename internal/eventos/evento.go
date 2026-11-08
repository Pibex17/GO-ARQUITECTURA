// Package eventos contiene el bus de eventos en proceso, el outbox
// transaccional y las garantías de idempotencia que sostienen la
// arquitectura de Saga Coreografía.
package eventos

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"
)

// Estados posibles de un evento dentro del outbox.
const (
	EstadoPendiente = "pendiente"
	EstadoPublicado = "publicado"
	EstadoFallido   = "fallido"
)

// Evento es un hecho de dominio ya ocurrido. Se guarda en la tabla
// eventos antes de publicarse para no perderlo si el proceso se cae.
type Evento struct {
	ID            string          `gorm:"primaryKey;size:64" json:"id"`
	Tipo          string          `gorm:"size:64;index;not null" json:"tipo"`
	AgregadoID    uint            `gorm:"index" json:"agregado_id"`
	SagaID        string          `gorm:"size:64;index" json:"saga_id"`
	CorrelationID string          `gorm:"size:64;index" json:"correlation_id"`
	CausaID       string          `gorm:"size:64" json:"causa_id"`
	Payload       json.RawMessage `gorm:"type:jsonb" json:"payload"`
	Estado        string          `gorm:"size:20;index;not null" json:"estado"`
	Intentos      int             `gorm:"not null;default:0" json:"intentos"`
	Error         string          `gorm:"type:text" json:"error"`
	OcurridoEn    time.Time       `gorm:"not null" json:"ocurrido_en"`
	PublicadoEn   *time.Time      `json:"publicado_en,omitempty"`
}

// TableName fija el nombre de la tabla del outbox.
func (Evento) TableName() string { return "eventos" }

// NuevoID genera un identificador opaco sin dependencias externas.
func NuevoID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// crypto/rand no debería fallar; si falla, al menos no repetimos
		// el identificador dentro del mismo proceso.
		return fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// ultimoTiempo guarda el último nanosegundo usado para que dos eventos
// creados en el mismo tick tengan un orden estable. El outbox ordena por
// ocurrido_en y la proyección descarta eventos viejos; sin esta garantía,
// dos eventos podrían compartir marca de tiempo y la proyección no sabría
// cuál es el más reciente.
var ultimoTiempo int64

// ahoraUnico devuelve una marca UTC estrictamente creciente dentro de este
// proceso, incluso si el reloj tiene poca resolución.
func ahoraUnico() time.Time {
	for {
		ahora := time.Now().UTC().UnixNano()
		ultimo := atomic.LoadInt64(&ultimoTiempo)
		if ahora <= ultimo {
			ahora = ultimo + 1
		}
		if atomic.CompareAndSwapInt64(&ultimoTiempo, ultimo, ahora) {
			return time.Unix(0, ahora).UTC()
		}
	}
}

// NuevoEvento construye un evento pendiente de publicar. El payload se
// serializa a JSON; los payloads del proyecto son structs simples, así que
// un error de serialización no puede ocurrir en la práctica.
func NuevoEvento(tipo string, agregadoID uint, sagaID, correlationID, causaID string, payload any) Evento {
	datos, _ := json.Marshal(payload)
	return Evento{
		ID:            NuevoID(),
		Tipo:          tipo,
		AgregadoID:    agregadoID,
		SagaID:        sagaID,
		CorrelationID: correlationID,
		CausaID:       causaID,
		Payload:       datos,
		Estado:        EstadoPendiente,
		OcurridoEn:    ahoraUnico(),
	}
}

// Decodificar convierte el payload de un evento al tipo concreto.
func Decodificar[T any](e Evento) (T, error) {
	var valor T
	err := json.Unmarshal(e.Payload, &valor)
	return valor, err
}

// Derivado construye el evento que produce un consumidor como consecuencia
// de otro evento. Conserva la saga y la correlación de la causa.
func Derivado(causa Evento, tipo string, agregadoID uint, payload any) Evento {
	return NuevoEvento(tipo, agregadoID, causa.SagaID, causa.CorrelationID, causa.ID, payload)
}

// DerivadoConID permite fijar un identificador determinista para que una
// compensación reintentada no se duplique.
func DerivadoConID(id string, causa Evento, tipo string, agregadoID uint, payload any) Evento {
	evento := Derivado(causa, tipo, agregadoID, payload)
	evento.ID = id
	return evento
}
