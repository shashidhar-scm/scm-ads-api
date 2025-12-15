package routes

import (
    "database/sql"

    "github.com/go-chi/chi/v5"
    "scm/internal/handlers"
    "scm/internal/repository"
)

func RegisterUserRoutes(router chi.Router, db *sql.DB) {
    userRepo := repository.NewUserRepository(db)
    userHandler := handlers.NewUserHandler(userRepo)

    router.Route("/users", func(r chi.Router) {
        r.Get("/", userHandler.ListUsers)

        r.Route("/{id}", func(r chi.Router) {
            r.Get("/", userHandler.GetUser)
            r.Put("/", userHandler.UpdateUser)
            r.Put("/password", userHandler.ChangePassword)
            r.Delete("/", userHandler.DeleteUser)
        })
    })
}
