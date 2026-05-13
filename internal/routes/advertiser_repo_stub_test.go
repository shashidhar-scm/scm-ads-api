package routes

import (
	"context"
	"database/sql"

	"scm/internal/interfaces"
	"scm/internal/models"
)

type noopAdvertiserRepo struct{}

var _ interfaces.AdvertiserRepository = (*noopAdvertiserRepo)(nil)

func (noopAdvertiserRepo) Create(context.Context, *models.Advertiser) error { return nil }
func (noopAdvertiserRepo) GetByID(context.Context, string) (*models.Advertiser, error) {
	return nil, sql.ErrNoRows
}
func (noopAdvertiserRepo) GetByIDInSet(context.Context, string, []string) (*models.Advertiser, error) {
	return nil, sql.ErrNoRows
}
func (noopAdvertiserRepo) List(context.Context, int, int) ([]models.Advertiser, error) {
	return nil, nil
}
func (noopAdvertiserRepo) ListByIDs(context.Context, []string, int, int) ([]models.Advertiser, error) {
	return nil, nil
}
func (noopAdvertiserRepo) Count(context.Context) (int, error)                { return 0, nil }
func (noopAdvertiserRepo) CountByIDs(context.Context, []string) (int, error) { return 0, nil }
func (noopAdvertiserRepo) Search(context.Context, string, int, int) ([]models.Advertiser, int, error) {
	return nil, 0, nil
}
func (noopAdvertiserRepo) SearchByIDs(context.Context, []string, string, int, int) ([]models.Advertiser, int, error) {
	return nil, 0, nil
}
func (noopAdvertiserRepo) Update(context.Context, string, *models.UpdateAdvertiserRequest) error {
	return nil
}
func (noopAdvertiserRepo) Delete(context.Context, string) error { return nil }
