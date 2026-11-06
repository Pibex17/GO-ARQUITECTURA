package almacen

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"canchas-sinteticas/internal/almacen/filas"
	"canchas-sinteticas/internal/reservas"
)

// ReservasSqlc implementa reservas.Repositorio con sqlc para la lectura de
// una reserva y para guardar el cambio de estado. El resto de métodos se
// heredan del adaptador GORM mediante composición.
type ReservasSqlc struct {
	reservas.RepositorioGORM
	Conexion *sql.DB
	q        *filas.Queries
}

var _ reservas.Repositorio = (*ReservasSqlc)(nil)

// NuevoReservasSqlc construye el adaptador con la misma conexión que usa
// GORM. No abre una conexión nueva ni cambia la configuración de la base.
func NuevoReservasSqlc(gormRepo reservas.RepositorioGORM, conexion *sql.DB) *ReservasSqlc {
	return &ReservasSqlc{
		RepositorioGORM: gormRepo,
		Conexion:        conexion,
		q:               filas.New(conexion),
	}
}

// ReservaPorID busca la fila de reserva con sqlc. Los pagos siguen
// cargándose con GORM porque esta fase solo migra la tabla reservas.
func (a *ReservasSqlc) ReservaPorID(id uint) (reservas.Reserva, error) {
	fila, err := a.q.BuscarReserva(context.Background(), int64(id))
	if errors.Is(err, sql.ErrNoRows) {
		return reservas.Reserva{}, reservas.ErrNoEncontrada
	}
	if err != nil {
		return reservas.Reserva{}, err
	}

	reserva := reservas.Reserva{
		ID:         uint(fila.ID),
		CanchaID:   uint(fila.CanchaID),
		ClienteID:  uint(fila.ClienteID),
		Fecha:      fila.Fecha,
		HoraInicio: fila.HoraInicio,
		HoraFin:    fila.HoraFin,
		Estado:     fila.Estado,
		Pagos:      []reservas.Pago{},
	}

	// La interfaz entrega la reserva con sus pagos. Esa relación no forma
	// parte de las dos consultas de esta fase, así que se conserva con GORM.
	if a.RepositorioGORM.DB != nil {
		if err := a.RepositorioGORM.DB.
			Where("reserva_id = ?", reserva.ID).
			Find(&reserva.Pagos).Error; err != nil {
			return reservas.Reserva{}, err
		}
	}
	if reserva.Pagos == nil {
		reserva.Pagos = []reservas.Pago{}
	}

	return reserva, nil
}

// AplicarCambioEstado conserva la transacción del adaptador GORM: el pago
// nuevo, la verificación de la seña y el estado de la reserva se confirman
// juntos o se revierten juntos. El UPDATE de la reserva lo ejecuta sqlc
// dentro de esa misma transacción.
func (a *ReservasSqlc) AplicarCambioEstado(
	reserva *reservas.Reserva, pago *reservas.Pago, verificarPago bool,
) error {
	if reserva == nil {
		return reservas.ErrReservaNoGuardada
	}

	return a.RepositorioGORM.DB.Transaction(func(tx *gorm.DB) error {
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

		sqlTx, ok := tx.Statement.ConnPool.(*sql.Tx)
		if !ok {
			return fmt.Errorf("%w: no se pudo obtener la transacción SQL",
				reservas.ErrReservaNoGuardada)
		}

		guardadas, err := a.q.WithTx(sqlTx).GuardarReserva(
			context.Background(),
			filas.GuardarReservaParams{
				ID:         int64(reserva.ID),
				CanchaID:   int64(reserva.CanchaID),
				ClienteID:  int64(reserva.ClienteID),
				Fecha:      reserva.Fecha,
				HoraInicio: reserva.HoraInicio,
				HoraFin:    reserva.HoraFin,
				Estado:     reserva.Estado,
			},
		)
		if err != nil {
			return fmt.Errorf("%w: %v", reservas.ErrReservaNoGuardada, err)
		}
		if guardadas == 0 {
			return reservas.ErrNoEncontrada
		}

		return nil
	})
}
