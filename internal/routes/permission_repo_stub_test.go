package routes

import (
	"context"
	"database/sql"

	"scm/internal/models"
	"scm/internal/repository"
)

type noopPermissionRepo struct{}

var _ repository.PermissionRepository = (*noopPermissionRepo)(nil)

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
