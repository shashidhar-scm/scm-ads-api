package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"scm/internal/interfaces"
	"scm/internal/models"
)

type mockCampaignRepo struct{}

var _ interfaces.CampaignRepository = (*mockCampaignRepo)(nil)

func (m *mockCampaignRepo) Create(ctx context.Context, campaign *models.Campaign) error { return nil }
func (m *mockCampaignRepo) GetByID(ctx context.Context, id string) (*models.Campaign, error) {
	return nil, sql.ErrNoRows
}
func (m *mockCampaignRepo) List(ctx context.Context, filter interfaces.CampaignFilter) ([]*models.Campaign, error) {
	return []*models.Campaign{}, nil
}
func (m *mockCampaignRepo) Summary(ctx context.Context, filter interfaces.CampaignFilter) (*models.CampaignSummary, error) {
	return &models.CampaignSummary{}, nil
}
func (m *mockCampaignRepo) Update(ctx context.Context, id string, campaign *models.Campaign) error { return nil }
func (m *mockCampaignRepo) Delete(ctx context.Context, id string) error                         { return nil }

func TestGetCampaignNotFoundReturnsJSON(t *testing.T) {
	h := NewCampaignHandler(&mockCampaignRepo{})
	r := chi.NewRouter()
	r.Get("/campaigns/{id}", h.GetCampaign)

	req := httptest.NewRequest(http.MethodGet, "/campaigns/c1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d (%s)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json got %q", ct)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp["error"] == nil {
		t.Fatalf("expected error field, got %v", resp)
	}
}
