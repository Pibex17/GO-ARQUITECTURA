package inscripcion

import "time"

type Inscripcion struct {
	ID        uint
	PartidoID uint
	JugadorID uint
	CreadoEn  time.Time
}
