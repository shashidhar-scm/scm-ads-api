package routes

import (
	"database/sql"

	"scm/internal/handlers"

	"github.com/go-chi/chi/v5"
)

func RegisterRegionSyncRoutes(r chi.Router, db *sql.DB, replicaDB *sql.DB) {
	h := handlers.NewRegionSyncHandler(db, replicaDB)

	r.Route("/sync", func(r chi.Router) {
		r.Post("/posters", h.SyncPosters)
		r.Post("/adposters", h.SyncAdPosters)
	})
}
