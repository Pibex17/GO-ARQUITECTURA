// Package respuesta define la única forma en que el servidor responde,
// tanto en éxito como en error. Un cliente mira el campo "ok" y sabe
// dónde buscar el resto.
package respuesta

import (
	"encoding/json"
	"net/http"
)

type detalleError struct {
	Codigo  string `json:"codigo"`
	Mensaje string `json:"mensaje"`
}

type envoltura struct {
	OK    bool          `json:"ok"`
	Datos any           `json:"datos,omitempty"`
	Error *detalleError `json:"error,omitempty"`
}

func escribir(w http.ResponseWriter, estado int, cuerpo envoltura) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(estado) // punto de no retorno
	_ = json.NewEncoder(w).Encode(cuerpo)
}

// Exito responde con el estado indicado y los datos dentro de la envoltura.
func Exito(w http.ResponseWriter, estado int, datos any) {
	escribir(w, estado, envoltura{OK: true, Datos: datos})
}

// Error responde con la envoltura de error. Nunca expone detalles internos.
func Error(w http.ResponseWriter, estado int, codigo, mensaje string) {
	escribir(w, estado, envoltura{OK: false,
		Error: &detalleError{Codigo: codigo, Mensaje: mensaje}})
}
