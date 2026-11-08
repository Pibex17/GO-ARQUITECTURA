package eventos

// Tipos de evento del ciclo de vida de una reserva y de sus participantes.
const (
	TipoReservaCreada = "ReservaCreada"

	TipoSenaDeclarada       = "SenaDeclarada"
	TipoSenaRegistrada      = "SenaRegistrada"
	TipoRegistroSenaFallido = "RegistroSenaFallido"

	TipoSenaAprobada                 = "SenaAprobada"
	TipoSenaVerificada               = "SenaVerificada"
	TipoVerificacionSenaFallida      = "VerificacionSenaFallida"
	TipoVerificacionRevertida        = "VerificacionRevertida"
	TipoReservaPasoAPagoPorVerificar = "ReservaPasoAPagoPorVerificar"
	TipoReservaConfirmada            = "ReservaConfirmada"
	TipoReservaNoActualizada         = "ReservaNoActualizada"
	TipoReservaNoConfirmada          = "ReservaNoConfirmada"

	TipoCancelacionSolicitada = "CancelacionSolicitada"
	TipoReservaCancelada      = "ReservaCancelada"
	TipoJugadaSolicitada      = "JugadaSolicitada"
	TipoReservaJugada         = "ReservaJugada"

	TipoPagoAnulado = "PagoAnulado"

	TipoSolicitudPartidoRecibida   = "SolicitudPartidoRecibida"
	TipoReservaValidadaParaPartido = "ReservaValidadaParaPartido"
	TipoPartidoCreado              = "PartidoCreado"
	TipoPartidoFallido             = "PartidoFallido"
	TipoPartidoCancelado           = "PartidoCancelado"

	TipoProcesamientoFallido = "ProcesamientoFallido"
)

// ReservaCreada abre la saga de una reserva.
type ReservaCreada struct {
	ReservaID uint `json:"reserva_id"`
	CanchaID  uint `json:"cancha_id"`
	ClienteID uint `json:"cliente_id"`
}

// SenaDeclarada lo publica el cliente cuando declara el pago de la seña.
type SenaDeclarada struct {
	ReservaID      uint    `json:"reserva_id"`
	CanchaID       uint    `json:"cancha_id"`
	PrecioHora     float64 `json:"precio_hora"`
	Metodo         string  `json:"metodo"`
	ComprobanteURL string  `json:"comprobante_url"`
}

// SenaRegistrada confirma que el participante de pagos creó la seña.
type SenaRegistrada struct {
	ReservaID uint    `json:"reserva_id"`
	PagoID    uint    `json:"pago_id"`
	Monto     float64 `json:"monto"`
}

// RegistroSenaFallido compensa una seña que no se pudo crear.
type RegistroSenaFallido struct {
	ReservaID uint   `json:"reserva_id"`
	Motivo    string `json:"motivo"`
}

// SenaAprobada lo publica el administrador al aprobar la seña.
type SenaAprobada struct {
	ReservaID uint `json:"reserva_id"`
}

// SenaVerificada confirma que la seña quedó verificada.
type SenaVerificada struct {
	ReservaID uint `json:"reserva_id"`
	PagoID    uint `json:"pago_id"`
}

// VerificacionSenaFallida compensa una verificación que no se pudo aplicar.
type VerificacionSenaFallida struct {
	ReservaID uint   `json:"reserva_id"`
	Motivo    string `json:"motivo"`
}

// ReservaPasoAPagoPorVerificar confirma el cambio de estado de la reserva.
type ReservaPasoAPagoPorVerificar struct {
	ReservaID uint `json:"reserva_id"`
}

// ReservaConfirmada confirma que la reserva quedó confirmada.
type ReservaConfirmada struct {
	ReservaID uint `json:"reserva_id"`
}

// ReservaNoActualizada compensa una reserva que no pudo guardar su estado.
type ReservaNoActualizada struct {
	ReservaID uint   `json:"reserva_id"`
	Motivo    string `json:"motivo"`
}

// ReservaNoConfirmada compensa una reserva que no pudo confirmarse.
type ReservaNoConfirmada struct {
	ReservaID uint   `json:"reserva_id"`
	Motivo    string `json:"motivo"`
}

// CancelacionSolicitada lo publica el administrador al cancelar.
type CancelacionSolicitada struct {
	ReservaID uint `json:"reserva_id"`
}

// ReservaCancelada confirma la cancelación y dispara compensaciones.
type ReservaCancelada struct {
	ReservaID uint `json:"reserva_id"`
}

// JugadaSolicitada lo publica el administrador al marcar la reserva jugada.
type JugadaSolicitada struct {
	ReservaID uint `json:"reserva_id"`
}

// ReservaJugada confirma el estado terminal jugada.
type ReservaJugada struct {
	ReservaID uint `json:"reserva_id"`
}

// PagoAnulado confirma que la seña quedó anulada.
type PagoAnulado struct {
	ReservaID uint `json:"reserva_id"`
	PagoID    uint `json:"pago_id"`
}

// VerificacionRevertida confirma que la seña volvió a pendiente.
type VerificacionRevertida struct {
	ReservaID uint `json:"reserva_id"`
	PagoID    uint `json:"pago_id"`
}

// SolicitudPartidoRecibida abre la creación de un partido.
type SolicitudPartidoRecibida struct {
	ReservaID  uint   `json:"reserva_id"`
	ClienteID  uint   `json:"cliente_id"`
	Nombre     string `json:"nombre"`
	CupoMaximo int    `json:"cupo_maximo"`
}

// ReservaValidadaParaPartido confirma que la reserva permite crear el partido.
type ReservaValidadaParaPartido struct {
	ReservaID  uint   `json:"reserva_id"`
	ClienteID  uint   `json:"cliente_id"`
	Nombre     string `json:"nombre"`
	CupoMaximo int    `json:"cupo_maximo"`
}

// PartidoCreado confirma la creación del partido.
type PartidoCreado struct {
	ReservaID uint `json:"reserva_id"`
	PartidoID uint `json:"partido_id"`
}

// PartidoFallido compensa un partido que no se pudo crear.
type PartidoFallido struct {
	ReservaID uint   `json:"reserva_id"`
	Motivo    string `json:"motivo"`
}

// PartidoCancelado confirma la cancelación del partido por compensación.
type PartidoCancelado struct {
	ReservaID uint `json:"reserva_id"`
	PartidoID uint `json:"partido_id"`
}

// ProcesamientoFallido lo publica el despachador cuando un evento agota
// sus reintentos. Permite que la saga quede marcada como fallida.
type ProcesamientoFallido struct {
	EventoID   string `json:"evento_id"`
	TipoEvento string `json:"tipo_evento"`
	AgregadoID uint   `json:"agregado_id"`
	SagaID     string `json:"saga_id"`
	Motivo     string `json:"motivo"`
}
