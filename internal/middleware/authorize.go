package middleware

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"scm/internal/repository"
)

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func writeAuthzError(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: code, Message: message})
}

func RequirePermission(db *sql.DB, permission string) func(http.Handler) http.Handler {
	ur := repository.NewUserRoleRepository(db)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, _ := r.Context().Value(CtxUserID).(string)
			if userID == "" {
				writeAuthzError(w, http.StatusUnauthorized, "unauthorized", "missing user identity")
				return
			}

			isSuper, err := ur.IsSuperAdmin(r.Context(), userID)
			if err != nil {
				writeAuthzError(w, http.StatusInternalServerError, "authz_failed", "failed to check permissions")
				return
			}
			if isSuper {
				next.ServeHTTP(w, r)
				return
			}

			// Permission must be granted by a global role (super_admin bypassed above).
			ok, err := ur.HasPermission(r.Context(), userID, permission, nil)
			if err != nil {
				writeAuthzError(w, http.StatusInternalServerError, "authz_failed", "failed to check permissions")
				return
			}
			if !ok {
				writeAuthzError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
