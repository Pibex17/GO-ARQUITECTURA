package sagas

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"canchas-sinteticas/internal/respuesta"
	"canchas-sinteticas/internal/sesion"
)

// Manejador expone la consulta del estado de una saga.
type Manejador struct {
	DB   *gorm.DB
	Repo Repositorio
}

func (m *Manejador) repositorio() Repositorio {
	if m.Repo != nil {
		return m.Repo
	}
	return RepositorioGORM{DB: m.DB}
}

// Rutas registra la consulta de sagas. Cualquier sesión válida puede
// consultar el estado de una saga.
func (m *Manejador) Rutas(r chi.Router, tokens *sesion.Tokens) {
	r.With(tokens.Autenticar).Get("/sagas/{id}", m.obtener)
}

// GET /sagas/{id}
func (m *Manejador) obtener(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respuesta.Error(w, http.StatusBadRequest,
			"id_invalido", "el id de la saga es obligatorio")
		return
	}

	saga, err := m.repositorio().PorID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respuesta.Error(w, http.StatusNotFound,
				"no_encontrado", "no existe una saga con ese id")
			return
		}
		respuesta.Error(w, http.StatusInternalServerError,
			"error_base", "no se pudo leer la saga")
		return
	}
	respuesta.Exito(w, http.StatusOK, saga)
}
