package handlers

import (
	"errors"
	"fmt"
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

// @Tags Advertisers
// @Summary Create advertiser
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body models.CreateAdvertiserRequest true "Create advertiser request"
// @Success 201 {object} models.Advertiser
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/advertisers/ [post]
func (h *AdvertiserHandler) CreateAdvertiser(w http.ResponseWriter, r *http.Request) {
	log.Println("=== CreateAdvertiser handler called ===")
	
	var req models.CreateAdvertiserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	advertiser := &models.Advertiser{
		Name:      req.Name,
		Email:     req.Email,
		CreatedBy: req.CreatedBy,
	}

	if err := h.repo.Create(r.Context(), advertiser); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "create_advertiser_failed", "Failed to create advertiser")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(advertiser)
}

// @Tags Advertisers
// @Summary Get advertiser
// @Security BearerAuth
// @Produce json
// @Param id path string true "Advertiser ID"
// @Success 200 {object} models.Advertiser
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/advertisers/{id}/ [get]
func (h *AdvertiserHandler) GetAdvertiser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "Advertiser ID is required")
		return
	}

	advertiser, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "advertiser_not_found", "Advertiser not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_advertiser_failed", "Failed to get advertiser")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(advertiser)
}

// @Tags Advertisers
// @Summary List advertisers
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Advertiser
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/advertisers/ [get]
func (h *AdvertiserHandler) ListAdvertisers(w http.ResponseWriter, r *http.Request) {
	p, err := parsePaginationParams(r, 50, 200)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "invalid pagination parameters")
		return
	}

	total, err := h.repo.Count(r.Context())
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_advertisers_failed", "Failed to list advertisers")
		return
	}

	advertisers, err := h.repo.List(r.Context(), p.limit, p.offset)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_advertisers_failed", "Failed to list advertisers")
		return
	}

	if advertisers == nil {
		advertisers = []models.Advertiser{} // Return empty array instead of null
	}

	writePaginatedResponse(w, http.StatusOK, advertisers, p.page, p.pageSize, total)
}

// @Tags Advertisers
// @Summary Update advertiser
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Advertiser ID"
// @Param body body models.UpdateAdvertiserRequest true "Update advertiser request"
// @Success 200 {object} models.Advertiser
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/advertisers/{id}/ [put]
func (h *AdvertiserHandler) UpdateAdvertiser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "Advertiser ID is required")
		return
	}

	_, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "advertiser_not_found", "Advertiser not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_advertiser_failed", "Failed to get advertiser")
		return
	}

	var req models.UpdateAdvertiserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Name == nil && req.Email == nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "No fields to update")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := h.repo.Update(r.Context(), id, &req); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "update_advertiser_failed", "Failed to update advertiser")
		return
	}

	advertiser, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "advertiser_not_found", "Advertiser not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_advertiser_failed", "Failed to get advertiser")
		return
	}


	w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(advertiser)
}

// @Tags Advertisers
// @Summary Delete advertiser
// @Security BearerAuth
// @Produce json
// @Param id path string true "Advertiser ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/advertisers/{id}/ [delete]
func (h *AdvertiserHandler) DeleteAdvertiser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "Advertiser ID is required")
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		var blocked *interfaces.DeletionBlockedError
		if errors.As(err, &blocked) {
			msg := fmt.Sprintf("Cannot delete %s: referenced by", blocked.Resource)
			for k, v := range blocked.References {
				msg += fmt.Sprintf(" %d %s", v, k)
			}
			writeJSONErrorResponse(w, http.StatusConflict, "delete_blocked", msg)
			return
		}
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "advertiser_not_found", "Advertiser not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "delete_advertiser_failed", "Failed to delete advertiser")
		return
	}
	// Success response
	writeJSONMessage(w, http.StatusOK, "advertiser deleted successfully")
}
