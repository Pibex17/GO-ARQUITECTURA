package partidos

import "time"

type Partido struct {
	ID               uint
	ReservaID        uint
	Nombre           string
	CupoMaximo       int
	CodigoInvitacion string
	CreadoEn         time.Time
}
