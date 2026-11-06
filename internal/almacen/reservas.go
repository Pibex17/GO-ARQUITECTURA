// Package almacen agrupa los adaptadores de persistencia. El servicio de
// reservas depende de la interfaz reservas.Repositorio; este paquete la
// implementa con PostgreSQL/GORM y queda fuera del paquete de negocio.
package almacen

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"canchas-sinteticas/internal/canchas"
	"canchas-sinteticas/internal/reservas"
)

// Reservas implementa reservas.Repositorio con PostgreSQL/GORM.
type Reservas struct {
	DB *gorm.DB
}

// La comprobación en tiempo de compilación evita que el cableado de main.go
// falle solo cuando se ejecuta una petición.
var _ reservas.Repositorio = (*Reservas)(nil)

// NuevoReservas inyecta la conexión en el almacén de reservas.
func NuevoReservas(db *gorm.DB) *Reservas {
	return &Reservas{DB: db}
}

func (a *Reservas) CanchaPorID(id uint) (canchas.Cancha, error) {
	var cancha canchas.Cancha
	err := a.DB.First(&cancha, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return canchas.Cancha{}, canchas.ErrNoEncontrada
	}
	return cancha, err
}

// HayConflicto es una consulta de persistencia: cuenta las reservas que
// siguen en pie para la misma cancha, fecha y hora de inicio.
func (a *Reservas) HayConflicto(canchaID uint, fecha, horaInicio string) (bool, error) {
	var enConflicto int64
	err := a.DB.Model(&reservas.Reserva{}).
		Where("cancha_id = ? AND fecha = ? AND hora_inicio = ? AND estado <> ?",
			canchaID, fecha, horaInicio, "cancelada").
		Count(&enConflicto).Error
	return enConflicto > 0, err
}

func (a *Reservas) CrearReserva(reserva *reservas.Reserva) error {
	return a.DB.Debug().Create(reserva).Error
}

func (a *Reservas) ListarReservas(filtro reservas.FiltroReservas) ([]reservas.Reserva, error) {
	consulta := a.DB.Debug().Preload("Pagos")

	if filtro.Estado != "" {
		consulta = consulta.Where("estado = ?", filtro.Estado)
	}
	if filtro.PropietarioClienteID != nil {
		consulta = consulta.Where("cliente_id = ?", *filtro.PropietarioClienteID)
	}
	if filtro.AdministradorID != nil {
		canchasDelAdmin := a.DB.Model(&canchas.Cancha{}).
			Select("id").
			Where("administrador_id = ?", *filtro.AdministradorID)
		consulta = consulta.Where("cancha_id IN (?)", canchasDelAdmin)
	}
	if filtro.ClienteID != nil {
		consulta = consulta.Where("cliente_id = ?", *filtro.ClienteID)
	}

	var lista []reservas.Reserva
	err := consulta.Find(&lista).Error
	return lista, err
}

func (a *Reservas) ReservaPorID(id uint) (reservas.Reserva, error) {
	var reserva reservas.Reserva
	err := a.DB.Preload("Pagos").First(&reserva, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return reservas.Reserva{}, reservas.ErrNoEncontrada
	}
	return reserva, err
}

// AplicarCambioEstado guarda, en una sola transacción, el pago nuevo (si
// corresponde), la verificación de la seña (si corresponde) y el estado de
// la reserva. Así no queda un pago sin su reserva ni al revés.
func (a *Reservas) AplicarCambioEstado(
	reserva *reservas.Reserva, pago *reservas.Pago, verificarPago bool,
) error {
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		if pago != nil {
			if err := tx.Create(pago).Error; err != nil {
				return fmt.Errorf("%w: %v", reservas.ErrPagoNoRegistrado, err)
			}
		}
		if verificarPago {
			if err := tx.Model(&reservas.Pago{}).
				Where("reserva_id = ? AND tipo = ? AND estado = ?",
					reserva.ID, "sena", "pendiente").
				Update("estado", "verificado").Error; err != nil {
				return fmt.Errorf("%w: %v", reservas.ErrPagoNoVerificado, err)
			}
		}
		if err := tx.Save(reserva).Error; err != nil {
			return fmt.Errorf("%w: %v", reservas.ErrReservaNoGuardada, err)
		}
		return nil
	})
	return err
}
