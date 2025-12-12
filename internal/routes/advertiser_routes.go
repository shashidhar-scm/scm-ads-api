package routes

import (
	"database/sql"
	"log"

	"github.com/go-chi/chi/v5"
	"scm/internal/handlers"
	"scm/internal/repository"
)

func RegisterAdvertiserRoutes(router chi.Router, db *sql.DB) {
	log.Println("Registering advertiser routes...")
	
	// Initialize repository and handler
	advertiserRepo := repository.NewAdvertiserRepository(db)
	advertiserHandler := handlers.NewAdvertiserHandler(advertiserRepo)

	// Define routes
	router.Route("/advertisers", func(r chi.Router) {
		r.Get("/", advertiserHandler.ListAdvertisers)
		r.Post("/", advertiserHandler.CreateAdvertiser)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", advertiserHandler.GetAdvertiser)
			r.Put("/", advertiserHandler.UpdateAdvertiser)
			r.Delete("/", advertiserHandler.DeleteAdvertiser)
		})
	})
}
