// internal/routes/routes.go
package routes

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	
	"github.com/go-chi/cors"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"scm/internal/config"
	authmw "scm/internal/middleware"
)

func SetupRoutes(db *sql.DB, cfg *config.Config, s3Config *config.S3Config) *chi.Mux {
	r := chi.NewRouter()
	
	// Middleware
	allowedOrigins := []string{"*"}
	if raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS")); raw != "" {
		parts := strings.Split(raw, ",")
		allowedOrigins = allowedOrigins[:0]
		for _, p := range parts {
			o := strings.TrimSpace(p)
			if o != "" {
				allowedOrigins = append(allowedOrigins, o)
			}
		}
		if len(allowedOrigins) == 0 {
			allowedOrigins = []string{"*"}
		}
	}

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders: []string{"Link"},
		AllowCredentials: false,
		MaxAge: 300,
	}))

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "Application Up and running"})
	})

	// Health check
    r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
        type dbStatus struct {
            Status string `json:"status"`
            Error  string `json:"error,omitempty"`
        }
        type healthResponse struct {
            Status string   `json:"status"`
            DB     dbStatus `json:"db"`
        }

        ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
        defer cancel()

        resp := healthResponse{
            Status: "ok",
            DB:     dbStatus{Status: "ok"},
        }

        if err := db.PingContext(ctx); err != nil {
            resp.Status = "degraded"
            resp.DB.Status = "down"
            resp.DB.Error = err.Error()
            w.WriteHeader(http.StatusServiceUnavailable)
        } else {
            w.WriteHeader(http.StatusOK)
        }

        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(resp)
    })
    
    // API v1 routes
    r.Route("/api/v1", func(r chi.Router) {
        // Public auth routes
        RegisterAuthRoutes(r, db, cfg)
        RegisterUserRoutes(r, db)

        r.Get("/debug/env", func(w http.ResponseWriter, r *http.Request) {
            sanitizeDatabaseURL := func(raw string) string {
                if raw == "" {
                    return ""
                }
                u, err := url.Parse(raw)
                if err != nil {
                    return ""
                }
                if u.User != nil {
                    username := u.User.Username()
                    if username != "" {
                        u.User = url.UserPassword(username, "***")
                    }
                }
                return u.String()
            }

            getEnvOrEmpty := func(key string) string {
                v, _ := os.LookupEnv(key)
                return v
            }

            resp := map[string]any{
                "config": map[string]any{
                    "environment":  cfg.Environment,
                    "port":         cfg.Port,
                    "database_url": sanitizeDatabaseURL(cfg.DatabaseURL),
                },
                "env": map[string]any{
                    "DATABASE_URL":              sanitizeDatabaseURL(getEnvOrEmpty("DATABASE_URL")),
                    "PSQL_HOST":                 getEnvOrEmpty("PSQL_HOST"),
                    "PSQL_PORT":                 getEnvOrEmpty("PSQL_PORT"),
                    "PSQL_USER":                 getEnvOrEmpty("PSQL_USER"),
                    "PSQL_DB_NAME":              getEnvOrEmpty("PSQL_DB_NAME"),
                    "PSQL_PASSWORD":             "***",
                    "JWT_SECRET":                "***",
                    "AWS_REGION":                getEnvOrEmpty("AWS_REGION"),
                    "S3_BUCKET_NAME":            getEnvOrEmpty("S3_BUCKET_NAME"),
                    "CREATIVE_PUBLIC_BASE_URL":  getEnvOrEmpty("CREATIVE_PUBLIC_BASE_URL"),
                    "AWS_ACCESS_KEY_ID":         "***",
                    "AWS_SECRET_ACCESS_KEY":     "***",
                },
            }

            w.Header().Set("Content-Type", "application/json")
            _ = json.NewEncoder(w).Encode(resp)
        })

        // Protected routes
        r.Group(func(r chi.Router) {
            r.Use(authmw.JWTAuth(cfg.JWTSecret))

            // Register campaign routes
            RegisterCampaignRoutes(r, db)  // Correct order: router first, then db
            // Register advertiser routes
            RegisterAdvertiserRoutes(r, db)
            RegisterCreativeRoutes(r, db, s3Config)

        })
    })
    
    return r
}