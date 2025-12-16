package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"scm/internal/config"
	"scm/internal/interfaces"
	"scm/internal/models"
	"scm/internal/repository"
)

type noopCreativeRepo struct{ repository.CreativeRepository }

type noopCampaignRepo struct{}

func (noopCampaignRepo) Create(ctx context.Context, campaign *models.Campaign) error { return nil }
func (noopCampaignRepo) GetByID(ctx context.Context, id string) (*models.Campaign, error) { return nil, nil }
func (noopCampaignRepo) List(ctx context.Context, filter interfaces.CampaignFilter) ([]*models.Campaign, error) {
	return nil, nil
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
