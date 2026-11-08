package pagos

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repositorio es la parte de la base que necesita el contexto de pagos.
// Los métodos EnTx participan de la transacción del consumidor de eventos.
type Repositorio interface {
	CrearEnTx(tx *gorm.DB, pago *Pago) error
	SenaPorReservaEnTx(tx *gorm.DB, reservaID uint) (Pago, error)
	MarcarVerificadoEnTx(tx *gorm.DB, reservaID uint) (Pago, error)
	MarcarAnuladoEnTx(tx *gorm.DB, reservaID uint) (Pago, error)
	RevertirVerificacionEnTx(tx *gorm.DB, reservaID uint) (Pago, error)
	ListarPorReserva(reservaID uint) ([]Pago, error)
}

// RepositorioGORM implementa Repositorio con PostgreSQL.
type RepositorioGORM struct {
	DB *gorm.DB
}

func (r RepositorioGORM) CrearEnTx(tx *gorm.DB, pago *Pago) error {
	return tx.Create(pago).Error
}

func (r RepositorioGORM) SenaPorReservaEnTx(tx *gorm.DB, reservaID uint) (Pago, error) {
	var pago Pago
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("reserva_id = ? AND tipo = ?", reservaID, "sena").
		Order("id desc").
		First(&pago).Error
	return pago, err
}

func (r RepositorioGORM) MarcarVerificadoEnTx(tx *gorm.DB, reservaID uint) (Pago, error) {
	pago, err := r.SenaPorReservaEnTx(tx, reservaID)
	if err != nil {
		return Pago{}, err
	}
	if pago.Estado == EstadoVerificado {
		return pago, nil
	}
	if pago.Estado != EstadoPendiente {
		return Pago{}, fmt.Errorf("la seña está en estado %s y no se puede verificar", pago.Estado)
	}
	pago.Estado = EstadoVerificado
	return pago, tx.Save(&pago).Error
}

func (r RepositorioGORM) MarcarAnuladoEnTx(tx *gorm.DB, reservaID uint) (Pago, error) {
	pago, err := r.SenaPorReservaEnTx(tx, reservaID)
	if err != nil {
		return Pago{}, err
	}
	if pago.Estado == EstadoAnulado {
		return pago, nil
	}
	pago.Estado = EstadoAnulado
	return pago, tx.Save(&pago).Error
}

func (r RepositorioGORM) RevertirVerificacionEnTx(tx *gorm.DB, reservaID uint) (Pago, error) {
	pago, err := r.SenaPorReservaEnTx(tx, reservaID)
	if err != nil {
		return Pago{}, err
	}
	if pago.Estado == EstadoPendiente {
		return pago, nil
	}
	if pago.Estado != EstadoVerificado {
		return Pago{}, fmt.Errorf("la seña está en estado %s y no se puede revertir", pago.Estado)
	}
	pago.Estado = EstadoPendiente
	return pago, tx.Save(&pago).Error
}

func (r RepositorioGORM) ListarPorReserva(reservaID uint) ([]Pago, error) {
	var lista []Pago
	err := r.DB.Where("reserva_id = ?", reservaID).Order("id asc").Find(&lista).Error
	return lista, err
}

// Sembrar crea la seña verificada de la reserva confirmada de ejemplo.
func Sembrar(db *gorm.DB, reservaID uint) {
	if reservaID == 0 {
		return
	}
	var total int64
	db.Model(&Pago{}).Where("reserva_id = ?", reservaID).Count(&total)
	if total > 0 {
		return
	}
	db.Create(&Pago{
		ReservaID: reservaID,
		Tipo:      "sena",
		Monto:     7.50,
		Metodo:    "transferencia",
		Estado:    EstadoVerificado,
	})
}
