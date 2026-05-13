package routes

import (
	"database/sql"

	"github.com/go-chi/chi/v5"
	"scm/internal/handlers"
	authmw "scm/internal/middleware"
	"scm/internal/repository"
)

func RegisterRBACRoutes(router chi.Router, db *sql.DB, roleRepo repository.RoleRepository, permissionRepo repository.PermissionRepository, userRoleRepo repository.UserRoleRepository) {
	h := handlers.NewRBACHandler(roleRepo, permissionRepo, userRoleRepo)

	router.Route("/roles", func(r chi.Router) {
		r.With(authmw.RequirePermission(userRoleRepo, "roles.read")).Get("/", h.ListRoles)
		r.With(authmw.RequirePermission(userRoleRepo, "roles.write")).Post("/", h.CreateRole)
		r.Route("/{id}", func(r chi.Router) {
			r.With(authmw.RequirePermission(userRoleRepo, "roles.read")).Get("/", h.GetRole)
			r.With(authmw.RequirePermission(userRoleRepo, "roles.write")).Put("/", h.UpdateRole)
			r.With(authmw.RequirePermission(userRoleRepo, "roles.write")).Delete("/", h.DeleteRole)
			r.With(authmw.RequirePermission(userRoleRepo, "roles.read")).Get("/permissions", h.GetRolePermissions)
			r.With(authmw.RequirePermission(userRoleRepo, "roles.write")).Put("/permissions", h.SetRolePermissions)
		})
	})

	router.Route("/permissions", func(r chi.Router) {
		r.With(authmw.RequirePermission(userRoleRepo, "permissions.read")).Get("/", h.ListPermissions)
		r.With(authmw.RequirePermission(userRoleRepo, "permissions.write")).Post("/", h.CreatePermission)
		r.Route("/{id}", func(r chi.Router) {
			r.With(authmw.RequirePermission(userRoleRepo, "permissions.read")).Get("/", h.GetPermission)
			r.With(authmw.RequirePermission(userRoleRepo, "permissions.write")).Put("/", h.UpdatePermission)
			r.With(authmw.RequirePermission(userRoleRepo, "permissions.write")).Delete("/", h.DeletePermission)
		})
	})

	router.Route("/users/{id}/roles", func(r chi.Router) {
		r.With(authmw.RequirePermission(userRoleRepo, "user_roles.read")).Get("/", h.ListUserRoles)
		r.With(authmw.RequirePermission(userRoleRepo, "user_roles.write")).Put("/", h.SetUserRoles)
	})
}
