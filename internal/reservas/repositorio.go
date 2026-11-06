package reservas

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"canchas-sinteticas/internal/canchas"
)

// FiltroReservas describe qué reservas puede ver una sesión. El servicio
// decide los campos de propiedad; el repositorio solo arma la consulta.
type FiltroReservas struct {
	Estado               string
	ClienteID            *uint // filtro adicional del administrador
	PropietarioClienteID *uint // reservas de este cliente (rol cliente)
	AdministradorID      *uint // reservas de las canchas de este administrador
}

// RepositorioGORM implementa Repositorio con PostgreSQL/GORM.
type RepositorioGORM struct {
	DB *gorm.DB
}

func (r RepositorioGORM) CanchaPorID(id uint) (canchas.Cancha, error) {
	var cancha canchas.Cancha
	err := r.DB.First(&cancha, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return canchas.Cancha{}, canchas.ErrNoEncontrada
	}
	return cancha, err
}

// HayConflicto es una consulta de persistencia: cuenta las reservas que
// siguen en pie para la misma cancha, fecha y hora de inicio.
func (r RepositorioGORM) HayConflicto(canchaID uint, fecha, horaInicio string) (bool, error) {
	var enConflicto int64
	err := r.DB.Model(&Reserva{}).
		Where("cancha_id = ? AND fecha = ? AND hora_inicio = ? AND estado <> ?",
			canchaID, fecha, horaInicio, "cancelada").
		Count(&enConflicto).Error
	return enConflicto > 0, err
}

func (r RepositorioGORM) CrearReserva(reserva *Reserva) error {
	return r.DB.Debug().Create(reserva).Error
}

func (r RepositorioGORM) ListarReservas(filtro FiltroReservas) ([]Reserva, error) {
	consulta := r.DB.Debug().Preload("Pagos")

	if filtro.Estado != "" {
		consulta = consulta.Where("estado = ?", filtro.Estado)
	}
	if filtro.PropietarioClienteID != nil {
		consulta = consulta.Where("cliente_id = ?", *filtro.PropietarioClienteID)
	}
	if filtro.AdministradorID != nil {
		canchasDelAdmin := r.DB.Model(&canchas.Cancha{}).
			Select("id").
			Where("administrador_id = ?", *filtro.AdministradorID)
		consulta = consulta.Where("cancha_id IN (?)", canchasDelAdmin)
	}
	if filtro.ClienteID != nil {
		consulta = consulta.Where("cliente_id = ?", *filtro.ClienteID)
	}

	var lista []Reserva
	err := consulta.Find(&lista).Error
	return lista, err
}

func (r RepositorioGORM) ReservaPorID(id uint) (Reserva, error) {
	var reserva Reserva
	err := r.DB.Preload("Pagos").First(&reserva, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Reserva{}, ErrNoEncontrada
	}
	return reserva, err
}

// AplicarCambioEstado guarda, en una sola transacción, el pago nuevo (si
// corresponde), la verificación de la seña (si corresponde) y el estado de
// la reserva. Así no queda un pago sin su reserva ni al revés.
func (r RepositorioGORM) AplicarCambioEstado(
	reserva *Reserva, pago *Pago, verificarPago bool,
) error {
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		// Primero la reserva y después el pago: si el pago no se puede
		// registrar, la transacción revierte también el cambio de estado.
		// Omit("Pagos") evita que Save vuelva a tocar la relación: los
		// pagos se crean o verifican explícitamente aquí abajo.
		if err := tx.Omit("Pagos").Save(reserva).Error; err != nil {
			return fmt.Errorf("%w: %v", ErrReservaNoGuardada, err)
		}
		if pago != nil {
			if err := tx.Create(pago).Error; err != nil {
				return fmt.Errorf("%w: %v", ErrPagoNoRegistrado, err)
			}
		}
		if verificarPago {
			if err := tx.Model(&Pago{}).
				Where("reserva_id = ? AND tipo = ? AND estado = ?",
					reserva.ID, "sena", "pendiente").
				Update("estado", "verificado").Error; err != nil {
				return fmt.Errorf("%w: %v", ErrPagoNoVerificado, err)
			}
		}
		return nil
	})
	return err
}
