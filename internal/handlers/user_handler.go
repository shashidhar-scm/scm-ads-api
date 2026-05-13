package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	"scm/internal/middleware"
	"scm/internal/models"
	"scm/internal/repository"
)

type UserHandler struct {
	users     repository.UserRepository
	v         *validator.Validate
	db        *sql.DB
	userRoles repository.UserRoleRepository
}

func NewUserHandler(users repository.UserRepository, db *sql.DB, userRoles repository.UserRoleRepository) *UserHandler {
	v := validator.New()
	_ = v.RegisterValidation("strongpassword", strongPassword)
	return &UserHandler{users: users, v: v, db: db, userRoles: userRoles}
}

// @Tags Account
// @Summary List users
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.User
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/users/ [get]
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	p, err := parsePaginationParams(r, 50, 200)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "invalid pagination parameters")
		return
	}

	total, err := h.users.Count(r.Context())
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_users_failed", "Failed to list users")
		return
	}

	users, err := h.users.List(r.Context(), p.limit, p.offset)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_users_failed", "Failed to list users")
		return
	}

	if users == nil {
		users = []models.User{}
	}

	if h.db != nil && len(users) > 0 {
		ids := make([]string, 0, len(users))
		for _, u := range users {
			ids = append(ids, u.ID)
		}

		rows, err := h.db.QueryContext(r.Context(), `
			SELECT ur.user_id, ro.name
			FROM user_roles ur
			JOIN roles ro ON ro.id = ur.role_id
			WHERE ur.user_id = ANY($1)
			ORDER BY ur.created_at ASC
		`, pq.Array(ids))
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "list_users_failed", "Failed to list users")
			return
		}
		defer rows.Close()

		byUser := make(map[string]string, len(users))
		for rows.Next() {
			var userID string
			var roleName string
			if err := rows.Scan(&userID, &roleName); err != nil {
				writeJSONErrorResponse(w, http.StatusInternalServerError, "list_users_failed", "Failed to list users")
				return
			}
			if _, ok := byUser[userID]; !ok {
				byUser[userID] = roleName
			}
		}
		if err := rows.Err(); err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "list_users_failed", "Failed to list users")
			return
		}

		for i := range users {
			users[i].Role = byUser[users[i].ID]
		}
	}

	writePaginatedResponse(w, http.StatusOK, users, p.page, p.pageSize, total)
}

// @Tags Account
// @Summary Get user
// @Security BearerAuth
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} models.User
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/users/{id}/ [get]
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "User ID is required")
		return
	}

	u, err := h.users.GetByID(r.Context(), id)
	if err != nil {
		if err.Error() == "user not found" {
			writeJSONErrorResponse(w, http.StatusNotFound, "user_not_found", "User not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_user_failed", "Failed to get user")
		return
	}

	if h.db != nil {
		rows, err := h.db.QueryContext(r.Context(), `
			SELECT ro.name
			FROM user_roles ur
			JOIN roles ro ON ro.id = ur.role_id
			WHERE ur.user_id = $1
			ORDER BY ur.created_at ASC
		`, u.ID)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "get_user_failed", "Failed to get user")
			return
		}
		defer rows.Close()

		role := ""
		for rows.Next() {
			var roleName string
			if err := rows.Scan(&roleName); err != nil {
				writeJSONErrorResponse(w, http.StatusInternalServerError, "get_user_failed", "Failed to get user")
				return
			}
			role = roleName
			break
		}
		if err := rows.Err(); err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "get_user_failed", "Failed to get user")
			return
		}
		u.Role = role
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(u)
}

