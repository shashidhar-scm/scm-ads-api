// internal/routes/creative_routes.go
package routes

import (
    "database/sql"
    
    "github.com/go-chi/chi/v5"
    "scm/internal/config"
    "scm/internal/handlers"
	authmw "scm/internal/middleware"
    "scm/internal/repository"
)

func RegisterPublicCreativeRoutes(router chi.Router, db *sql.DB, s3Config *config.S3Config) {
	creativeRepo := repository.NewCreativeRepository(db)
	campaignRepo := repository.NewCampaignRepository(db)
	creativeHandler := handlers.NewCreativeHandler(creativeRepo, campaignRepo, s3Config, db)

	router.Get("/creatives/device/{device}", creativeHandler.ListCreativesByDevice)
}

func RegisterCreativeRoutes(router chi.Router, db *sql.DB, s3Config *config.S3Config) {
    creativeRepo := repository.NewCreativeRepository(db)
    campaignRepo := repository.NewCampaignRepository(db)
    creativeHandler := handlers.NewCreativeHandler(creativeRepo, campaignRepo, s3Config, db)

    router.Route("/creatives", func(r chi.Router) {
		r.With(authmw.RequirePermission(db, "creatives.read")).Get("/search", creativeHandler.SearchCreatives)
		r.With(authmw.RequirePermission(db, "creatives.read")).Get("/", creativeHandler.ListCreatives)
		r.With(authmw.RequirePermission(db, "creatives.read")).Post("/suggestions", creativeHandler.SuggestVenues)
        r.With(authmw.RequirePermission(db, "creatives.write")).Post("/upload", creativeHandler.UploadCreative)
        r.With(authmw.RequirePermission(db, "creatives.read")).Get("/campaign/{campaignID}", creativeHandler.ListCreativesByCampaign)
        r.Route("/{id}", func(r chi.Router) {
            r.With(authmw.RequirePermission(db, "creatives.read")).Get("/", creativeHandler.GetCreative)
            r.With(authmw.RequirePermission(db, "creatives.write")).Put("/", creativeHandler.UpdateCreative)
            r.With(authmw.RequirePermission(db, "creatives.write")).Delete("/", creativeHandler.DeleteCreative)
        })
    })
}