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
			if isSuper {
				next.ServeHTTP(w, r)
				return
			}

			// First, allow via global (non-scoped) role.
			ok, err := ur.HasPermission(r.Context(), userID, permission, nil)
			if err != nil {
				writeAuthzError(w, http.StatusInternalServerError, "authz_failed", "failed to check permissions")
				return
			}
			if ok {
				next.ServeHTTP(w, r)
				return
			}

			allowedAdvertisers, _ := r.Context().Value(CtxAdvertiserIDs).([]string)
			if len(allowedAdvertisers) == 0 {
				writeAuthzError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
				return
			}

			hasScoped, err := ur.HasPermissionInAnyScope(r.Context(), userID, permission)
			if err != nil {
				writeAuthzError(w, http.StatusInternalServerError, "authz_failed", "failed to check permissions")
				return
			}
			if !hasScoped {
				writeAuthzError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
				return
			}

			selectedAdvertiserID := strings.TrimSpace(r.Header.Get("X-Advertiser-Id"))
			if selectedAdvertiserID == "" {
				writeAuthzError(w, http.StatusBadRequest, "advertiser_not_selected", "missing X-Advertiser-Id")
				return
			}
			isAllowed := false
			for _, id := range allowedAdvertisers {
				if selectedAdvertiserID == id {
					isAllowed = true
					break
				}
			}
			if !isAllowed {
				writeAuthzError(w, http.StatusForbidden, "forbidden", "invalid advertiser scope")
				return
			}

			advertiserScope := &selectedAdvertiserID
			ok, err = ur.HasPermission(r.Context(), userID, permission, advertiserScope)
			if err != nil {
				writeAuthzError(w, http.StatusInternalServerError, "authz_failed", "failed to check permissions")
				return
			}
			if !ok {
				writeAuthzError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
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
			if isSuper {
				ctx := context.WithValue(r.Context(), CtxPermissionGlobal, true)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			ok, err := ur.HasPermission(r.Context(), userID, permission, nil)
			if err != nil {
				writeAuthzError(w, http.StatusInternalServerError, "authz_failed", "failed to check permissions")
				return
			}

			allowedAdvertisers, _ := r.Context().Value(CtxAdvertiserIDs).([]string)
			if ok {
				// If the token carries advertiser scopes, prefer scoped behavior (Option A):
				// list endpoints should be filtered to those scopes instead of returning all data.
				if len(allowedAdvertisers) == 0 {
					// For advertisers.read, do not return the full advertiser list to non-super users
					// just because they have a global role. Treat as scoped with an empty set.
					if permission == "advertisers.read" {
						ctx := r.Context()
						ctx = context.WithValue(ctx, CtxAdvertiserIDs, []string{})
						ctx = context.WithValue(ctx, CtxPermissionGlobal, false)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}

					ctx := context.WithValue(r.Context(), CtxPermissionGlobal, true)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				// Fall through to scoped path below to filter advertiser_ids to ones that truly have this permission.
			}

			if len(allowedAdvertisers) == 0 {
				writeAuthzError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
				return
			}

			hasScoped, err := ur.HasPermissionInAnyScope(r.Context(), userID, permission)
			if err != nil {
				writeAuthzError(w, http.StatusInternalServerError, "authz_failed", "failed to check permissions")
				return
			}
			if !hasScoped {
				writeAuthzError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
				return
			}

			var permitted []string
			for _, advID := range allowedAdvertisers {
				id := strings.TrimSpace(advID)
				if id == "" {
					continue
				}
				adv := id
				ok, err := ur.HasPermission(r.Context(), userID, permission, &adv)
				if err != nil {
					writeAuthzError(w, http.StatusInternalServerError, "authz_failed", "failed to check permissions")
					return
				}
				if ok {
					permitted = append(permitted, id)
				}
			}
			if len(permitted) == 0 {
				writeAuthzError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, CtxAdvertiserIDs, permitted)
			ctx = context.WithValue(ctx, CtxPermissionGlobal, false)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
