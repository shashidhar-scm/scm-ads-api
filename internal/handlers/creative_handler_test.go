package handlers

import (
	"context"
	"mime/multipart"
	"net/textproto"
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

func (noopCreativeRepo) EnsureRotationGroup(ctx context.Context, campaignID string, name string, selectedDays []string, timeSlots []string) (string, error) {
	return "", nil
}

func (noopCreativeRepo) PickNextRotationalCreative(ctx context.Context, device string, campaignID string, rotationGroupID string, candidateCreativeIDs []string) (string, error) {
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
	h := NewCreativeHandler(&noopCreativeRepo{}, noopCampaignRepo{}, &config.S3Config{}, nil)

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
	h := NewCreativeHandler(&noopCreativeRepo{}, noopCampaignRepo{}, &config.S3Config{}, nil)
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
	h := NewCreativeHandler(&noopCreativeRepo{}, noopCampaignRepo{}, &config.S3Config{}, nil)

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
