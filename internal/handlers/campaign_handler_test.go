package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"scm/internal/interfaces"
	"scm/internal/models"
	"scm/internal/services"
)

type mockCampaignRepo struct{}

type mockPopAPI struct {
	byCampaignID map[string]*services.CampaignImpressions
}

func (m *mockPopAPI) CampaignImpressions(ctx context.Context, campaignID string) (*services.CampaignImpressions, error) {
	if m == nil {
		return nil, nil
	}
	return m.byCampaignID[campaignID], nil
}

func TestListCampaignsByAdvertiserReturnsJSON(t *testing.T) {
	h := NewCampaignHandler(&mockCampaignRepo{})
	r := chi.NewRouter()
	r.Get("/campaigns/advertiser/{advertiserID}", h.ListCampaignsByAdvertiser)

	// valid UUID
	req := httptest.NewRequest(http.MethodGet, "/campaigns/advertiser/550e8400-e29b-41d4-a716-446655440000", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d (%s)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json got %q", ct)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %v", resp)
	}
	if _, ok := data["campaigns"]; !ok {
		t.Fatalf("expected campaigns field, got %v", resp)
	}
}

func TestListCampaignsIncludesLifetimeImpressionsWhenRequested(t *testing.T) {
	pop := &mockPopAPI{byCampaignID: map[string]*services.CampaignImpressions{
		"c1": {CampaignID: "c1", Impressions: 123},
	}}
	h := NewCampaignHandlerWithPop(&mockCampaignRepoWithCampaigns{}, pop)
	r := chi.NewRouter()
	r.Get("/campaigns", h.ListCampaigns)

	req := httptest.NewRequest(http.MethodGet, "/campaigns?include_impressions=true", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d (%s)", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %v", resp)
	}

	campaigns, ok := data["campaigns"].([]any)
	if !ok || len(campaigns) != 1 {
		t.Fatalf("expected 1 campaign, got %T %v", data["campaigns"], data["campaigns"])
	}
	c0, ok := campaigns[0].(map[string]any)
	if !ok {
		t.Fatalf("expected campaign object, got %T", campaigns[0])
	}
	if v, ok := c0["lifetime_impressions"]; !ok {
		t.Fatalf("expected lifetime_impressions field, got %v", c0)
	} else {
		// json numbers decode as float64
		if v.(float64) != 123 {
			t.Fatalf("expected lifetime_impressions=123 got %v", v)
		}
	}
}

func TestListCampaignsDoesNotIncludeLifetimeImpressionsByDefault(t *testing.T) {
	pop := &mockPopAPI{byCampaignID: map[string]*services.CampaignImpressions{
		"c1": {CampaignID: "c1", Impressions: 123},
	}}
	h := NewCampaignHandlerWithPop(&mockCampaignRepoWithCampaigns{}, pop)
	r := chi.NewRouter()
	r.Get("/campaigns", h.ListCampaigns)

	req := httptest.NewRequest(http.MethodGet, "/campaigns", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d (%s)", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %v", resp)
	}

	campaigns, ok := data["campaigns"].([]any)
	if !ok || len(campaigns) != 1 {
		t.Fatalf("expected 1 campaign, got %T %v", data["campaigns"], data["campaigns"])
	}
	c0, ok := campaigns[0].(map[string]any)
	if !ok {
		t.Fatalf("expected campaign object, got %T", campaigns[0])
	}
	if _, ok := c0["lifetime_impressions"]; ok {
		t.Fatalf("did not expect lifetime_impressions by default, got %v", c0)
	}
}

var _ interfaces.CampaignRepository = (*mockCampaignRepo)(nil)
var _ services.PopAPI = (*mockPopAPI)(nil)

func (m *mockCampaignRepo) Create(ctx context.Context, campaign *models.Campaign) error { return nil }
func (m *mockCampaignRepo) GetByID(ctx context.Context, id string) (*models.Campaign, error) {
	return nil, sql.ErrNoRows
}
func (m *mockCampaignRepo) List(ctx context.Context, filter interfaces.CampaignFilter) ([]*models.Campaign, error) {
	return []*models.Campaign{}, nil
}
func (m *mockCampaignRepo) Count(ctx context.Context, filter interfaces.CampaignFilter) (int, error) { return 0, nil }
func (m *mockCampaignRepo) Summary(ctx context.Context, filter interfaces.CampaignFilter) (*models.CampaignSummary, error) {
	return &models.CampaignSummary{}, nil
}
func (m *mockCampaignRepo) Search(ctx context.Context, term string, limit int, offset int) ([]*models.Campaign, int, error) {
	return []*models.Campaign{}, 0, nil
}
func (m *mockCampaignRepo) ActivateScheduledStartingOn(ctx context.Context, startDate time.Time, scheduledStatus string, timeZone string) (int64, error) {
	return 0, nil
}
func (m *mockCampaignRepo) CompleteActiveEndedBefore(ctx context.Context, now time.Time, activeStatus string, completedStatus string, timeZone string) (int64, error) {
	return 0, nil
}
func (m *mockCampaignRepo) Update(ctx context.Context, id string, campaign *models.Campaign) error { return nil }
func (m *mockCampaignRepo) Delete(ctx context.Context, id string) error                         { return nil }

type mockCampaignRepoWithCampaigns struct{}

var _ interfaces.CampaignRepository = (*mockCampaignRepoWithCampaigns)(nil)

func (m *mockCampaignRepoWithCampaigns) Create(ctx context.Context, campaign *models.Campaign) error { return nil }
func (m *mockCampaignRepoWithCampaigns) GetByID(ctx context.Context, id string) (*models.Campaign, error) {
	return &models.Campaign{ID: id}, nil
}
func (m *mockCampaignRepoWithCampaigns) List(ctx context.Context, filter interfaces.CampaignFilter) ([]*models.Campaign, error) {
	return []*models.Campaign{{ID: "c1", Name: "n1"}}, nil
}
func (m *mockCampaignRepoWithCampaigns) Count(ctx context.Context, filter interfaces.CampaignFilter) (int, error) { return 1, nil }
func (m *mockCampaignRepoWithCampaigns) Summary(ctx context.Context, filter interfaces.CampaignFilter) (*models.CampaignSummary, error) {
	return &models.CampaignSummary{}, nil
}
func (m *mockCampaignRepoWithCampaigns) Search(ctx context.Context, term string, limit int, offset int) ([]*models.Campaign, int, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return []*models.Campaign{}, 0, nil
	}
	return []*models.Campaign{{ID: "c1", Name: "n1"}}, 1, nil
}
func (m *mockCampaignRepoWithCampaigns) ActivateScheduledStartingOn(ctx context.Context, startDate time.Time, scheduledStatus string, timeZone string) (int64, error) {
	return 0, nil
}
func (m *mockCampaignRepoWithCampaigns) CompleteActiveEndedBefore(ctx context.Context, now time.Time, activeStatus string, completedStatus string, timeZone string) (int64, error) {
	return 0, nil
}
func (m *mockCampaignRepoWithCampaigns) Update(ctx context.Context, id string, campaign *models.Campaign) error { return nil }
func (m *mockCampaignRepoWithCampaigns) Delete(ctx context.Context, id string) error                         { return nil }

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
