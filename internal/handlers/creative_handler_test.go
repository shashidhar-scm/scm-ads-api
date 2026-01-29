package handlers

import (
	"context"
	"encoding/json"
	"mime/multipart"
	"net/textproto"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"scm/internal/config"
	"scm/internal/interfaces"
	"scm/internal/models"
)

type rotationTestCreativeRepo struct {
	creatives []*models.Creative
	served    map[string]map[string]int
}

func (r *rotationTestCreativeRepo) Create(ctx context.Context, creative *models.Creative) error { return nil }
func (r *rotationTestCreativeRepo) GetByID(ctx context.Context, id string) (*models.Creative, error) {
	return nil, nil
}
func (r *rotationTestCreativeRepo) ListAll(ctx context.Context, limit int, offset int, createdByUserID *string) ([]*models.Creative, error) {
	return []*models.Creative{}, nil
}
func (r *rotationTestCreativeRepo) CountAll(ctx context.Context, createdByUserID *string) (int, error) { return 0, nil }
func (r *rotationTestCreativeRepo) Search(ctx context.Context, term string, limit int, offset int, createdByUserID *string) ([]*models.Creative, int, error) {
	return []*models.Creative{}, 0, nil
}
func (r *rotationTestCreativeRepo) ListByCampaign(ctx context.Context, campaignID string, limit int, offset int) ([]*models.Creative, error) {
	return []*models.Creative{}, nil
}
func (r *rotationTestCreativeRepo) CountByCampaign(ctx context.Context, campaignID string) (int, error) { return 0, nil }
func (r *rotationTestCreativeRepo) ListByDevice(ctx context.Context, device string, activeNow bool, now time.Time, limit int, offset int) ([]*models.Creative, error) {
	return r.creatives, nil
}
func (r *rotationTestCreativeRepo) CountByDevice(ctx context.Context, device string, activeNow bool, now time.Time) (int, error) {
	return len(r.creatives), nil
}
func (r *rotationTestCreativeRepo) Update(ctx context.Context, id string, req *models.UpdateCreativeRequest) error { return nil }
func (r *rotationTestCreativeRepo) Delete(ctx context.Context, id string) error { return nil }
func (r *rotationTestCreativeRepo) PickNextRotationalCreative(ctx context.Context, device string, campaignID string, candidateCreativeIDs []string) (string, error) {
	if len(candidateCreativeIDs) == 0 {
		return "", nil
	}
	if r.served == nil {
		r.served = make(map[string]map[string]int)
	}
	key := device + "|" + campaignID
	if r.served[key] == nil {
		r.served[key] = make(map[string]int)
	}
	weights := make(map[string]int, len(candidateCreativeIDs))
	sumW := 0
	for _, id := range candidateCreativeIDs {
		w := 0
		for _, c := range r.creatives {
			if c != nil && c.ID == id {
				w = c.PlayWeight
				break
			}
		}
		if w < 0 {
			w = 0
		}
		weights[id] = w
		sumW += w
	}
	if sumW <= 0 {
		sumW = len(candidateCreativeIDs)
		for _, id := range candidateCreativeIDs {
			weights[id] = 1
		}
	}
	// Deficit-based pick
	totalServed := 0
	for _, id := range candidateCreativeIDs {
		totalServed += r.served[key][id]
	}
	projectedTotal := totalServed + 1
	chosen := candidateCreativeIDs[0]
	best := -1e9
	for _, id := range candidateCreativeIDs {
		expected := float64(projectedTotal) * float64(weights[id]) / float64(sumW)
		deficit := expected - float64(r.served[key][id])
		if deficit > best {
			best = deficit
			chosen = id
		}
	}
	r.served[key][chosen]++
	return chosen, nil
}

type noopCreativeRepo struct{}