// @Tags Account
// @Summary Update user
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param body body models.UpdateUserRequest true "Update user request"
// @Success 200 {object} models.User
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/users/{id}/ [put]
func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "User ID is required")
		return
	}

	var req models.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.UserName != nil {
		if err := h.v.Var(*req.UserName, "alphanum"); err != nil {
			writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "user_name must contain only letters and numbers")
			return
		}
	}

	if req.Status != nil {
		v := strings.ToLower(strings.TrimSpace(*req.Status))
		if v != "pending" && v != "active" {
			writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "status must be pending or active")
			return
		}
		req.Status = &v

		if h.userRoles == nil {
			writeJSONErrorResponse(w, http.StatusForbidden, "forbidden", "insufficient permissions")
			return
		}
		currentUserID, _ := r.Context().Value(middleware.CtxUserID).(string)
		if strings.TrimSpace(currentUserID) == "" {
			writeJSONErrorResponse(w, http.StatusUnauthorized, "unauthorized", "missing user identity")
			return
		}
		isSuper, err := h.userRoles.IsSuperAdmin(r.Context(), currentUserID)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "update_user_failed", "Failed to update user")
			return
		}
		if !isSuper {
			writeJSONErrorResponse(w, http.StatusForbidden, "forbidden", "insufficient permissions")
			return
		}
	}

	if err := h.users.UpdateProfile(r.Context(), id, &req); err != nil {
		if err.Error() == "user not found" {
			writeJSONErrorResponse(w, http.StatusNotFound, "user_not_found", "User not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "update_user_failed", "Failed to update user")
		return
	}

	updated, err := h.users.GetByID(r.Context(), id)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_user_failed", "Failed to fetch updated user")
		return
	}

	if h.db != nil {
		rows, err := h.db.QueryContext(r.Context(), `
            SELECT ro.name
            FROM user_roles ur
            JOIN roles ro ON ro.id = ur.role_id
            WHERE ur.user_id = $1
            ORDER BY ur.created_at ASC
        `, updated.ID)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "get_user_failed", "Failed to fetch updated user")
			return
		}
		defer rows.Close()

		role := ""
		for rows.Next() {
			var roleName string
			if err := rows.Scan(&roleName); err != nil {
				writeJSONErrorResponse(w, http.StatusInternalServerError, "get_user_failed", "Failed to fetch updated user")
				return
			}
			role = roleName
			break
		}
		if err := rows.Err(); err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "get_user_failed", "Failed to fetch updated user")
			return
		}
		updated.Role = role
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
}

// @Tags Account
// @Summary Change password
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param body body models.ChangePasswordRequest true "Change password request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/users/{id}/password [put]
func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "User ID is required")
		return
	}

	var req models.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.v.Struct(req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", validationMessage(err))
		return
	}

	u, err := h.users.GetByID(r.Context(), id)
	if err != nil {
		if err.Error() == "user not found" {
			writeJSONErrorResponse(w, http.StatusNotFound, "user_not_found", "User not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_user_failed", "Failed to get user")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.OldPassword)); err != nil {
		writeJSONErrorResponse(w, http.StatusUnauthorized, "invalid_password", "Old password is incorrect")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "hash_failed", "Failed to change password")
		return
	}

	if err := h.users.UpdatePasswordHash(r.Context(), id, string(hash)); err != nil {
		if err.Error() == "user not found" {
			writeJSONErrorResponse(w, http.StatusNotFound, "user_not_found", "User not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "change_password_failed", "Failed to change password")
		return
	}

	writeJSONMessage(w, http.StatusOK, "password updated")
}

// @Tags Account
// @Summary Delete user
// @Security BearerAuth
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/users/{id}/ [delete]
func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "User ID is required")
		return
	}

	if err := h.users.Delete(r.Context(), id); err != nil {
		if err.Error() == "user not found" {
			writeJSONErrorResponse(w, http.StatusNotFound, "user_not_found", "User not found")
			return
		}
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			// 23503 = foreign_key_violation (e.g. user is referenced by user_roles or other tables)
			if string(pqErr.Code) == "23503" {
				writeJSONErrorResponse(w, http.StatusConflict, "user_in_use", "User cannot be deleted because it is referenced by other records")
				return
			}
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "delete_user_failed", "Failed to delete user")
		return
	}

	writeJSONMessage(w, http.StatusOK, "User has been deleted successfully")
}
