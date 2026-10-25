package sesion

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"canchas-sinteticas/internal/respuesta"
)

// clave es el tipo de la llave con la que la sesión se guarda en el
// contexto de la petición. Es propio de este paquete: nadie más puede
// escribir ahí.
type clave struct{}

// De devuelve la sesión que el guardia dejó en la petición.
func De(r *http.Request) (Sesion, bool) {
	s, ok := r.Context().Value(clave{}).(Sesion)
	return s, ok
}

// Autenticar es el guardia: corre ANTES que el manejador.
// Sin token, con un token cambiado o con un token vencido responde 401,
// y el manejador que sigue nunca se entera.
func (t *Tokens) Autenticar(siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cabecera := r.Header.Get("Authorization")
		texto, ok := strings.CutPrefix(cabecera, "Bearer ")
		if !ok || texto == "" {
			respuesta.Error(w, http.StatusUnauthorized,
				"falta_token", "falta el token: inicie sesión en POST /login")
			return
		}

		s, err := t.Leer(texto)
		if errors.Is(err, ErrVencido) {
			respuesta.Error(w, http.StatusUnauthorized,
				"token_vencido", "el token está vencido: inicie sesión otra vez")
			return
		}
		if err != nil {
			respuesta.Error(w, http.StatusUnauthorized,
				"token_invalido", "el token no es válido")
			return
		}

		// Ya se sabe quién es: se anota en la petición y se sigue.
		ctx := context.WithValue(r.Context(), clave{}, s)
		siguiente.ServeHTTP(w, r.WithContext(ctx))
	})
}

// SoloRol responde a la segunda pregunta: ya sé quién es, ¿puede hacer
// esto? Si su rol no es el pedido responde 403. Va DESPUÉS de Autenticar.
func SoloRol(rol string) func(http.Handler) http.Handler {
	return func(siguiente http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s, ok := De(r)
			if !ok {
				respuesta.Error(w, http.StatusUnauthorized,
					"falta_token", "falta el token: inicie sesión en POST /login")
				return
			}
			if s.Rol != rol {
				respuesta.Error(w, http.StatusForbidden,
					"permiso_denegado", "su rol no tiene permiso para esta acción")
				return
			}
			siguiente.ServeHTTP(w, r)
		})
	}
}
