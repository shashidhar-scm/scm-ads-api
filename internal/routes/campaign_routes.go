// internal/routes/campaign_routes.go
package routes

import (
    "database/sql"
    "github.com/go-chi/chi/v5"
    "scm/internal/handlers"
    authmw "scm/internal/middleware"
    "scm/internal/services"
    "scm/internal/repository"
    "log"
	"net/http"
)

func RegisterCampaignRoutes(router chi.Router, db *sql.DB, popBaseURL string) {
    log.Println("Registering campaign routes...")

    campaignRepo := repository.NewCampaignRepository(db)
    popClient := services.NewPopClient(popBaseURL)
    campaignHandler := handlers.NewCampaignHandlerWithPop(campaignRepo, popClient)

    router.Route("/campaigns", func(r chi.Router) {
        r.With(authmw.RequirePermission(db, "campaigns.read")).Get("/search", campaignHandler.SearchCampaigns)
        r.With(authmw.RequirePermission(db, "campaigns.read")).Get("/", campaignHandler.ListCampaigns)
        r.With(authmw.RequirePermission(db, "campaigns.read")).Get("/advertiser/{advertiserID}", campaignHandler.ListCampaignsByAdvertiser)
        r.With(authmw.RequirePermission(db, "campaigns.write")).Post("/", func(w http.ResponseWriter, r *http.Request) {
            log.Println("POST /campaigns endpoint hit")
            campaignHandler.CreateCampaign(w, r)
        })
        
        r.Route("/{id}", func(r chi.Router) {
            r.With(authmw.RequirePermission(db, "campaigns.read")).Get("/", campaignHandler.GetCampaign)
			r.With(authmw.RequirePermission(db, "campaigns.read")).Get("/impressions", campaignHandler.GetCampaignImpressions)
            r.With(authmw.RequirePermission(db, "campaigns.write")).Put("/", campaignHandler.UpdateCampaign)
            r.With(authmw.RequirePermission(db, "campaigns.write")).Delete("/", campaignHandler.DeleteCampaign)
        })

    })
}