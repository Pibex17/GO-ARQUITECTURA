package reservas

import (
	"gorm.io/gorm"

	"canchas-sinteticas/internal/canchas"
)

// FiltroReservas describe qué reservas puede ver una sesión. El manejador
// decide los campos de propiedad; el repositorio solo arma la consulta.
type FiltroReservas struct {
	Estado               string
	ClienteID            *uint // filtro adicional del administrador
	PropietarioClienteID *uint // reservas de este cliente (rol cliente)
	AdministradorID      *uint // reservas de las canchas de este administrador
}

// Repositorio es la parte de la base que necesita el manejador de reservas.
// Es una interfaz para poder probar la autorización con dobles.
type Repositorio interface {
	CanchaPorID(id uint) (canchas.Cancha, error)
	HayConflicto(canchaID uint, fecha, horaInicio string) (bool, error)
	CrearReserva(reserva *Reserva) error
	ListarReservas(filtro FiltroReservas) ([]Reserva, error)
	ReservaPorID(id uint) (Reserva, error)
	GuardarReserva(reserva *Reserva) error
	CrearPago(pago *Pago) error
	MarcarPagoSenaVerificado(reservaID uint) error
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
	return reserva, err
}

func (r RepositorioGORM) GuardarReserva(reserva *Reserva) error {
	return r.DB.Save(reserva).Error
}

func (r RepositorioGORM) CrearPago(pago *Pago) error {
	return r.DB.Create(pago).Error
}

func (r RepositorioGORM) MarcarPagoSenaVerificado(reservaID uint) error {
	return r.DB.Model(&Pago{}).
		Where("reserva_id = ? AND tipo = ? AND estado = ?", reservaID, "sena", "pendiente").
		Update("estado", "verificado").Error
}
