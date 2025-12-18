package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"scm/internal/config"
	"scm/internal/interfaces"
	"scm/internal/models"
)

type noopCreativeRepo struct{}

func (noopCreativeRepo) Create(ctx context.Context, creative *models.Creative) error { return nil }
func (noopCreativeRepo) GetByID(ctx context.Context, id string) (*models.Creative, error) {
	return nil, nil
}
func (noopCreativeRepo) ListAll(ctx context.Context, limit int, offset int) ([]*models.Creative, error) {
	return []*models.Creative{}, nil
}
func (noopCreativeRepo) CountAll(ctx context.Context) (int, error) { return 0, nil }
func (noopCreativeRepo) ListByCampaign(ctx context.Context, campaignID string, limit int, offset int) ([]*models.Creative, error) {
	return []*models.Creative{}, nil
}
func (noopCreativeRepo) CountByCampaign(ctx context.Context, campaignID string) (int, error) { return 0, nil }
func (noopCreativeRepo) ListByDevice(ctx context.Context, device string, activeNow bool, now time.Time, limit int, offset int) ([]*models.Creative, error) {
	return []*models.Creative{}, nil
}
func (noopCreativeRepo) CountByDevice(ctx context.Context, device string, activeNow bool, now time.Time) (int, error) {
	return 0, nil
}
func (noopCreativeRepo) Update(ctx context.Context, id string, req *models.UpdateCreativeRequest) error { return nil }
func (noopCreativeRepo) Delete(ctx context.Context, id string) error { return nil }

type noopCampaignRepo struct{}

func (noopCampaignRepo) Create(ctx context.Context, campaign *models.Campaign) error { return nil }
func (noopCampaignRepo) GetByID(ctx context.Context, id string) (*models.Campaign, error) { return nil, nil }
func (noopCampaignRepo) List(ctx context.Context, filter interfaces.CampaignFilter) ([]*models.Campaign, error) {
	return nil, nil
}
func (noopCampaignRepo) Count(ctx context.Context, filter interfaces.CampaignFilter) (int, error) {
	return 0, nil
}
func (noopCampaignRepo) Summary(ctx context.Context, filter interfaces.CampaignFilter) (*models.CampaignSummary, error) {
	return &models.CampaignSummary{}, nil
}
func (noopCampaignRepo) ActivateScheduledStartingOn(ctx context.Context, startDate time.Time, scheduledStatus string, timeZone string) (int64, error) {
	return 0, nil
}
func (noopCampaignRepo) CompleteActiveEndedBefore(ctx context.Context, now time.Time, activeStatus string, completedStatus string, timeZone string) (int64, error) {
	return 0, nil
}
func (noopCampaignRepo) Update(ctx context.Context, id string, campaign *models.Campaign) error { return nil }
func (noopCampaignRepo) Delete(ctx context.Context, id string) error { return nil }

func TestUploadCreativeMissingCampaignIDReturnsJSON(t *testing.T) {
	h := NewCreativeHandler(&noopCreativeRepo{}, noopCampaignRepo{}, &config.S3Config{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/creatives/upload", nil)
	// No multipart => ParseMultipartForm fails => JSON error
	w := httptest.NewRecorder()
	h.UploadCreative(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d (%s)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json got %q", ct)
	}
}
