package routes

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "scm/docs"
)

func RegisterSwaggerRoutes(r chi.Router) {
	// Custom swagger.json handler with dynamic version
	r.Get("/swagger/doc.json", func(w http.ResponseWriter, r *http.Request) {
		// Get version from environment or use default
		version := os.Getenv("VERSION")
		if version == "" {
			version = "1.0.0"
		}
		
		// Read the generated swagger.json file
		swaggerFile, err := ioutil.ReadFile("app/docs/swagger.json")
		if err != nil {
			// Try fallback to docs/swagger.json for local development
			swaggerFile, err = ioutil.ReadFile("docs/swagger.json")
			if err != nil {
				http.Error(w, "Swagger documentation not found", http.StatusNotFound)
				return
			}
		}
		
		// Parse the JSON
		var swaggerDoc map[string]interface{}
		if err := json.Unmarshal(swaggerFile, &swaggerDoc); err != nil {
			http.Error(w, "Error parsing swagger documentation", http.StatusInternalServerError)
			return
		}
		
		// Update the version
		if info, ok := swaggerDoc["info"].(map[string]interface{}); ok {
			info["version"] = version
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(swaggerDoc)
	})
	
	r.Get("/swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/index.html", http.StatusMovedPermanently)
	})
	r.Get("/swagger/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/index.html", http.StatusMovedPermanently)
	})
	
	// Custom Swagger handler with collapsed sections by default
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
		httpSwagger.DeepLinking(false),
		httpSwagger.DocExpansion("none"), // Collapse all by default
		httpSwagger.DomID("swagger-ui"),
		httpSwagger.PersistAuthorization(true),
	))
}
