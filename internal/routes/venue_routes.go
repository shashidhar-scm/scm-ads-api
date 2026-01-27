package routes

import (
	"database/sql"

	"github.com/go-chi/chi/v5"
	"scm/internal/handlers"
	authmw "scm/internal/middleware"
	"scm/internal/repository"
)

func RegisterVenueRoutes(r chi.Router, db *sql.DB) {
	repo := repository.NewVenueRepository(db)
	handler := handlers.NewVenueHandler(repo)

	r.Route("/venues", func(r chi.Router) {
		r.With(authmw.RequirePermission(db, "venues.read")).Get("/search", handler.Search)
		r.With(authmw.RequirePermission(db, "venues.read")).Get("/", handler.List)
		r.With(authmw.RequirePermission(db, "venues.write")).Post("/", handler.Create)
		r.With(authmw.RequirePermission(db, "venues.read")).Get("/{id}", handler.Get)
		r.With(authmw.RequirePermission(db, "venues.write")).Put("/{id}", handler.Update)
		r.With(authmw.RequirePermission(db, "venues.write")).Delete("/{id}", handler.Delete)
		
		// Bulk operations for many-to-many relationships
		r.With(authmw.RequirePermission(db, "venues.write")).Post("/{id}/devices", handler.AddDevicesToVenue)
		r.With(authmw.RequirePermission(db, "venues.write")).Delete("/{id}/devices", handler.RemoveDevicesFromVenue)
		r.With(authmw.RequirePermission(db, "venues.read")).Get("/{id}/devices", handler.GetDevicesByVenue)
	})

	// Route for listing venues by device
	r.With(authmw.RequirePermission(db, "venues.read")).Get("/devices/{deviceID}/venues", handler.ListByDevice)
}
