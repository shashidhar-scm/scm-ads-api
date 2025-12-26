package routes

import (
	"database/sql"

	"scm/internal/handlers"
	"scm/internal/repository"

	"github.com/go-chi/chi/v5"
)

func RegisterDeviceReadRoutes(r chi.Router, db *sql.DB) {
	repo := repository.NewDeviceRepository(db)
	handler := handlers.NewDeviceReadHandler(repo)

	r.Route("/devices", func(r chi.Router) {
		r.Get("/counts/regions", handler.CountByRegion)
		r.Get("/", handler.List)
		r.Get("/{hostName}", handler.Get)
	})
}
