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

type mockAdvertiserRepo struct{}

var _ interfaces.AdvertiserRepository = (*mockAdvertiserRepo)(nil)

func (m *mockAdvertiserRepo) Create(ctx context.Context, advertiser *models.Advertiser) error { return nil }
func (m *mockAdvertiserRepo) GetByID(ctx context.Context, id string) (*models.Advertiser, error) {
	return nil, sql.ErrNoRows
}
func (m *mockAdvertiserRepo) GetByIDInSet(ctx context.Context, id string, allowedIDs []string) (*models.Advertiser, error) {
	return nil, sql.ErrNoRows
}
func (m *mockAdvertiserRepo) List(ctx context.Context, limit int, offset int) ([]models.Advertiser, error) {
	return []models.Advertiser{}, nil
}
func (m *mockAdvertiserRepo) ListByIDs(ctx context.Context, allowedIDs []string, limit int, offset int) ([]models.Advertiser, error) {
	return []models.Advertiser{}, nil
}
func (m *mockAdvertiserRepo) Count(ctx context.Context) (int, error) { return 0, nil }
func (m *mockAdvertiserRepo) CountByIDs(ctx context.Context, allowedIDs []string) (int, error) { return 0, nil }
func (m *mockAdvertiserRepo) Search(ctx context.Context, term string, limit int, offset int) ([]models.Advertiser, int, error) {
	return []models.Advertiser{}, 0, nil
}
func (m *mockAdvertiserRepo) SearchByIDs(ctx context.Context, allowedIDs []string, term string, limit int, offset int) ([]models.Advertiser, int, error) {
	return []models.Advertiser{}, 0, nil
}
func (m *mockAdvertiserRepo) Update(ctx context.Context, id string, req *models.UpdateAdvertiserRequest) error {
	return nil
}
func (m *mockAdvertiserRepo) Delete(ctx context.Context, id string) error { return nil }

func TestGetAdvertiserNotFoundJSON(t *testing.T) {
	h := NewAdvertiserHandler(&mockAdvertiserRepo{}, nil)
	r := chi.NewRouter()
	r.Get("/advertisers/{id}", h.GetAdvertiser)

	req := httptest.NewRequest(http.MethodGet, "/advertisers/a1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d (%s)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected json content-type got %q", ct)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp["error"] == nil {
		t.Fatalf("expected error field, got %v", resp)
	}
}