func (noopCreativeRepo) Create(ctx context.Context, creative *models.Creative) error { return nil }
func (noopCreativeRepo) GetByID(ctx context.Context, id string) (*models.Creative, error) {
	return nil, nil
}
func (noopCreativeRepo) ListAll(ctx context.Context, limit int, offset int, createdByUserID *string) ([]*models.Creative, error) {
	return []*models.Creative{}, nil
}
func (noopCreativeRepo) CountAll(ctx context.Context, createdByUserID *string) (int, error) { return 0, nil }
func (noopCreativeRepo) Search(ctx context.Context, term string, limit int, offset int, createdByUserID *string) ([]*models.Creative, int, error) {
	return []*models.Creative{}, 0, nil
}
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

func (noopCreativeRepo) PickNextRotationalCreative(ctx context.Context, device string, campaignID string, candidateCreativeIDs []string) (string, error) {
	if len(candidateCreativeIDs) == 0 {
		return "", nil
	}
	return candidateCreativeIDs[0], nil
}

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
func (noopCampaignRepo) Search(ctx context.Context, term string, limit int, offset int, createdByUserID *string) ([]*models.Campaign, int, error) {
	return []*models.Campaign{}, 0, nil
}
func (noopCampaignRepo) ActivateScheduledStartingOn(ctx context.Context, startDate time.Time, scheduledStatus string, timeZone string) (int64, error) {
	return 0, nil
}
func (noopCampaignRepo) CompleteActiveEndedBefore(ctx context.Context, now time.Time, activeStatus string, completedStatus string, timeZone string) (int64, error) {
	return 0, nil
}
func (noopCampaignRepo) Update(ctx context.Context, id string, campaign *models.Campaign) error { return nil }
func (noopCampaignRepo) Delete(ctx context.Context, id string) error { return nil }
func (noopCampaignRepo) ListByStartDate(ctx context.Context, startDate time.Time) ([]*models.Campaign, error) {
	return []*models.Campaign{}, nil
}
func (noopCampaignRepo) ListByEndDate(ctx context.Context, endDate time.Time) ([]*models.Campaign, error) {
	return []*models.Campaign{}, nil
}

