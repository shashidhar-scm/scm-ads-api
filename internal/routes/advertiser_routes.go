package routes

import (
	"database/sql"
	"log"

	"github.com/go-chi/chi/v5"
	"scm/internal/handlers"
	authmw "scm/internal/middleware"
	"scm/internal/repository"
)

func RegisterAdvertiserRoutes(router chi.Router, db *sql.DB) {
	log.Println("Registering advertiser routes...")
	
	// Initialize repository and handler
	advertiserRepo := repository.NewAdvertiserRepository(db)
	advertiserHandler := handlers.NewAdvertiserHandler(advertiserRepo, db)

	// Define routes
	router.Route("/advertisers", func(r chi.Router) {
		r.With(authmw.RequirePermissionNoAdvertiserSelection(db, "advertisers.read")).Get("/search", advertiserHandler.SearchAdvertisers)
		r.With(authmw.RequirePermissionNoAdvertiserSelection(db, "advertisers.read")).Get("/", advertiserHandler.ListAdvertisers)
		r.With(authmw.RequirePermissionNoAdvertiserSelection(db, "advertisers.write")).Post("/", advertiserHandler.CreateAdvertiser)
		r.Route("/{id}", func(r chi.Router) {
			r.With(authmw.RequirePermissionNoAdvertiserSelection(db, "advertisers.read")).Get("/", advertiserHandler.GetAdvertiser)
			r.With(authmw.RequirePermission(db, "advertisers.write")).Put("/", advertiserHandler.UpdateAdvertiser)
			r.With(authmw.RequirePermission(db, "advertisers.write")).Delete("/", advertiserHandler.DeleteAdvertiser)
		})
	})
}
