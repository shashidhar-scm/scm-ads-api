package routes

import (
	"context"
	"database/sql"
	"time"

	"scm/internal/interfaces"
	"scm/internal/models"
)

type noopCampaignRepo struct{}

var _ interfaces.CampaignRepository = (*noopCampaignRepo)(nil)

func (noopCampaignRepo) Create(context.Context, *models.Campaign) error { return nil }
func (noopCampaignRepo) GetByID(context.Context, string) (*models.Campaign, error) {
	return nil, sql.ErrNoRows
}
func (noopCampaignRepo) List(context.Context, interfaces.CampaignFilter) ([]*models.Campaign, error) {
	return nil, nil
}
func (noopCampaignRepo) Count(context.Context, interfaces.CampaignFilter) (int, error) { return 0, nil }
func (noopCampaignRepo) Search(context.Context, string, int, int, *string) ([]*models.Campaign, int, error) {
	return nil, 0, nil
}
func (noopCampaignRepo) Summary(context.Context, interfaces.CampaignFilter) (*models.CampaignSummary, error) {
	return &models.CampaignSummary{}, nil
}
func (noopCampaignRepo) ActivateScheduledStartingOn(context.Context, time.Time, string, string) (int64, error) {
	return 0, nil
}
func (noopCampaignRepo) CompleteActiveEndedBefore(context.Context, time.Time, string, string, string) (int64, error) {
	return 0, nil
}
func (noopCampaignRepo) Update(context.Context, string, *models.Campaign) error { return nil }
func (noopCampaignRepo) Delete(context.Context, string) error                   { return nil }
func (noopCampaignRepo) ListByStartDate(context.Context, time.Time) ([]*models.Campaign, error) {
	return nil, nil
}
func (noopCampaignRepo) ListByEndDate(context.Context, time.Time) ([]*models.Campaign, error) {
	return nil, nil
}
