package routes

import (
	"database/sql"

	"scm/internal/handlers"
	authmw "scm/internal/middleware"
	"scm/internal/repository"

	"github.com/go-chi/chi/v5"
)

func RegisterDeviceReadRoutes(r chi.Router, db *sql.DB, userRoleRepo repository.UserRoleRepository, deviceRepo repository.DeviceRepository) {
	handler := handlers.NewDeviceReadHandler(deviceRepo)

	r.Route("/devices", func(r chi.Router) {
		perm := authmw.RequirePermission(userRoleRepo, "devices.read")
		r.With(perm).Get("/recommendations", handler.Recommend)
		r.With(perm).Post("/query", handler.Query)
		r.With(perm).Get("/search", handler.Search)
		r.With(perm).Get("/counts/regions", handler.CountByRegion)
		r.With(perm).Get("/", handler.List)
		r.With(perm).Get("/{hostName}", handler.Get)
	})
}
