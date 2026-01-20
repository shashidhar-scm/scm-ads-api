package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

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
			isAdmin, err := ur.IsAdmin(r.Context(), userID)
			if err != nil {
				writeAuthzError(w, http.StatusInternalServerError, "authz_failed", "failed to check permissions")
				return
			}
			if isSuper || isAdmin {
				next.ServeHTTP(w, r)
				return
			}

			// Permission must be granted by a global role (super_admin/admin bypassed above).
			ok, err := ur.HasPermission(r.Context(), userID, permission, nil)
			if err != nil {
				writeAuthzError(w, http.StatusInternalServerError, "authz_failed", "failed to check permissions")
				return
			}
			if !ok {
				writeAuthzError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
				return
			}

			// For advertiser users, scope is determined by advertiser ownership (advertisers.created_by).
			selectedAdvertiserID := strings.TrimSpace(r.Header.Get("X-Advertiser-Id"))
			if selectedAdvertiserID == "" {
				writeAuthzError(w, http.StatusBadRequest, "advertiser_not_selected", "missing X-Advertiser-Id")
				return
			}

			var owned bool
			if err := db.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM advertisers WHERE id = $1 AND created_by = $2)`, selectedAdvertiserID, userID).Scan(&owned); err != nil {
				writeAuthzError(w, http.StatusInternalServerError, "authz_failed", "failed to validate advertiser scope")
				return
			}
			if !owned {
				writeAuthzError(w, http.StatusForbidden, "forbidden", "invalid advertiser scope")
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, CtxAdvertiserID, selectedAdvertiserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequirePermissionNoAdvertiserSelection(db *sql.DB, permission string) func(http.Handler) http.Handler {
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
			isAdmin, err := ur.IsAdmin(r.Context(), userID)
			if err != nil {
				writeAuthzError(w, http.StatusInternalServerError, "authz_failed", "failed to check permissions")
				return
			}
			if isSuper || isAdmin {
				ctx := context.WithValue(r.Context(), CtxPermissionGlobal, true)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			ok, err := ur.HasPermission(r.Context(), userID, permission, nil)
			if err != nil {
				writeAuthzError(w, http.StatusInternalServerError, "authz_failed", "failed to check permissions")
				return
			}
			if !ok {
				writeAuthzError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
				return
			}

			ctx := context.WithValue(r.Context(), CtxPermissionGlobal, true)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
