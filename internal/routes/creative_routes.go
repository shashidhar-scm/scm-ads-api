// internal/routes/creative_routes.go
package routes

import (
	"database/sql"
	"fmt"

	"github.com/go-chi/chi/v5"
	"scm/internal/config"
	"scm/internal/handlers"
	authmw "scm/internal/middleware"
	"scm/internal/repository"
)

func RegisterPublicCreativeRoutes(router chi.Router, db *sql.DB, s3Config *config.S3Config, cfg *config.Config) error {
	creativeRepo := repository.NewCreativeRepository(db)
	campaignRepo := repository.NewCampaignRepository(db)
	creativeHandler, err := handlers.NewCreativeHandler(creativeRepo, campaignRepo, s3Config, db, cfg)
	if err != nil {
		return fmt.Errorf("new creative handler: %w", err)
	}

	router.Get("/creatives/device/{device}", creativeHandler.ListCreativesByDevice)
	return nil
}

func RegisterCreativeRoutes(router chi.Router, db *sql.DB, s3Config *config.S3Config, cfg *config.Config) error {
	creativeRepo := repository.NewCreativeRepository(db)
	campaignRepo := repository.NewCampaignRepository(db)
	creativeHandler, err := handlers.NewCreativeHandler(creativeRepo, campaignRepo, s3Config, db, cfg)
	if err != nil {
		return fmt.Errorf("new creative handler: %w", err)
	}

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
	return nil
}
