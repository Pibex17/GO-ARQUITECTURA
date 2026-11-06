package reservas

// DTO de transporte de reservas. La entidad Reserva no se serializa
// directamente: el cuerpo de entrada no puede fijar ID, dueño ni estado, y
// el de salida declara exactamente los campos que el cliente puede ver.
//
// Se conservan los nombres PascalCase del contrato actual (Bruno, OpenAPI y
// pruebas dependen de ellos); no se usan etiquetas json en la entidad.

// ReservaPeticion es el cuerpo de POST /reservas. No incluye ID, ClienteID
// ni Estado: el id lo asigna la base, el dueño sale de la sesión y el estado
// inicial lo decide el servicio.
type ReservaPeticion struct {
	CanchaID   uint   `json:"CanchaID"`
	Fecha      string `json:"Fecha"`
	HoraInicio string `json:"HoraInicio"`
	HoraFin    string `json:"HoraFin"`
}

// aEntrada traduce el DTO al caso de uso, sin arrastrar campos de más.
func (p ReservaPeticion) aEntrada() EntradaCrear {
	return EntradaCrear{
		CanchaID:   p.CanchaID,
		Fecha:      p.Fecha,
		HoraInicio: p.HoraInicio,
		HoraFin:    p.HoraFin,
	}
}

// CambioEstadoPeticion es el cuerpo de PATCH /reservas/{id}/estado. El
// estado destino sí viene del cliente, pero solo por esta ruta de
// administrador y sujeto a la máquina de estados del servicio.
type CambioEstadoPeticion struct {
	NuevoEstado    string `json:"NuevoEstado"`
	Metodo         string `json:"Metodo"`         // solo si NuevoEstado es "pago_por_verificar"
	ComprobanteURL string `json:"ComprobanteURL"` // solo si NuevoEstado es "pago_por_verificar"
}

// PagoRespuesta es la vista pública de un pago dentro de una reserva.
type PagoRespuesta struct {
	ID             uint    `json:"ID"`
	ReservaID      uint    `json:"ReservaID"`
	Tipo           string  `json:"Tipo"`
	Monto          float64 `json:"Monto"`
	Metodo         string  `json:"Metodo"`
	ComprobanteURL string  `json:"ComprobanteURL"`
	Estado         string  `json:"Estado"`
}

// ReservaRespuesta es la vista pública de una reserva, con sus pagos.
type ReservaRespuesta struct {
	ID         uint            `json:"ID"`
	CanchaID   uint            `json:"CanchaID"`
	ClienteID  uint            `json:"ClienteID"`
	Fecha      string          `json:"Fecha"`
	HoraInicio string          `json:"HoraInicio"`
	HoraFin    string          `json:"HoraFin"`
	Estado     string          `json:"Estado"`
	Pagos      []PagoRespuesta `json:"Pagos"`
}

// nuevaReservaRespuesta convierte una entidad en su DTO. Conserva la
// diferencia entre Pagos nil (null) y vacío ([]), igual que antes.
func nuevaReservaRespuesta(r Reserva) ReservaRespuesta {
	var pagos []PagoRespuesta
	if r.Pagos != nil {
		pagos = make([]PagoRespuesta, 0, len(r.Pagos))
		for _, p := range r.Pagos {
			pagos = append(pagos, PagoRespuesta{
				ID:             p.ID,
				ReservaID:      p.ReservaID,
				Tipo:           p.Tipo,
				Monto:          p.Monto,
				Metodo:         p.Metodo,
				ComprobanteURL: p.ComprobanteURL,
				Estado:         p.Estado,
			})
		}
	}

	return ReservaRespuesta{
		ID:         r.ID,
		CanchaID:   r.CanchaID,
		ClienteID:  r.ClienteID,
		Fecha:      r.Fecha,
		HoraInicio: r.HoraInicio,
		HoraFin:    r.HoraFin,
		Estado:     r.Estado,
		Pagos:      pagos,
	}
}

// nuevasReservasRespuesta convierte una lista completa sin cambiar null por
// [] ni al revés.
func nuevasReservasRespuesta(lista []Reserva) []ReservaRespuesta {
	if lista == nil {
		return nil
	}

	respuesta := make([]ReservaRespuesta, 0, len(lista))
	for _, r := range lista {
		respuesta = append(respuesta, nuevaReservaRespuesta(r))
	}
	return respuesta
}
