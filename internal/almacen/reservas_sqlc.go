package almacen

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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

// AplicarCambioEstado resuelve con sqlc su propia transacción: el estado de
// la reserva y el pago nuevo o su verificación se confirman juntos o se
// revierten juntos. No usa la transacción de GORM ni mezcla herramientas.
func (a *ReservasSqlc) AplicarCambioEstado(
	reserva *reservas.Reserva, pago *reservas.Pago, verificarPago bool,
) error {
	if reserva == nil {
		return reservas.ErrReservaNoGuardada
	}
	if a.Conexion == nil {
		return fmt.Errorf("%w: no hay conexión sqlc", reservas.ErrReservaNoGuardada)
	}

	ctx := context.Background()
	tx, err := a.Conexion.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", reservas.ErrReservaNoGuardada, err)
	}
	// Si no se llega a Commit, este Rollback deja la base como estaba.
	defer func() { _ = tx.Rollback() }()

	q := a.q.WithTx(tx)

	// Primero la reserva y después el pago: si la restricción del pago
	// rechaza la fila, también se revierte el cambio de estado.
	guardadas, err := q.GuardarReserva(ctx, filas.GuardarReservaParams{
		ID:         int64(reserva.ID),
		CanchaID:   int64(reserva.CanchaID),
		ClienteID:  int64(reserva.ClienteID),
		Fecha:      reserva.Fecha,
		HoraInicio: reserva.HoraInicio,
		HoraFin:    reserva.HoraFin,
		Estado:     reserva.Estado,
	})
	if err != nil {
		return fmt.Errorf("%w: %v", reservas.ErrReservaNoGuardada, err)
	}
	if guardadas == 0 {
		return reservas.ErrNoEncontrada
	}

	if pago != nil {
		id, err := q.CrearPago(ctx, filas.CrearPagoParams{
			ReservaID:      int64(pago.ReservaID),
			Tipo:           pago.Tipo,
			Monto:          pago.Monto,
			Metodo:         pago.Metodo,
			ComprobanteUrl: pago.ComprobanteURL,
			Estado:         pago.Estado,
		})
		if err != nil {
			return fmt.Errorf("%w: %v", reservas.ErrPagoNoRegistrado, err)
		}
		pago.ID = uint(id)
	}

	if verificarPago {
		if err := q.VerificarPago(ctx, int64(reserva.ID)); err != nil {
			return fmt.Errorf("%w: %v", reservas.ErrPagoNoVerificado, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: %v", reservas.ErrReservaNoGuardada, err)
	}
	return nil
}
