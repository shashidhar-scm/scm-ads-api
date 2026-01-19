package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/lib/pq"
	"scm/internal/interfaces"
	"scm/internal/middleware"
	"scm/internal/models"
)

type AdvertiserHandler struct {
    repo      interfaces.AdvertiserRepository
    db        *sql.DB
    validator *validator.Validate
}

func NewAdvertiserHandler(repo interfaces.AdvertiserRepository, db *sql.DB) *AdvertiserHandler {
    return &AdvertiserHandler{
        repo:      repo,
        db:        db,
        validator: validator.New(),
    }
}

func (h *AdvertiserHandler) assignCreatorAdvertiserRole(ctx context.Context, userID string, advertiserID string) error {
	if h.db == nil {
		return nil
	}
	var roleID string
	if err := h.db.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = 'advertiser' LIMIT 1`).Scan(&roleID); err != nil {
		return err
	}
	_, err := h.db.ExecContext(ctx, `INSERT INTO user_roles (user_id, role_id, advertiser_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, userID, roleID, advertiserID)
	return err
}

// @Tags Advertisers
// @Summary Create advertiser
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body models.CreateAdvertiserRequest true "Create advertiser request"
// @Success 201 {object} models.Advertiser
// @Failure 400 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/advertisers/ [post]
func (h *AdvertiserHandler) CreateAdvertiser(w http.ResponseWriter, r *http.Request) {
	log.Println("=== CreateAdvertiser handler called ===")
	createdBy, _ := r.Context().Value(middleware.CtxUserID).(string)
	if strings.TrimSpace(createdBy) == "" {
		writeJSONErrorResponse(w, http.StatusUnauthorized, "unauthorized", "missing user identity")
		return
	}
	
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
		CreatedBy: createdBy,
	}

	if err := h.repo.Create(r.Context(), advertiser); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			// 23505 = unique_violation
			if pqErr.Code == "23505" {
				writeJSONErrorResponse(w, http.StatusConflict, "advertiser_already_exists", "Advertiser already exists")
				return
			}
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "create_advertiser_failed", "Failed to create advertiser")
		return
	}

	if err := h.assignCreatorAdvertiserRole(r.Context(), createdBy, advertiser.ID); err != nil {
		_ = h.repo.Delete(r.Context(), advertiser.ID)
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

	permissionGlobal, _ := r.Context().Value(middleware.CtxPermissionGlobal).(bool)
	var advertiser *models.Advertiser
	var err error
	if permissionGlobal {
		advertiser, err = h.repo.GetByID(r.Context(), id)
	} else {
		allowedIDs, _ := r.Context().Value(middleware.CtxAdvertiserIDs).([]string)
		advertiser, err = h.repo.GetByIDInSet(r.Context(), id, allowedIDs)
	}
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

	permissionGlobal, _ := r.Context().Value(middleware.CtxPermissionGlobal).(bool)
	allowedIDs, _ := r.Context().Value(middleware.CtxAdvertiserIDs).([]string)

	var total int
	if permissionGlobal {
		total, err = h.repo.Count(r.Context())
	} else {
		total, err = h.repo.CountByIDs(r.Context(), allowedIDs)
	}
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_advertisers_failed", "Failed to list advertisers")
		return
	}

	var advertisers []models.Advertiser
	if permissionGlobal {
		advertisers, err = h.repo.List(r.Context(), p.limit, p.offset)
	} else {
		advertisers, err = h.repo.ListByIDs(r.Context(), allowedIDs, p.limit, p.offset)
	}
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
// @Summary Search advertisers
// @Security BearerAuth
// @Produce json
// @Param query query string true "Search text (matches name or email)"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/advertisers/search [get]
func (h *AdvertiserHandler) SearchAdvertisers(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "query is required")
		return
	}

	p, err := parsePaginationParams(r, 50, 200)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "invalid pagination parameters")
		return
	}

	permissionGlobal, _ := r.Context().Value(middleware.CtxPermissionGlobal).(bool)
	allowedIDs, _ := r.Context().Value(middleware.CtxAdvertiserIDs).([]string)

	var advertisers []models.Advertiser
	var total int
	if permissionGlobal {
		advertisers, total, err = h.repo.Search(r.Context(), query, p.limit, p.offset)
	} else {
		advertisers, total, err = h.repo.SearchByIDs(r.Context(), allowedIDs, query, p.limit, p.offset)
	}
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "search_advertisers_failed", "Failed to search advertisers")
		return
	}

	if advertisers == nil {
		advertisers = []models.Advertiser{}
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
