package routes

import (
	"context"
	"database/sql"

	"scm/internal/models"
	"scm/internal/repository"
)

type noopDeviceRepo struct{}

var _ repository.DeviceRepository = (*noopDeviceRepo)(nil)

func (noopDeviceRepo) Upsert(context.Context, *models.Device) error { return nil }
func (noopDeviceRepo) GetByHostName(context.Context, string) (*models.Device, error) {
	return nil, sql.ErrNoRows
}
func (noopDeviceRepo) List(context.Context, int, int) ([]*models.Device, error) {
	return nil, nil
}
func (noopDeviceRepo) Count(context.Context) (int, error) { return 0, nil }
func (noopDeviceRepo) ListByProject(context.Context, int, int, int) ([]*models.Device, error) {
	return nil, nil
}
func (noopDeviceRepo) CountByProject(context.Context, int) (int, error) { return 0, nil }
func (noopDeviceRepo) ListWithFilters(context.Context, repository.DeviceFilters, int, int) ([]*models.Device, error) {
	return nil, nil
}
func (noopDeviceRepo) CountWithFilters(context.Context, repository.DeviceFilters) (int, error) {
	return 0, nil
}
func (noopDeviceRepo) CountByRegion(context.Context, *string, *bool) ([]repository.RegionDeviceCount, error) {
	return nil, nil
}
func (noopDeviceRepo) Search(context.Context, string, *string, *string, int, int) ([]*models.Device, int, error) {
	return nil, 0, nil
}
func (noopDeviceRepo) Recommend(context.Context, string, string, int) ([]repository.DeviceRecommendation, error) {
	return nil, nil
}
