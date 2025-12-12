// internal/routes/routes.go
package routes

import (
    "database/sql"
    "net/http"
    
    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "scm/internal/config"
)

func SetupRoutes(db *sql.DB, cfg *config.Config, s3Config *config.S3Config) *chi.Mux {
    r := chi.NewRouter()
    
    // Middleware
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    
    // Health check
    r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    })
    
    // API v1 routes
    r.Route("/api/v1", func(r chi.Router) {
        // Register campaign routes
        RegisterCampaignRoutes(r, db)  // Correct order: router first, then db
        // Register advertiser routes
        RegisterAdvertiserRoutes(r, db)
		RegisterCreativeRoutes(r, db, s3Config)

    })
    
    return r
}