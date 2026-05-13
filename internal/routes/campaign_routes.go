// internal/routes/campaign_routes.go
package routes

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"scm/internal/handlers"
	"scm/internal/interfaces"
	authmw "scm/internal/middleware"
	"scm/internal/repository"
	"scm/internal/services"
)

func RegisterCampaignRoutes(router chi.Router, db *sql.DB, campaignRepo interfaces.CampaignRepository, popBaseURL string, userRoleRepo repository.UserRoleRepository) {
	log.Println("Registering campaign routes...")

	popClient := services.NewPopClient(popBaseURL)
	campaignHandler := handlers.NewCampaignHandlerWithPop(campaignRepo, popClient, db, userRoleRepo)

	router.Route("/campaigns", func(r chi.Router) {
		readPermission := authmw.RequirePermission(userRoleRepo, "campaigns.read")
		writePermission := authmw.RequirePermission(userRoleRepo, "campaigns.write")
		r.With(readPermission).Get("/search", campaignHandler.SearchCampaigns)
		r.With(readPermission).Get("/", campaignHandler.ListCampaigns)
		r.With(readPermission).Get("/advertiser/{advertiserID}", campaignHandler.ListCampaignsByAdvertiser)
		r.With(writePermission).Post("/", func(w http.ResponseWriter, r *http.Request) {
			log.Println("POST /campaigns endpoint hit")
			campaignHandler.CreateCampaign(w, r)
		})

		r.Route("/{id}", func(r chi.Router) {
			r.With(readPermission).Get("/", campaignHandler.GetCampaign)
			r.With(readPermission).Get("/impressions", campaignHandler.GetCampaignImpressions)
			r.With(writePermission).Put("/", campaignHandler.UpdateCampaign)
			r.With(writePermission).Delete("/", campaignHandler.DeleteCampaign)
		})

	})
}
