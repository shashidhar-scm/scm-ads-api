// internal/routes/campaign_routes.go
package routes

import (
    "database/sql"
    "github.com/go-chi/chi/v5"
    "scm/internal/handlers"
    "scm/internal/repository"
    "log"
	"net/http"
)

func RegisterCampaignRoutes(router chi.Router, db *sql.DB) {
    log.Println("Registering campaign routes...")

    campaignRepo := repository.NewCampaignRepository(db)
    campaignHandler := handlers.NewCampaignHandler(campaignRepo)

    router.Route("/campaigns", func(r chi.Router) {
        r.Get("/", campaignHandler.ListCampaigns)
        r.Get("/advertiser/{advertiserID}", campaignHandler.ListCampaignsByAdvertiser)
        r.Post("/", func(w http.ResponseWriter, r *http.Request) {
            log.Println("POST /campaigns endpoint hit")
            campaignHandler.CreateCampaign(w, r)
        })
        
        r.Route("/{id}", func(r chi.Router) {
            r.Get("/", campaignHandler.GetCampaign)
            r.Put("/", campaignHandler.UpdateCampaign)
            r.Delete("/", campaignHandler.DeleteCampaign)
        })
    })
}