package routes

import (
	"database/sql"

	"scm/internal/handlers"
	"scm/internal/repository"

	"github.com/go-chi/chi/v5"
)

func RegisterProjectRoutes(r chi.Router, db *sql.DB) {
	repo := repository.NewProjectRepository(db)
	handler := handlers.NewProjectHandler(repo)

	r.Route("/projects", func(r chi.Router) {
		r.Get("/", handler.List)
		r.Get("/{name}", handler.Get)
	})
}
