package routes

import (
	"database/sql"

	"github.com/go-chi/chi/v5"
	"scm/internal/config"
	"scm/internal/handlers"
	"scm/internal/services"
)

func RegisterAuthRoutes(router chi.Router, db *sql.DB, cfg *config.Config) {
	mailer := &services.SMTPSender{
		Host:  cfg.SMTPHost,
		Port:  cfg.SMTPPort,
		User:  cfg.SMTPUser,
		Pass:  cfg.SMTPPassword,
		From:  cfg.SMTPFrom,
		UseTLS: cfg.SMTPUseTLS,
	}
	authHandler := handlers.NewAuthHandler(db, cfg, mailer)

	router.Route("/auth", func(r chi.Router) {
		r.Post("/signup", authHandler.Signup)
		r.Post("/login", authHandler.Login)
		r.Post("/forgot-password", authHandler.ForgotPassword)
		r.Post("/reset-password", authHandler.ResetPassword)
	})
}
