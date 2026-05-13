package routes

import (
	"context"
	"database/sql"

	"scm/internal/models"
	"scm/internal/repository"
)

type noopUserRepo struct{}

var _ repository.UserRepository = (*noopUserRepo)(nil)

func (noopUserRepo) Create(context.Context, *models.User) error { return nil }
func (noopUserRepo) GetByID(context.Context, string) (*models.User, error) {
	return nil, sql.ErrNoRows
}
func (noopUserRepo) GetByEmail(context.Context, string) (*models.User, error) {
	return nil, sql.ErrNoRows
}
func (noopUserRepo) GetByIdentifier(context.Context, string) (*models.User, error) {
	return nil, sql.ErrNoRows
}
func (noopUserRepo) List(context.Context, int, int) ([]models.User, error) {
	return nil, nil
}
func (noopUserRepo) Count(context.Context) (int, error) { return 0, nil }
func (noopUserRepo) ListAll(context.Context) ([]models.User, error) {
	return nil, nil
}
func (noopUserRepo) UpdateProfile(context.Context, string, *models.UpdateUserRequest) error {
	return nil
}
func (noopUserRepo) UpdatePasswordHash(context.Context, string, string) error {
	return nil
}
func (noopUserRepo) Delete(context.Context, string) error { return nil }
