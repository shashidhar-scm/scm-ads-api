package routes

import (
	"database/sql"

	"scm/internal/handlers"
	authmw "scm/internal/middleware"
	"scm/internal/repository"

	"github.com/go-chi/chi/v5"
)

func RegisterProjectRoutes(r chi.Router, db *sql.DB) {
	repo := repository.NewProjectRepository(db)
	handler := handlers.NewProjectHandler(repo)

	r.Route("/projects", func(r chi.Router) {
		r.With(authmw.RequirePermission(db, "projects.read")).Get("/search", handler.Search)
		r.With(authmw.RequirePermission(db, "projects.read")).Get("/", handler.List)
		r.With(authmw.RequirePermission(db, "projects.read")).Get("/{name}", handler.Get)
	})
}
