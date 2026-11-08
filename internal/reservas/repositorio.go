package reservas

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"canchas-sinteticas/internal/canchas"
	"canchas-sinteticas/internal/eventos"
)

// FiltroReservas describe qué reservas puede ver una sesión. El manejador
// decide los campos de propiedad; el repositorio solo arma la consulta.
type FiltroReservas struct {
	Estado               string
	ClienteID            *uint // filtro adicional del administrador
	PropietarioClienteID *uint // reservas de este cliente (rol cliente)
	AdministradorID      *uint // reservas de las canchas de este administrador
}

// ConstructorEvento construye el evento que acompaña a la creación de una
// reserva. Se ejecuta dentro de la misma transacción, después de que la
// reserva recibe su ID.
type ConstructorEvento func(reserva *Reserva) eventos.Evento

// Repositorio es la parte de la base que necesita el contexto de reservas.
type Repositorio interface {
	CanchaPorID(id uint) (canchas.Cancha, error)
	HayConflicto(canchaID uint, fecha, horaInicio string) (bool, error)
	CrearReservaConEvento(reserva *Reserva, nuevoEvento ConstructorEvento) error
	ListarReservas(filtro FiltroReservas) ([]Reserva, error)
	ReservaPorID(id uint) (Reserva, error)
	ReservaPorIDEnTx(tx *gorm.DB, id uint) (Reserva, error)
	GuardarReservaEnTx(tx *gorm.DB, reserva *Reserva) error
}

// RepositorioGORM implementa Repositorio con PostgreSQL/GORM.
type RepositorioGORM struct {
	DB *gorm.DB
}

func (r RepositorioGORM) CanchaPorID(id uint) (canchas.Cancha, error) {
	var cancha canchas.Cancha
	err := r.DB.First(&cancha, id).Error
	return cancha, err
}

func (r RepositorioGORM) HayConflicto(canchaID uint, fecha, horaInicio string) (bool, error) {
	var enConflicto int64
	err := r.DB.Model(&Reserva{}).
		Where("cancha_id = ? AND fecha = ? AND hora_inicio = ? AND estado <> ?",
			canchaID, fecha, horaInicio, "cancelada").
		Count(&enConflicto).Error
	return enConflicto > 0, err
}

// CrearReservaConEvento guarda la reserva y su evento ReservaCreada en la
// misma transacción: o se guardan ambos, o no se guarda ninguno.
func (r RepositorioGORM) CrearReservaConEvento(reserva *Reserva, nuevoEvento ConstructorEvento) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(reserva).Error; err != nil {
			return err
		}
		evento := nuevoEvento(reserva)
		return tx.Create(&evento).Error
	})
}

func (r RepositorioGORM) ListarReservas(filtro FiltroReservas) ([]Reserva, error) {
	consulta := r.DB.Preload("Pagos")

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
	return reserva, err
}

// ReservaPorIDEnTx bloquea la fila mientras el consumidor aplica la
// transición, para que dos eventos concurrentes no pisen el mismo estado.
func (r RepositorioGORM) ReservaPorIDEnTx(tx *gorm.DB, id uint) (Reserva, error) {
	var reserva Reserva
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&reserva, id).Error
	return reserva, err
}

func (r RepositorioGORM) GuardarReservaEnTx(tx *gorm.DB, reserva *Reserva) error {
	return tx.Save(reserva).Error
}
