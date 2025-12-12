package handlers

import (
    "encoding/json"
    "log"
    "net/http"
    "database/sql"
    "github.com/go-chi/chi/v5"
    "github.com/go-playground/validator/v10"
    "scm/internal/interfaces"
    "scm/internal/models"
)

type AdvertiserHandler struct {
    repo      interfaces.AdvertiserRepository
    validator *validator.Validate
}

func NewAdvertiserHandler(repo interfaces.AdvertiserRepository) *AdvertiserHandler {
    return &AdvertiserHandler{
        repo:      repo,
        validator: validator.New(),
    }
}


func (h *AdvertiserHandler) CreateAdvertiser(w http.ResponseWriter, r *http.Request) {
	log.Println("=== CreateAdvertiser handler called ===")
	
	var req models.CreateAdvertiserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	advertiser := &models.Advertiser{
		Name:  req.Name,
		Email: req.Email,
	}

	if err := h.repo.Create(r.Context(), advertiser); err != nil {
		http.Error(w, "Failed to create advertiser: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(advertiser)
}

func (h *AdvertiserHandler) GetAdvertiser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Advertiser ID is required", http.StatusBadRequest)
		return
	}

	advertiser, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "advertiser not found",
			})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(advertiser)
}

func (h *AdvertiserHandler) ListAdvertisers(w http.ResponseWriter, r *http.Request) {
	advertisers, err := h.repo.List(r.Context())
	if err != nil {
		http.Error(w, "Failed to list advertisers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if advertisers == nil {
		advertisers = []models.Advertiser{} // Return empty array instead of null
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(advertisers)
}

func (h *AdvertiserHandler) UpdateAdvertiser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Advertiser ID is required", http.StatusBadRequest)
		return
	}

	_, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "advertiser not found",
			})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var req models.UpdateAdvertiserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == nil && req.Email == nil {
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.repo.Update(r.Context(), id, &req); err != nil {
		http.Error(w, "Failed to update advertiser: "+err.Error(), http.StatusInternalServerError)
		return
	}

	advertiser, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "advertiser not found",
			})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}


	w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(advertiser)
}

func (h *AdvertiserHandler) DeleteAdvertiser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Advertiser ID is required", http.StatusBadRequest)
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "advertiser not found",
			})
			return
		}
		http.Error(w, "Failed to delete advertiser: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Success response
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(map[string]string{
        "message": "advertiser deleted successfully",
        "id":      id,
    })
}
