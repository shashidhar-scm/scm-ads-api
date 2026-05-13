package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"scm/internal/middleware"
	"scm/internal/models"
)

type noopPermissionRepo struct{}
type noopRoleRepo struct{}

func (noopPermissionRepo) Create(context.Context, *models.Permission) error { return nil }
func (noopPermissionRepo) GetByID(context.Context, string) (*models.Permission, error) {
	return nil, sql.ErrNoRows
}
func (noopPermissionRepo) List(context.Context, int, int) ([]models.Permission, error) {
	return nil, nil
}
func (noopPermissionRepo) Count(context.Context) (int, error) { return 0, nil }
func (noopPermissionRepo) Update(context.Context, string, *models.UpdatePermissionRequest) error {
	return nil
}
func (noopPermissionRepo) Delete(context.Context, string) error { return nil }

func (noopRoleRepo) Create(context.Context, *models.Role) error { return nil }
func (noopRoleRepo) GetByID(context.Context, string) (*models.Role, error) {
	return nil, sql.ErrNoRows
}
func (noopRoleRepo) List(context.Context, int, int) ([]models.Role, error) { return nil, nil }
func (noopRoleRepo) Count(context.Context) (int, error)                    { return 0, nil }
func (noopRoleRepo) Update(context.Context, string, *models.UpdateRoleRequest) error {
	return nil
}
func (noopRoleRepo) Delete(context.Context, string) error                   { return nil }
func (noopRoleRepo) SetPermissions(context.Context, string, []string) error { return nil }
func (noopRoleRepo) ListPermissionIDs(context.Context, string) ([]string, error) {
	return nil, nil
}

func TestSetUserRolesNonSuperAdminCannotChangeRoles(t *testing.T) {
	userRoles := newStubUserRoleRepo()
	userRoles.assignments["u1"] = []models.UserRoleAssignment{{RoleID: "role-old"}}

	h := NewRBACHandler(noopRoleRepo{}, noopPermissionRepo{}, userRoles)
	r := chi.NewRouter()
	r.Put("/users/{id}/roles", h.SetUserRoles)

	payload := map[string]any{"role_id": "role-new"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/users/u1/roles", bytes.NewReader(b))
	req = req.WithContext(context.WithValue(req.Context(), middleware.CtxUserID, "caller-1"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d (%s)", w.Code, w.Body.String())
	}
}

func TestSetUserRolesNonSuperAdminNoOpAllowed(t *testing.T) {
	userRoles := newStubUserRoleRepo()
	userRoles.assignments["u1"] = []models.UserRoleAssignment{{RoleID: "role-same"}}

	h := NewRBACHandler(noopRoleRepo{}, noopPermissionRepo{}, userRoles)
	r := chi.NewRouter()
	r.Put("/users/{id}/roles", h.SetUserRoles)

	payload := map[string]any{"role_id": "role-same"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/users/u1/roles", bytes.NewReader(b))
	req = req.WithContext(context.WithValue(req.Context(), middleware.CtxUserID, "caller-1"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d (%s)", w.Code, w.Body.String())
	}
}
