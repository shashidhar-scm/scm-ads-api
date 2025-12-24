package routes

import (
	"database/sql"

	"github.com/go-chi/chi/v5"
	"scm/internal/handlers"
	"scm/internal/repository"
)

func RegisterVenueRoutes(r chi.Router, db *sql.DB) {
	repo := repository.NewVenueRepository(db)
	handler := handlers.NewVenueHandler(repo)

	r.Route("/venues", func(r chi.Router) {
		r.Get("/", handler.List)
		r.Post("/", handler.Create)
		r.Get("/{id}", handler.Get)
		r.Put("/{id}", handler.Update)
		r.Delete("/{id}", handler.Delete)
		
		// Bulk operations for many-to-many relationships
		r.Post("/{id}/devices", handler.AddDevicesToVenue)
		r.Delete("/{id}/devices", handler.RemoveDevicesFromVenue)
		r.Get("/{id}/devices", handler.GetDevicesByVenue)
	})

	// Route for listing venues by device
	r.Get("/devices/{deviceID}/venues", handler.ListByDevice)
}
