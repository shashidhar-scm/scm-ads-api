package routes

import (
	"context"
	"database/sql"

	"scm/internal/models"
	"scm/internal/repository"
)

type noopRoleRepo struct{}

type noopUserRoleRepo struct{}

var _ repository.RoleRepository = (*noopRoleRepo)(nil)
var _ repository.UserRoleRepository = (*noopUserRoleRepo)(nil)

func (noopRoleRepo) Create(context.Context, *models.Role) error { return nil }
func (noopRoleRepo) GetByID(context.Context, string) (*models.Role, error) {
	return nil, sql.ErrNoRows
}
func (noopRoleRepo) List(context.Context, int, int) ([]models.Role, error) {
	return nil, nil
}
func (noopRoleRepo) Count(context.Context) (int, error)                              { return 0, nil }
func (noopRoleRepo) Update(context.Context, string, *models.UpdateRoleRequest) error { return nil }
func (noopRoleRepo) Delete(context.Context, string) error                            { return nil }
func (noopRoleRepo) SetPermissions(context.Context, string, []string) error          { return nil }
func (noopRoleRepo) ListPermissionIDs(context.Context, string) ([]string, error)     { return nil, nil }

func (noopUserRoleRepo) ReplaceUserRoles(context.Context, string, []models.UserRoleAssignment) error {
	return nil
}
func (noopUserRoleRepo) ListUserRoleAssignments(context.Context, string) ([]models.UserRoleAssignment, error) {
	return nil, nil
}
func (noopUserRoleRepo) HasPermission(context.Context, string, string, *string) (bool, error) {
	return false, nil
}
func (noopUserRoleRepo) HasPermissionInAnyScope(context.Context, string, string) (bool, error) {
	return false, nil
}
func (noopUserRoleRepo) IsSuperAdmin(context.Context, string) (bool, error) { return false, nil }
func (noopUserRoleRepo) IsAdmin(context.Context, string) (bool, error)      { return false, nil }
func (noopUserRoleRepo) ListScopedAdvertiserIDs(context.Context, string) ([]string, error) {
	return nil, nil
}
