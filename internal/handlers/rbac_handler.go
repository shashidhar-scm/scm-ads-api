package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"scm/internal/middleware"
	"scm/internal/models"
	"scm/internal/repository"
)

type RBACHandler struct {
	roles       repository.RoleRepository
	permissions repository.PermissionRepository
	userRoles   repository.UserRoleRepository
	v           *validator.Validate
}

func NewRBACHandler(db *sql.DB) *RBACHandler {
	return &RBACHandler{
		roles:       repository.NewRoleRepository(db),
		permissions: repository.NewPermissionRepository(db),
		userRoles:   repository.NewUserRoleRepository(db),
		v:           validator.New(),
	}
}

// @Tags Roles
// @Summary List roles
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/roles [get]
func (h *RBACHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	p, err := parsePaginationParams(r, 50, 200)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "invalid pagination parameters")
		return
	}
	total, err := h.roles.Count(r.Context())
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_roles_failed", "Failed to list roles")
		return
	}
	items, err := h.roles.List(r.Context(), p.limit, p.offset)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_roles_failed", "Failed to list roles")
		return
	}
	if items == nil {
		items = []models.Role{}
	}
	writePaginatedResponse(w, http.StatusOK, items, p.page, p.pageSize, total)
}

// @Tags Roles
// @Summary Create role
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body models.CreateRoleRequest true "Create role request"
// @Success 201 {object} models.Role
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/roles [post]
func (h *RBACHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	var req models.CreateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.v.Struct(req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", validationMessage(err))
		return
	}

	role := &models.Role{ID: uuid.NewString(), Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description), IsSystem: false, CreatedAt: time.Now().UTC()}
	if err := h.roles.Create(r.Context(), role); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			if pqErr.Code == "23505" {
				writeJSONErrorResponse(w, http.StatusBadRequest, "role_already_exists", "Role already exists")
				return
			}
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "create_role_failed", "Failed to create role")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(role)
}

// @Tags Roles
// @Summary Get role
// @Security BearerAuth
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} models.Role
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/roles/{id} [get]
func (h *RBACHandler) GetRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Role ID is required")
		return
	}
	role, err := h.roles.GetByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "role_not_found", "Role not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_role_failed", "Failed to get role")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(role)
}

// @Tags Roles
// @Summary Update role
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param body body models.UpdateRoleRequest true "Update role request"
// @Success 200 {object} models.Role
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/roles/{id} [put]
func (h *RBACHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Role ID is required")
		return
	}
	var req models.UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.roles.Update(r.Context(), id, &req); err != nil {
		if strings.Contains(err.Error(), "system roles") {
			writeJSONErrorResponse(w, http.StatusForbidden, "system_role", "System roles cannot be modified")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "update_role_failed", "Failed to update role")
		return
	}
	updated, err := h.roles.GetByID(r.Context(), id)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_role_failed", "Failed to fetch updated role")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
}

// @Tags Roles
// @Summary Delete role
// @Security BearerAuth
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/roles/{id} [delete]
func (h *RBACHandler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Role ID is required")
		return
	}
	if err := h.roles.Delete(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "system roles") {
			writeJSONErrorResponse(w, http.StatusForbidden, "system_role", "System roles cannot be deleted")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "delete_role_failed", "Failed to delete role")
		return
	}
	writeJSONMessage(w, http.StatusOK, "role deleted")
}

// @Tags Roles
// @Summary Get role permissions
// @Security BearerAuth
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/roles/{id}/permissions [get]
func (h *RBACHandler) GetRolePermissions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Role ID is required")
		return
	}
	ids, err := h.roles.ListPermissionIDs(r.Context(), id)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_role_permissions_failed", "Failed to get role permissions")
		return
	}
	if ids == nil {
		ids = []string{}
	}

	perms := make([]models.Permission, 0, len(ids))
	for _, pid := range ids {
		p, err := h.permissions.GetByID(r.Context(), pid)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "get_role_permissions_failed", "Failed to get role permissions")
			return
		}
		if p != nil {
			perms = append(perms, *p)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"permissions": perms})
}

// @Tags Roles
// @Summary Set role permissions
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param body body models.SetRolePermissionsRequest true "Set role permissions request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/roles/{id}/permissions [put]
func (h *RBACHandler) SetRolePermissions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Role ID is required")
		return
	}
	var req models.SetRolePermissionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.v.Struct(req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", validationMessage(err))
		return
	}
	if err := h.roles.SetPermissions(r.Context(), id, req.PermissionIDs); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "set_role_permissions_failed", "Failed to set role permissions")
		return
	}
	writeJSONMessage(w, http.StatusOK, "role permissions updated")
}

// @Tags Permissions
// @Summary List permissions
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/permissions [get]
func (h *RBACHandler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	p, err := parsePaginationParams(r, 50, 200)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "invalid pagination parameters")
		return
	}
	total, err := h.permissions.Count(r.Context())
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_permissions_failed", "Failed to list permissions")
		return
	}
	items, err := h.permissions.List(r.Context(), p.limit, p.offset)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_permissions_failed", "Failed to list permissions")
		return
	}
	if items == nil {
		items = []models.Permission{}
	}
	writePaginatedResponse(w, http.StatusOK, items, p.page, p.pageSize, total)
}

