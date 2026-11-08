// Package sagas mantiene la proyección de las sagas y expone su estado por
// HTTP. No decide pasos: solo observa eventos y registra el avance.
package sagas

import (
	"fmt"
	"time"
)

const (
	TipoCicloReserva = "ciclo_reserva"
)

const (
	EstadoIniciada    = "iniciada"
	EstadoEnProceso   = "en_proceso"
	EstadoCompletada  = "completada"
	EstadoCompensando = "compensando"
	EstadoCompensada  = "compensada"
	EstadoFallida     = "fallida"
	EstadoCancelada   = "cancelada"
)

// Saga es la proyección de una transacción de negocio distribuida.
type Saga struct {
	ID             string    `gorm:"primaryKey;size:64" json:"id"`
	Tipo           string    `gorm:"size:64;index;not null" json:"tipo"`
	AgregadoID     uint      `gorm:"index;not null" json:"agregado_id"`
	Estado         string    `gorm:"size:32;index;not null" json:"estado"`
	Paso           string    `gorm:"size:64" json:"paso"`
	UltimoEventoID string    `gorm:"size:64" json:"ultimo_evento_id"`
	UltimoEventoEn time.Time `json:"ultimo_evento_en"`
	Error          string    `gorm:"type:text" json:"error,omitempty"`
	RecursoID      uint      `json:"recurso_id"`
	CreadaEn       time.Time `gorm:"not null" json:"creada_en"`
	ActualizadaEn  time.Time `gorm:"not null" json:"actualizada_en"`
}

// TableName fija el nombre de la tabla de sagas.
func (Saga) TableName() string { return "sagas" }

// IDDeReserva genera un identificador estable para la saga de una reserva.
// Al ser determinista, el productor y la proyección no compiten por crearla.
func IDDeReserva(reservaID uint) string {
	return fmt.Sprintf("reserva-%d", reservaID)
}
