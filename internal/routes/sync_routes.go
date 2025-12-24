package routes

import (
	"database/sql"

	"scm/internal/handlers"
	"scm/internal/repository"
	"scm/internal/services"

	"github.com/go-chi/chi/v5"
)

func RegisterSyncRoutes(r chi.Router, db *sql.DB, client *services.CityPostConsoleClient) {
	projectRepo := repository.NewProjectRepository(db)
	deviceRepo := repository.NewDeviceRepository(db)
	syncHandler := handlers.NewSyncHandler(projectRepo, deviceRepo, client)

	r.Post("/sync/console", syncHandler.SyncConsole)
}
