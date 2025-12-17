package handlers

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
    "golang.org/x/crypto/bcrypt"
    "scm/internal/models"
    "scm/internal/repository"
)

type UserHandler struct {
    users repository.UserRepository
}

func NewUserHandler(users repository.UserRepository) *UserHandler {
    return &UserHandler{users: users}
}

// @Tags Account
// @Summary List users
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.User
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/users/ [get]
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
    users, err := h.users.ListAll(r.Context())
    if err != nil {
        writeJSONErrorResponse(w, http.StatusInternalServerError, "list_users_failed", "Failed to list users")
        return
    }

    if users == nil {
        users = []models.User{}
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(users)
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
    if req.OldPassword == "" || req.NewPassword == "" {
        writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "old_password and new_password are required")
        return
    }
    if len(req.NewPassword) < 8 {
        writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "new_password must be at least 8 characters")
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
        writeJSONErrorResponse(w, http.StatusInternalServerError, "delete_user_failed", "Failed to delete user")
        return
    }

    writeJSONMessage(w, http.StatusOK, "User has been deleted successfully")
}
