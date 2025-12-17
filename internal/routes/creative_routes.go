// internal/routes/creative_routes.go
package routes

import (
    "database/sql"
    
    "github.com/go-chi/chi/v5"
    "scm/internal/config"
    "scm/internal/handlers"
    "scm/internal/repository"
)

func RegisterPublicCreativeRoutes(router chi.Router, db *sql.DB, s3Config *config.S3Config) {
	creativeRepo := repository.NewCreativeRepository(db)
	campaignRepo := repository.NewCampaignRepository(db)
	creativeHandler := handlers.NewCreativeHandler(creativeRepo, campaignRepo, s3Config)

	router.Get("/creatives/device/{device}", creativeHandler.ListCreativesByDevice)
}

func RegisterCreativeRoutes(router chi.Router, db *sql.DB, s3Config *config.S3Config) {
    creativeRepo := repository.NewCreativeRepository(db)
    campaignRepo := repository.NewCampaignRepository(db)
    creativeHandler := handlers.NewCreativeHandler(creativeRepo, campaignRepo, s3Config)

    router.Route("/creatives", func(r chi.Router) {
        r.Get("/", creativeHandler.ListCreatives)
        r.Post("/upload", creativeHandler.UploadCreative)
        r.Get("/campaign/{campaignID}", creativeHandler.ListCreativesByCampaign)
        r.Route("/{id}", func(r chi.Router) {
            r.Get("/", creativeHandler.GetCreative)
            r.Put("/", creativeHandler.UpdateCreative)
            r.Delete("/", creativeHandler.DeleteCreative)
        })
    })
}