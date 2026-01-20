package routes

import (
	"database/sql"

	"scm/internal/handlers"
	authmw "scm/internal/middleware"
	"scm/internal/repository"

	"github.com/go-chi/chi/v5"
)

func RegisterDeviceReadRoutes(r chi.Router, db *sql.DB) {
	repo := repository.NewDeviceRepository(db)
	handler := handlers.NewDeviceReadHandler(repo)

	r.Route("/devices", func(r chi.Router) {
		r.With(authmw.RequirePermissionNoAdvertiserSelection(db, "devices.read")).Post("/query", handler.Query)
		r.With(authmw.RequirePermissionNoAdvertiserSelection(db, "devices.read")).Get("/search", handler.Search)
		r.With(authmw.RequirePermissionNoAdvertiserSelection(db, "devices.read")).Get("/counts/regions", handler.CountByRegion)
		r.With(authmw.RequirePermissionNoAdvertiserSelection(db, "devices.read")).Get("/", handler.List)
		r.With(authmw.RequirePermissionNoAdvertiserSelection(db, "devices.read")).Get("/{hostName}", handler.Get)
	})
}
