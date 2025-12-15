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

func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
    users, err := h.users.ListAll(r.Context())
    if err != nil {
        http.Error(w, "Failed to list users", http.StatusInternalServerError)
        return
    }

    if users == nil {
        users = []models.User{}
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(users)
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if id == "" {
        http.Error(w, "User ID is required", http.StatusBadRequest)
        return
    }

    u, err := h.users.GetByID(r.Context(), id)
    if err != nil {
        if err.Error() == "user not found" {
            http.Error(w, "User not found", http.StatusNotFound)
            return
        }
        http.Error(w, "Failed to get user", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(u)
}

func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if id == "" {
        http.Error(w, "User ID is required", http.StatusBadRequest)
        return
    }

    var req models.UpdateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if err := h.users.UpdateProfile(r.Context(), id, &req); err != nil {
        if err.Error() == "user not found" {
            http.Error(w, "User not found", http.StatusNotFound)
            return
        }
        http.Error(w, "Failed to update user", http.StatusInternalServerError)
        return
    }

    updated, err := h.users.GetByID(r.Context(), id)
    if err != nil {
        http.Error(w, "Failed to fetch updated user", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(updated)
}

func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if id == "" {
        http.Error(w, "User ID is required", http.StatusBadRequest)
        return
    }

    var req models.ChangePasswordRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    if req.OldPassword == "" || req.NewPassword == "" {
        http.Error(w, "old_password and new_password are required", http.StatusBadRequest)
        return
    }
    if len(req.NewPassword) < 8 {
        http.Error(w, "new_password must be at least 8 characters", http.StatusBadRequest)
        return
    }

    u, err := h.users.GetByID(r.Context(), id)
    if err != nil {
        if err.Error() == "user not found" {
            http.Error(w, "User not found", http.StatusNotFound)
            return
        }
        http.Error(w, "Failed to get user", http.StatusInternalServerError)
        return
    }

    if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.OldPassword)); err != nil {
        http.Error(w, "Old password is incorrect", http.StatusUnauthorized)
        return
    }

    hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
    if err != nil {
        http.Error(w, "Failed to change password", http.StatusInternalServerError)
        return
    }

    if err := h.users.UpdatePasswordHash(r.Context(), id, string(hash)); err != nil {
        if err.Error() == "user not found" {
            http.Error(w, "User not found", http.StatusNotFound)
            return
        }
        http.Error(w, "Failed to change password", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]any{"message": "password updated"})
}

func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if id == "" {
        http.Error(w, "User ID is required", http.StatusBadRequest)
        return
    }

    if err := h.users.Delete(r.Context(), id); err != nil {
        if err.Error() == "user not found" {
            http.Error(w, "User not found", http.StatusNotFound)
            return
        }
        http.Error(w, "Failed to delete user", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}
