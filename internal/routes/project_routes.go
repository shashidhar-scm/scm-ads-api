package routes

import (
	"database/sql"

	"scm/internal/handlers"
	authmw "scm/internal/middleware"
	"scm/internal/repository"

	"github.com/go-chi/chi/v5"
)

func RegisterProjectRoutes(r chi.Router, db *sql.DB, userRoleRepo repository.UserRoleRepository, projectRepo repository.ProjectRepository) {
	handler := handlers.NewProjectHandler(projectRepo)

	r.Route("/projects", func(r chi.Router) {
		perm := authmw.RequirePermission(userRoleRepo, "projects.read")
		r.With(perm).Get("/search", handler.Search)
		r.With(perm).Get("/", handler.List)
		r.With(perm).Get("/{name}", handler.Get)
	})
}
