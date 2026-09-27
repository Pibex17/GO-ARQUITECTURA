// Package middleware trae los dos middleware propios del hilo servidor:
// Registro (loguea método, ruta, código y duración) y Recuperacion
// (atrapa un pánico y responde 500 con la envoltura, sin tumbar el servidor).
package middleware

import (
	"log"
	"net/http"
	"time"

	"canchas-sinteticas/internal/respuesta"
)

// grabador envuelve al ResponseWriter para poder saber, después, qué
// código de estado escribió el handler (ResponseWriter no lo expone).
type grabador struct {
	http.ResponseWriter
	estado int
}

func (g *grabador) WriteHeader(codigo int) {
	g.estado = codigo
	g.ResponseWriter.WriteHeader(codigo)
}

// Registro escribe una línea por petición: método, ruta, estado y duración.
func Registro(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		g := &grabador{ResponseWriter: w, estado: http.StatusOK}
		next.ServeHTTP(g, r)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, g.estado, time.Since(inicio))
	})
}

// Recuperacion atrapa un pánico y lo convierte en 500 con la envoltura.
// El detalle del pánico va al log del servidor, nunca al cliente.
func Recuperacion(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				log.Printf("PÁNICO en %s %s: %v", r.Method, r.URL.Path, p)
				respuesta.Error(w, http.StatusInternalServerError,
					"error_interno", "ocurrió un error inesperado")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
