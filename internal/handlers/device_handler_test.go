package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"scm/internal/models"
	"scm/internal/repository"
)

type mockDeviceRepo struct {
	city   *string
	region *string
	recCity string
	recRegion string
	recLimit int
	limit  int
	offset int
	test   *bool
}

var _ repository.DeviceRepository = (*mockDeviceRepo)(nil)

func (m *mockDeviceRepo) Upsert(ctx context.Context, device *models.Device) error { return nil }
func (m *mockDeviceRepo) GetByHostName(ctx context.Context, hostName string) (*models.Device, error) {
	return nil, sql.ErrNoRows
}
func (m *mockDeviceRepo) List(ctx context.Context, limit int, offset int) ([]*models.Device, error) {
	return []*models.Device{}, nil
}
func (m *mockDeviceRepo) Count(ctx context.Context) (int, error) { return 0, nil }
func (m *mockDeviceRepo) ListByProject(ctx context.Context, projectID int, limit int, offset int) ([]*models.Device, error) {
	return []*models.Device{}, nil
}
func (m *mockDeviceRepo) CountByProject(ctx context.Context, projectID int) (int, error) { return 0, nil }
func (m *mockDeviceRepo) ListWithFilters(ctx context.Context, filters repository.DeviceFilters, limit int, offset int) ([]*models.Device, error) {
	return []*models.Device{}, nil
}
func (m *mockDeviceRepo) CountWithFilters(ctx context.Context, filters repository.DeviceFilters) (int, error) { return 0, nil }

func (m *mockDeviceRepo) CountByRegion(ctx context.Context, city *string, test *bool) ([]repository.RegionDeviceCount, error) {
	m.test = test
	return []repository.RegionDeviceCount{}, nil
}
func (m *mockDeviceRepo) Search(ctx context.Context, term string, city *string, region *string, limit int, offset int) ([]*models.Device, int, error) {
	m.city = city
	m.region = region
	m.limit = limit
	m.offset = offset
	return []*models.Device{}, 0, nil
}

func (m *mockDeviceRepo) Recommend(ctx context.Context, city string, region string, limit int) ([]repository.DeviceRecommendation, error) {
	m.recCity = city
	m.recRegion = region
	m.recLimit = limit
	return []repository.DeviceRecommendation{}, nil
}

func TestDeviceSearchCityRegionAreApplied(t *testing.T) {
	repo := &mockDeviceRepo{}
	h := NewDeviceReadHandler(repo)

	r := chi.NewRouter()
	r.Get("/devices/search", h.Search)

	req := httptest.NewRequest(http.MethodGet, "/devices/search?query=abc&city=NYC&region=R1&page=2&page_size=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d (%s)", w.Code, w.Body.String())
	}
	if repo.city == nil || *repo.city != "NYC" {
		t.Fatalf("expected city NYC, got %+v", repo.city)
	}
	if repo.region == nil || *repo.region != "R1" {
		t.Fatalf("expected region R1, got %+v", repo.region)
	}
	if repo.limit != 10 {
		t.Fatalf("expected limit 10 got %d", repo.limit)
	}
	if repo.offset != 10 {
		t.Fatalf("expected offset 10 got %d", repo.offset)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp["data"] == nil {
		t.Fatalf("expected data, got %v", resp)
	}
	if resp["pagination"] == nil {
		t.Fatalf("expected pagination, got %v", resp)
	}
}

func TestDeviceCountByRegionTestFilterIsApplied(t *testing.T) {
	repo := &mockDeviceRepo{}
	h := NewDeviceReadHandler(repo)

	r := chi.NewRouter()
	r.Get("/devices/counts/regions", h.CountByRegion)

	req := httptest.NewRequest(http.MethodGet, "/devices/counts/regions?test=true", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d (%s)", w.Code, w.Body.String())
	}
	if repo.test == nil || *repo.test != true {
		t.Fatalf("expected test=true, got %+v", repo.test)
	}
}

func TestDeviceRecommendRequiresCityAndRegion(t *testing.T) {
	repo := &mockDeviceRepo{}
	h := NewDeviceReadHandler(repo)

	r := chi.NewRouter()
	r.Get("/devices/recommendations", h.Recommend)

	{
		req := httptest.NewRequest(http.MethodGet, "/devices/recommendations?region=R1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 got %d (%s)", w.Code, w.Body.String())
		}
	}
	{
		req := httptest.NewRequest(http.MethodGet, "/devices/recommendations?city=NYC", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 got %d (%s)", w.Code, w.Body.String())
		}
	}
}

func TestDeviceRecommendPassesArgsToRepo(t *testing.T) {
	repo := &mockDeviceRepo{}
	h := NewDeviceReadHandler(repo)

	r := chi.NewRouter()
	r.Get("/devices/recommendations", h.Recommend)

	req := httptest.NewRequest(http.MethodGet, "/devices/recommendations?city=NYC&region=R1&limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d (%s)", w.Code, w.Body.String())
	}
	if repo.recCity != "NYC" {
		t.Fatalf("expected recCity NYC got %q", repo.recCity)
	}
	if repo.recRegion != "R1" {
		t.Fatalf("expected recRegion R1 got %q", repo.recRegion)
	}
	if repo.recLimit != 10 {
		t.Fatalf("expected recLimit 10 got %d", repo.recLimit)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp["data"] == nil {
		t.Fatalf("expected data, got %v", resp)
	}
}
