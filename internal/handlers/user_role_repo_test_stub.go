package handlers

import (
	"context"

	"scm/internal/models"
	"scm/internal/repository"
)

// stubUserRoleRepo is a lightweight test double for repository.UserRoleRepository.
// It keeps responses in-memory and is safe for single-threaded tests.
type stubUserRoleRepo struct {
	assignments map[string][]models.UserRoleAssignment

	isSuper bool
	isAdmin bool
	hasPerm bool

	replaceErr  error
	listErr     error
	hasPermErr  error
	anyScopeErr error
	superErr    error
	adminErr    error
	listAdvErr  error
}

var _ repository.UserRoleRepository = (*stubUserRoleRepo)(nil)

func newStubUserRoleRepo() *stubUserRoleRepo {
	return &stubUserRoleRepo{
		assignments: make(map[string][]models.UserRoleAssignment),
		hasPerm:     true,
	}
}

func (s *stubUserRoleRepo) ensureAssignments() {
	if s.assignments == nil {
		s.assignments = make(map[string][]models.UserRoleAssignment)
	}
}

func (s *stubUserRoleRepo) ReplaceUserRoles(_ context.Context, userID string, roles []models.UserRoleAssignment) error {
	if s.replaceErr != nil {
		return s.replaceErr
	}
	s.ensureAssignments()
	copyRoles := make([]models.UserRoleAssignment, len(roles))
	copy(copyRoles, roles)
	s.assignments[userID] = copyRoles
	return nil
}

func (s *stubUserRoleRepo) ListUserRoleAssignments(_ context.Context, userID string) ([]models.UserRoleAssignment, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	copyRoles := make([]models.UserRoleAssignment, len(s.assignments[userID]))
	copy(copyRoles, s.assignments[userID])
	return copyRoles, nil
}

func (s *stubUserRoleRepo) HasPermission(_ context.Context, _, _ string, _ *string) (bool, error) {
	if s.hasPermErr != nil {
		return false, s.hasPermErr
	}
	return s.hasPerm, nil
}

func (s *stubUserRoleRepo) HasPermissionInAnyScope(_ context.Context, _, _ string) (bool, error) {
	if s.anyScopeErr != nil {
		return false, s.anyScopeErr
	}
	return s.hasPerm, nil
}

func (s *stubUserRoleRepo) IsSuperAdmin(_ context.Context, _ string) (bool, error) {
	if s.superErr != nil {
		return false, s.superErr
	}
	return s.isSuper, nil
}

func (s *stubUserRoleRepo) IsAdmin(_ context.Context, _ string) (bool, error) {
	if s.adminErr != nil {
		return false, s.adminErr
	}
	return s.isAdmin, nil
}

func (s *stubUserRoleRepo) ListScopedAdvertiserIDs(_ context.Context, _ string) ([]string, error) {
	if s.listAdvErr != nil {
		return nil, s.listAdvErr
	}
	return nil, nil
}
