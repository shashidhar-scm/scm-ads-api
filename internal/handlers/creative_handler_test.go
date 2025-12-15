package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"scm/internal/config"
	"scm/internal/repository"
)

type noopCreativeRepo struct{ repository.CreativeRepository }

func TestUploadCreativeMissingCampaignIDReturnsJSON(t *testing.T) {
	h := NewCreativeHandler(&noopCreativeRepo{}, &config.S3Config{})

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
