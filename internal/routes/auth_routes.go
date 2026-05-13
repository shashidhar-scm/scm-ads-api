package routes

import (
	"database/sql"

	"github.com/go-chi/chi/v5"
	"scm/internal/config"
	"scm/internal/handlers"
	"scm/internal/repository"
	"scm/internal/services"
)

func RegisterAuthRoutes(router chi.Router, db *sql.DB, cfg *config.Config, userRepo repository.UserRepository, userRoleRepo repository.UserRoleRepository) {
	mailer := &services.SMTPSender{
		Host:   cfg.SMTPHost,
		Port:   cfg.SMTPPort,
		User:   cfg.SMTPUser,
		Pass:   cfg.SMTPPassword,
		From:   cfg.SMTPFrom,
		UseTLS: cfg.SMTPUseTLS,
	}
	authHandler := handlers.NewAuthHandler(db, cfg, mailer, userRepo, userRoleRepo)

	router.Route("/auth", func(r chi.Router) {
		r.Post("/signup", authHandler.Signup)
		r.Post("/login", authHandler.Login)
		r.Post("/google", authHandler.GoogleAuth)
		r.Post("/forgot-password", authHandler.ForgotPassword)
		r.Post("/reset-password", authHandler.ResetPassword)
	})
}
