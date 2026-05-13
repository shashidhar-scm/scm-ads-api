package routes

import (
	"database/sql"

	"github.com/go-chi/chi/v5"
	"scm/internal/handlers"
	authmw "scm/internal/middleware"
	"scm/internal/repository"
)

func RegisterUserRoutes(router chi.Router, db *sql.DB, userRoleRepo repository.UserRoleRepository, userRepo repository.UserRepository) {
	userHandler := handlers.NewUserHandler(userRepo, db, userRoleRepo)

	router.Route("/users", func(r chi.Router) {
		r.With(authmw.RequirePermission(userRoleRepo, "users.read")).Get("/", userHandler.ListUsers)

		r.Route("/{id}", func(r chi.Router) {
			r.With(authmw.RequirePermission(userRoleRepo, "users.read")).Get("/", userHandler.GetUser)
			r.With(authmw.RequirePermission(userRoleRepo, "users.write")).Put("/", userHandler.UpdateUser)
			r.With(authmw.RequirePermission(userRoleRepo, "users.write")).Put("/password", userHandler.ChangePassword)
			r.With(authmw.RequirePermission(userRoleRepo, "users.write")).Delete("/", userHandler.DeleteUser)
		})
	})
}
