package routes

import (
	"database/sql"
	"log"

	"github.com/go-chi/chi/v5"
	"scm/internal/handlers"
	"scm/internal/interfaces"
	authmw "scm/internal/middleware"
	"scm/internal/repository"
)

func RegisterAdvertiserRoutes(router chi.Router, db *sql.DB, advertiserRepo interfaces.AdvertiserRepository, userRoleRepo repository.UserRoleRepository) {
	log.Println("Registering advertiser routes...")

	// Initialize handler
	advertiserHandler := handlers.NewAdvertiserHandler(advertiserRepo, db, userRoleRepo)

	// Define routes
	router.Route("/advertisers", func(r chi.Router) {
		r.With(authmw.RequirePermission(userRoleRepo, "advertisers.read")).Get("/search", advertiserHandler.SearchAdvertisers)
		r.With(authmw.RequirePermission(userRoleRepo, "advertisers.read")).Get("/", advertiserHandler.ListAdvertisers)
		r.With(authmw.RequirePermission(userRoleRepo, "advertisers.write")).Post("/", advertiserHandler.CreateAdvertiser)
		r.Route("/{id}", func(r chi.Router) {
			r.With(authmw.RequirePermission(userRoleRepo, "advertisers.read")).Get("/", advertiserHandler.GetAdvertiser)
			r.With(authmw.RequirePermission(userRoleRepo, "advertisers.write")).Put("/", advertiserHandler.UpdateAdvertiser)
			r.With(authmw.RequirePermission(userRoleRepo, "advertisers.write")).Delete("/", advertiserHandler.DeleteAdvertiser)
		})
	})
}
