package routes

import (
	"database/sql"

	"github.com/go-chi/chi/v5"
	"scm/internal/handlers"
	authmw "scm/internal/middleware"
	"scm/internal/repository"
)

func RegisterReplicatorRoutes(r chi.Router, db *sql.DB, userRoleRepo repository.UserRoleRepository, replicatorDB *sql.DB) {
	h := handlers.NewReplicatorHandler(replicatorDB)

	r.Route("/replicator", func(r chi.Router) {
		r.With(authmw.RequirePermission(userRoleRepo, "replicator.read")).Get("/ad_posters", h.ListAdPosters)
	})
}