// @Tags Permissions
// @Summary Create permission
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body models.CreatePermissionRequest true "Create permission request"
// @Success 201 {object} models.Permission
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/permissions [post]
func (h *RBACHandler) CreatePermission(w http.ResponseWriter, r *http.Request) {
	var req models.CreatePermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.v.Struct(req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", validationMessage(err))
		return
	}
	p := &models.Permission{ID: uuid.NewString(), Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description), CreatedAt: time.Now().UTC()}
	if err := h.permissions.Create(r.Context(), p); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			if pqErr.Code == "23505" {
				writeJSONErrorResponse(w, http.StatusBadRequest, "permission_already_exists", "Permission already exists")
				return
			}
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "create_permission_failed", "Failed to create permission")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(p)
}

// @Tags Permissions
// @Summary Get permission
// @Security BearerAuth
// @Produce json
// @Param id path string true "Permission ID"
// @Success 200 {object} models.Permission
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/permissions/{id} [get]
func (h *RBACHandler) GetPermission(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Permission ID is required")
		return
	}
	p, err := h.permissions.GetByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "permission_not_found", "Permission not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_permission_failed", "Failed to get permission")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
}

// @Tags Permissions
// @Summary Update permission
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Permission ID"
// @Param body body models.UpdatePermissionRequest true "Update permission request"
// @Success 200 {object} models.Permission
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/permissions/{id} [put]
func (h *RBACHandler) UpdatePermission(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Permission ID is required")
		return
	}
	var req models.UpdatePermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.permissions.Update(r.Context(), id, &req); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "update_permission_failed", "Failed to update permission")
		return
	}
	updated, err := h.permissions.GetByID(r.Context(), id)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_permission_failed", "Failed to fetch updated permission")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
}

// @Tags Permissions
// @Summary Delete permission
// @Security BearerAuth
// @Produce json
// @Param id path string true "Permission ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/permissions/{id} [delete]
func (h *RBACHandler) DeletePermission(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Permission ID is required")
		return
	}
	if err := h.permissions.Delete(r.Context(), id); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "delete_permission_failed", "Failed to delete permission")
		return
	}
	writeJSONMessage(w, http.StatusOK, "permission deleted")
}

// @Tags UserRoles
// @Summary Get user role
// @Security BearerAuth
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/users/{id}/roles [get]
func (h *RBACHandler) ListUserRoles(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "User ID is required")
		return
	}
	roles, err := h.userRoles.ListUserRoleAssignments(r.Context(), userID)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_user_roles_failed", "Failed to list user roles")
		return
	}
	var out models.UserRole
	if len(roles) > 0 {
		ro, err := h.roles.GetByID(r.Context(), roles[0].RoleID)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "list_user_roles_failed", "Failed to list user roles")
			return
		}
		if ro != nil {
			out = models.UserRole{ID: ro.ID, Name: ro.Name}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"role": out})
}

// @Tags UserRoles
// @Summary Replace user role
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param body body models.SetUserRoleRequest true "Set user role request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/users/{id}/roles [put]
func (h *RBACHandler) SetUserRoles(w http.ResponseWriter, r *http.Request) {
	callerID, _ := r.Context().Value(middleware.CtxUserID).(string)
	if strings.TrimSpace(callerID) == "" {
		writeJSONErrorResponse(w, http.StatusUnauthorized, "unauthorized", "missing user identity")
		return
	}
	isSuper, err := h.userRoles.IsSuperAdmin(r.Context(), callerID)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "set_user_roles_failed", "Failed to set user roles")
		return
	}

	userID := chi.URLParam(r, "id")
	if userID == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "User ID is required")
		return
	}
	var req models.SetUserRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.v.Struct(req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", validationMessage(err))
		return
	}

	if !isSuper {
		current, err := h.userRoles.ListUserRoleAssignments(r.Context(), userID)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "set_user_roles_failed", "Failed to set user roles")
			return
		}
		currentRoleID := ""
		if len(current) > 0 {
			currentRoleID = strings.TrimSpace(current[0].RoleID)
		}
		requestedRoleID := strings.TrimSpace(req.RoleID)
		if currentRoleID != requestedRoleID {
			writeJSONErrorResponse(w, http.StatusForbidden, "forbidden", "You are not allowed to change the role")
			return
		}
		writeJSONMessage(w, http.StatusOK, "user roles updated")
		return
	}
	if err := h.userRoles.ReplaceUserRoles(r.Context(), userID, []models.UserRoleAssignment{{RoleID: req.RoleID}}); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "set_user_roles_failed", "Failed to set user roles")
		return
	}
	writeJSONMessage(w, http.StatusOK, "user roles updated")
}
