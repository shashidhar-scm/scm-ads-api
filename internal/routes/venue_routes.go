package routes

import (
	"database/sql"

	"github.com/go-chi/chi/v5"
	"scm/internal/handlers"
	authmw "scm/internal/middleware"
	"scm/internal/repository"
)

func RegisterVenueRoutes(r chi.Router, db *sql.DB, userRoleRepo repository.UserRoleRepository) {
	repo := repository.NewVenueRepository(db)
	handler := handlers.NewVenueHandler(repo)

	r.Route("/venues", func(r chi.Router) {
		readPerm := authmw.RequirePermission(userRoleRepo, "venues.read")
		writePerm := authmw.RequirePermission(userRoleRepo, "venues.write")
		r.With(readPerm).Get("/search", handler.Search)
		r.With(readPerm).Get("/", handler.List)
		r.With(writePerm).Post("/", handler.Create)
		r.With(readPerm).Get("/{id}", handler.Get)
		r.With(writePerm).Put("/{id}", handler.Update)
		r.With(writePerm).Delete("/{id}", handler.Delete)

		// Bulk operations for many-to-many relationships
		r.With(writePerm).Post("/{id}/devices", handler.AddDevicesToVenue)
		r.With(writePerm).Delete("/{id}/devices", handler.RemoveDevicesFromVenue)
		r.With(readPerm).Get("/{id}/devices", handler.GetDevicesByVenue)
	})

	// Route for listing venues by device
	r.With(authmw.RequirePermission(userRoleRepo, "venues.read")).Get("/devices/{deviceID}/venues", handler.ListByDevice)
}
