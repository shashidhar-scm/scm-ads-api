package routes

import (
	"context"
	"database/sql"
	"time"

	"scm/internal/models"
	"scm/internal/repository"
)

type noopCreativeRepo struct{}

var _ repository.CreativeRepository = (*noopCreativeRepo)(nil)

func (noopCreativeRepo) Create(context.Context, *models.Creative) error { return nil }
func (noopCreativeRepo) GetByID(context.Context, string) (*models.Creative, error) {
	return nil, sql.ErrNoRows
}
func (noopCreativeRepo) ListAll(context.Context, int, int, *string) ([]*models.Creative, error) {
	return nil, nil
}
func (noopCreativeRepo) CountAll(context.Context, *string) (int, error) { return 0, nil }
func (noopCreativeRepo) Search(context.Context, string, int, int, *string) ([]*models.Creative, int, error) {
	return nil, 0, nil
}
func (noopCreativeRepo) ListByCampaign(context.Context, string, int, int) ([]*models.Creative, error) {
	return nil, nil
}
func (noopCreativeRepo) CountByCampaign(context.Context, string) (int, error) { return 0, nil }
func (noopCreativeRepo) ListByDevice(context.Context, string, bool, time.Time, int, int) ([]*models.Creative, error) {
	return nil, nil
}
func (noopCreativeRepo) CountByDevice(context.Context, string, bool, time.Time) (int, error) {
	return 0, nil
}
func (noopCreativeRepo) Update(context.Context, string, *models.UpdateCreativeRequest) error {
	return nil
}
func (noopCreativeRepo) Delete(context.Context, string) error { return nil }
func (noopCreativeRepo) PickNextRotationalCreative(context.Context, string, string, []string) (string, error) {
	return "", nil
}
