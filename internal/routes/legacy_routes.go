package routes

import (
	"database/sql"

	"scm/internal/handlers"
	"scm/internal/repository"

	"github.com/go-chi/chi/v5"
)

func RegisterLegacyRoutes(r chi.Router, db *sql.DB, replicatorDB *sql.DB, creativeRepo repository.CreativeRepository, placeExchangeRepo repository.PlaceExchangeTokenRepository, revisionRepo repository.LegacyRevisionRepository) {
	h := handlers.NewLegacyHandler(db, replicatorDB, creativeRepo, placeExchangeRepo, revisionRepo)

	// CouchDB-style individual document endpoints
	r.Get("/posters/{id}", h.GetPosterByID)
	r.Get("/adposters/{id}", h.GetAdPosterByID)
	r.Get("/themes/{id}", h.GetThemeByID)
	r.Get("/loop_posters/{id}", h.GetLoopPosterByID)

	// CouchDB-style _all_docs endpoints
	r.Get("/posters/_all_docs", h.GetAllPosters)
	r.Get("/adposters/_all_docs", h.GetAllAdPosters)
	r.Get("/themes/_all_docs", h.GetAllThemes)

	// RESTful endpoint for loop data by device
	r.Get("/theme/*", h.GetLoopByDevice)

	r.Route("/scm-api", func(r chi.Router) {
		r.Get("/theme", h.GetTheme)
		r.Get("/getContent", h.GetContent)
		r.Get("/getLoopPostersWeb", h.GetLoopPostersWeb)
	})
}
