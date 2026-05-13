package routes

import (
	"context"
	"database/sql"

	"scm/internal/models"
	"scm/internal/repository"
)

type noopProjectRepo struct{}

var _ repository.ProjectRepository = (*noopProjectRepo)(nil)

func (noopProjectRepo) Upsert(context.Context, *models.Project) error { return nil }
func (noopProjectRepo) GetByID(context.Context, int) (*models.Project, error) {
	return nil, sql.ErrNoRows
}
func (noopProjectRepo) GetByName(context.Context, string) (*models.Project, error) {
	return nil, sql.ErrNoRows
}
func (noopProjectRepo) List(context.Context, int, int) ([]*models.Project, error) {
	return nil, nil
}
func (noopProjectRepo) Count(context.Context) (int, error) { return 0, nil }
func (noopProjectRepo) ListWithFilters(context.Context, repository.ProjectFilters, int, int) ([]*models.Project, error) {
	return nil, nil
}
func (noopProjectRepo) CountWithFilters(context.Context, repository.ProjectFilters) (int, error) {
	return 0, nil
}
func (noopProjectRepo) Search(context.Context, string, int, int) ([]*models.Project, int, error) {
	return nil, 0, nil
}
func (noopProjectRepo) Delete(context.Context, int) error { return nil }