func TestUploadCreativeMissingCampaignIDReturnsJSON(t *testing.T) {
	h := NewCreativeHandler(&noopCreativeRepo{}, noopCampaignRepo{}, &config.S3Config{}, nil, &config.Config{})

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

func TestScoreVenuesReturnsMatches(t *testing.T) {
	venues := []*models.Venue{
		{ID: 16, Name: "Education", SubCategory: []string{"Schools", "Colleges and Universities"}},
		{ID: 10, Name: "Residential", SubCategory: []string{"Apartment Buildings"}},
	}

	suggestions, keywords := scoreVenues(venues, "Summer camp registration at colleges", 5)
	if len(keywords) == 0 {
		t.Fatalf("expected keywords")
	}
	if len(suggestions) == 0 {
		t.Fatalf("expected suggestions")
	}
	if suggestions[0].VenueID != 16 {
		t.Fatalf("expected top suggestion Education(16), got %+v", suggestions[0])
	}
}

func TestScoreVenuesMatchesRealTaxonomy(t *testing.T) {
	venues := []*models.Venue{
		{ID: 16, Name: "Education", SubCategory: []string{"Schools", "Colleges and Universities"}},
		{ID: 15, Name: "Entertainment", SubCategory: []string{"Recreational Locations", "Sports Entertainment"}},
		{ID: 8, Name: "Transit", SubCategory: []string{"Airports", "Buses"}},
	}

	text := "Parks & Recreation Summer Camps Summer Sports Clinics"
	suggestions, _ := scoreVenues(venues, text, 5)
	if len(suggestions) == 0 {
		t.Fatalf("expected suggestions, got none")
	}
	// Should include at least Education (camp->school->Schools) or Entertainment (recreation/parks->recreational)
	found := false
	for _, s := range suggestions {
		if s.VenueID == 16 || s.VenueID == 15 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected Education or Entertainment in suggestions, got %+v", suggestions)
	}
}

func TestScoreVenuesRetailAndFamilyIntentBoosts(t *testing.T) {
	venues := []*models.Venue{
		{ID: 9, Name: "Retail", SubCategory: []string{"Grocery", "Malls", "Pharmacies"}},
		{ID: 10, Name: "Residential", SubCategory: []string{"Apartment Buildings"}},
		{ID: 15, Name: "Entertainment", SubCategory: []string{"QSR", "Recreational Locations"}},
		{ID: 14, Name: "Financial", SubCategory: []string{"Banks"}},
	}

	text := "New & only at Target. Start potty training with training pants"
	suggestions, _ := scoreVenues(venues, text, 5)
	if len(suggestions) == 0 {
		t.Fatalf("expected suggestions")
	}
	// Retail should be present due to retail intent phrase
	foundRetail := false
	foundResidential := false
	for _, s := range suggestions {
		if s.VenueID == 9 {
			foundRetail = true
		}
		if s.VenueID == 10 {
			foundResidential = true
		}
	}
	if !foundRetail {
		t.Fatalf("expected Retail in suggestions, got %+v", suggestions)
	}
	if !foundResidential {
		t.Fatalf("expected Residential in suggestions, got %+v", suggestions)
	}
}

func TestSuggestVenuesNoFilesReturnsJSON(t *testing.T) {
	h := NewCreativeHandler(&noopCreativeRepo{}, noopCampaignRepo{}, &config.S3Config{}, nil, &config.Config{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/creatives/suggestions", nil)
	w := httptest.NewRecorder()
	h.SuggestVenues(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d (%s)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json got %q", ct)
	}
}

func TestSuggestVenuesUnsupportedContentTypeReturnsFileError(t *testing.T) {
	h := NewCreativeHandler(&noopCreativeRepo{}, noopCampaignRepo{}, &config.S3Config{}, nil, &config.Config{})

	form := &multipart.Form{File: map[string][]*multipart.FileHeader{}}
	fh := &multipart.FileHeader{Filename: "x.txt", Header: textproto.MIMEHeader{}}
	fh.Header.Set("Content-Type", "text/plain")
	form.File["files"] = []*multipart.FileHeader{fh}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/creatives/suggestions", nil)
	req.MultipartForm = form
	w := httptest.NewRecorder()
	h.SuggestVenues(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d (%s)", w.Code, w.Body.String())
	}
}

func TestListCreativesByDeviceWeightedRotationPerCampaign(t *testing.T) {
	baseTime := time.Date(2026, 1, 22, 0, 0, 0, 0, time.UTC)
	repo := &rotationTestCreativeRepo{creatives: []*models.Creative{
		{ID: "c1a", CampaignID: "camp1", PlayWeight: 75, UploadedAt: baseTime.Add(-2 * time.Hour)},
		{ID: "c1b", CampaignID: "camp1", PlayWeight: 25, UploadedAt: baseTime.Add(-1 * time.Hour)},
		{ID: "c2a", CampaignID: "camp2", PlayWeight: 100, UploadedAt: baseTime.Add(-3 * time.Hour)},
	}}
	h := NewCreativeHandler(repo, noopCampaignRepo{}, &config.S3Config{}, nil, &config.Config{})

	withDeviceParam := func(req *http.Request, device string) *http.Request {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("device", device)
		return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	}

	counts := map[string]int{"c1a": 0, "c1b": 0}
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/creatives/device/dev1?page=1&page_size=50", nil)
		req = withDeviceParam(req, "dev1")
		w := httptest.NewRecorder()
		h.ListCreativesByDevice(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 got %d (%s)", w.Code, w.Body.String())
		}
		var resp struct {
			Data []models.Creative `json:"data"`
		}
		if err := json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v (%s)", err, w.Body.String())
		}
		if len(resp.Data) != 2 {
			t.Fatalf("expected 2 creatives (one per campaign), got %d (%s)", len(resp.Data), w.Body.String())
		}
		for _, c := range resp.Data {
			if c.CampaignID == "camp1" {
				counts[c.ID]++
			}
		}
	}
	// Allow small drift; deficit algorithm should be very close.
	if counts["c1a"] < 70 || counts["c1a"] > 80 {
		t.Fatalf("expected c1a around 75%%; got %d/100", counts["c1a"])
	}
	if counts["c1b"] < 20 || counts["c1b"] > 30 {
		t.Fatalf("expected c1b around 25%%; got %d/100", counts["c1b"])
	}
}
