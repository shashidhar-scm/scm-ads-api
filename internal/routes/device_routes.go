package routes

import (
	"github.com/go-chi/chi/v5"
	"scm/internal/config"
	"scm/internal/handlers"
)

func RegisterDeviceRoutes(router chi.Router, cfg *config.Config) {
	h := handlers.NewDeviceHandlerFromConfig(cfg)
	router.Get("/devices", h.ListDevices)
}
