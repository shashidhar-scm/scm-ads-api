// internal/routes/creative_routes.go
package routes

import (
	"database/sql"
	"fmt"

	"github.com/go-chi/chi/v5"
	"scm/internal/config"
	"scm/internal/handlers"
	"scm/internal/interfaces"
	authmw "scm/internal/middleware"
	"scm/internal/repository"
)

func RegisterPublicCreativeRoutes(router chi.Router, s3Config *config.S3Config, campaignRepo interfaces.CampaignRepository, creativeRepo repository.CreativeRepository, cfg *config.Config) error {
	creativeHandler, err := handlers.NewCreativeHandler(creativeRepo, campaignRepo, nil, s3Config, nil, cfg)
	if err != nil {
		return fmt.Errorf("new creative handler: %w", err)
	}

	router.Get("/creatives/device/{device}", creativeHandler.ListCreativesByDevice)
	return nil
}

func RegisterCreativeRoutes(router chi.Router, db *sql.DB, userRoleRepo repository.UserRoleRepository, s3Config *config.S3Config, campaignRepo interfaces.CampaignRepository, creativeRepo repository.CreativeRepository, cfg *config.Config) error {
	creativeHandler, err := handlers.NewCreativeHandler(creativeRepo, campaignRepo, userRoleRepo, s3Config, db, cfg)
	if err != nil {
		return fmt.Errorf("new creative handler: %w", err)
	}

	router.Route("/creatives", func(r chi.Router) {
		readPerm := authmw.RequirePermission(userRoleRepo, "creatives.read")
		writePerm := authmw.RequirePermission(userRoleRepo, "creatives.write")
		r.With(readPerm).Get("/search", creativeHandler.SearchCreatives)
		r.With(readPerm).Get("/", creativeHandler.ListCreatives)
		r.With(readPerm).Post("/suggestions", creativeHandler.SuggestVenues)
		r.With(writePerm).Post("/upload", creativeHandler.UploadCreative)
		r.With(readPerm).Get("/campaign/{campaignID}", creativeHandler.ListCreativesByCampaign)
		r.Route("/{id}", func(r chi.Router) {
			r.With(readPerm).Get("/", creativeHandler.GetCreative)
			r.With(writePerm).Put("/", creativeHandler.UpdateCreative)
			r.With(writePerm).Delete("/", creativeHandler.DeleteCreative)
		})
	})
	return nil
}
