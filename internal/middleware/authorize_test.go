package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"scm/internal/models"
	"scm/internal/repository"
)

type stubUserRoleRepo struct {
	isSuper    bool
	hasPerm    bool
	isSuperErr error
	hasPermErr error
}

var _ repository.UserRoleRepository = (*stubUserRoleRepo)(nil)

func (s stubUserRoleRepo) ReplaceUserRoles(context.Context, string, []models.UserRoleAssignment) error {
	return nil
}

func (s stubUserRoleRepo) ListUserRoleAssignments(context.Context, string) ([]models.UserRoleAssignment, error) {
	return nil, nil
}

func (s stubUserRoleRepo) HasPermission(ctx context.Context, userID string, permission string, advertiserID *string) (bool, error) {
	if s.hasPermErr != nil {
		return false, s.hasPermErr
	}
	return s.hasPerm, nil
}

func (s stubUserRoleRepo) HasPermissionInAnyScope(context.Context, string, string) (bool, error) {
	return s.hasPerm, nil
}

func (s stubUserRoleRepo) IsSuperAdmin(context.Context, string) (bool, error) {
	if s.isSuperErr != nil {
		return false, s.isSuperErr
	}
	return s.isSuper, nil
}

func (s stubUserRoleRepo) IsAdmin(context.Context, string) (bool, error) {
	return false, nil
}

func (s stubUserRoleRepo) ListScopedAdvertiserIDs(context.Context, string) ([]string, error) {
	return nil, nil
}

func TestRequirePermission_SuperAdminBypass(t *testing.T) {
	repo := stubUserRoleRepo{isSuper: true}

	h := RequirePermission(repo, "roles.read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), CtxUserID, "user-1"))
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestRequirePermission_ForbiddenWhenNoPermission(t *testing.T) {
	repo := stubUserRoleRepo{isSuper: false, hasPerm: false}

	h := RequirePermission(repo, "roles.read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), CtxUserID, "user-1"))
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rr.Code)
	}
}

func TestRequirePermission_AllowsWhenPermissionGranted(t *testing.T) {
	repo := stubUserRoleRepo{isSuper: false, hasPerm: true}

	h := RequirePermission(repo, "roles.read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), CtxUserID, "user-1"))
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}
