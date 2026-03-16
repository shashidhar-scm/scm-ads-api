package routes

import (
	"database/sql"

	"scm/internal/handlers"

	"github.com/go-chi/chi/v5"
)

func RegisterLegacyRoutes(r chi.Router, db *sql.DB, replicatorDB *sql.DB) {
	h := handlers.NewLegacyHandler(db, replicatorDB)

	r.Route("/scm-api", func(r chi.Router) {
		r.Get("/theme", h.GetTheme)
		r.Get("/getContent", h.GetContent)
		r.Get("/getLoopPostersWeb", h.GetLoopPostersWeb)
	})
}
