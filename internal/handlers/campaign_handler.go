// internal/handlers/campaign_handler.go
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
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"scm/internal/interfaces"
	authmw "scm/internal/middleware"
	"scm/internal/models"
	"scm/internal/repository"
	"scm/internal/services"
)

func (h *CampaignHandler) isGlobalAdmin(ctx context.Context, userID string) (bool, error) {
	if h.userRoles == nil || strings.TrimSpace(userID) == "" {
		return false, nil
	}
	isSuper, err := h.userRoles.IsSuperAdmin(ctx, userID)
	if err != nil {
		return false, err
	}
	isAdmin, err := h.userRoles.IsAdmin(ctx, userID)
	if err != nil {
		return false, err
	}
	return isSuper || isAdmin, nil
}

func (h *CampaignHandler) ensureAdvertiserOwnedByCaller(ctx context.Context, advertiserID string, callerID string) (bool, error) {
	if h.db == nil {
		return false, nil
	}
	var ok bool
	if err := h.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM advertisers WHERE id = $1 AND created_by = $2)`, advertiserID, callerID).Scan(&ok); err != nil {
		return false, err
	}
	return ok, nil
}

func (h *CampaignHandler) ensureCampaignOwnedByCaller(ctx context.Context, campaignID string, callerID string) (bool, error) {
	if h.db == nil {
		return false, nil
	}
	var ok bool
	if err := h.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM campaigns c
			JOIN advertisers a ON a.id = c.advertiser_id
			WHERE c.id = $1 AND a.created_by = $2
		)
	`, campaignID, callerID).Scan(&ok); err != nil {
		return false, err
	}
	return ok, nil
}

func writeJSONErrorCampaign(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":   code,
		"message": message,
	})
}

type CampaignHandler struct {
	repo      interfaces.CampaignRepository
	validator *validator.Validate
	popAPI    services.PopAPI
	db        *sql.DB
	userRoles repository.UserRoleRepository
}

func NewCampaignHandler(repo interfaces.CampaignRepository, db *sql.DB, userRoles repository.UserRoleRepository) *CampaignHandler {
	return &CampaignHandler{
		repo:      repo,
		validator: validator.New(),
		db:        db,
		userRoles: userRoles,
	}
}

func NewCampaignHandlerWithPop(repo interfaces.CampaignRepository, popAPI services.PopAPI, db *sql.DB, userRoles repository.UserRoleRepository) *CampaignHandler {
	return &CampaignHandler{
		repo:      repo,
		validator: validator.New(),
		popAPI:    popAPI,
		db:        db,
		userRoles: userRoles,
	}
}

// @Tags Campaigns
// @Summary Campaign lifetime impressions
// @Security BearerAuth
// @Produce json
// @Param id path string true "Campaign ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/campaigns/{id}/impressions [get]
func (h *CampaignHandler) GetCampaignImpressions(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "id")
	if campaignID == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "Campaign ID is required")
		return
	}

	callerID, _ := r.Context().Value(authmw.CtxUserID).(string)
	isGlobalAdmin, err := h.isGlobalAdmin(r.Context(), callerID)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_campaign_failed", "Failed to fetch campaign")
		return
	}
	if !isGlobalAdmin {
		ok, err := h.ensureCampaignOwnedByCaller(r.Context(), campaignID, callerID)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "get_campaign_failed", "Failed to fetch campaign")
			return
		}
		if !ok {
			writeJSONErrorResponse(w, http.StatusNotFound, "campaign_not_found", "Campaign not found")
			return
		}
	}

	if _, err := h.repo.GetByID(r.Context(), campaignID); err != nil {
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "campaign_not_found", "Campaign not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_campaign_failed", "Failed to fetch campaign")
		return
	}

	if h.popAPI == nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "pop_not_configured", "POP API client is not configured")
		return
	}

	impressions, err := h.popAPI.CampaignImpressions(r.Context(), campaignID)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "pop_request_failed", "Failed to fetch impressions")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{
			"campaign_id": campaignID,
			"impressions": impressions.Impressions,
			"posters":     impressions.Posters,
		},
	})
}

// @Tags Campaigns
// @Summary Create campaign
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body models.CreateCampaignRequest true "Create campaign request"
// @Success 201 {object} models.Campaign
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/campaigns/ [post]
func (h *CampaignHandler) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	log.Println("=== CreateCampaign handler called ===")
	var req models.CreateCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorCampaign(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		writeJSONErrorCampaign(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	callerID, _ := r.Context().Value(authmw.CtxUserID).(string)
	isGlobalAdmin, err := h.isGlobalAdmin(r.Context(), callerID)
	if err != nil {
		writeJSONErrorCampaign(w, http.StatusInternalServerError, "create_campaign_failed", "Failed to create campaign")
		return
	}
	if !isGlobalAdmin {
		ok, err := h.ensureAdvertiserOwnedByCaller(r.Context(), req.AdvertiserID, callerID)
		if err != nil {
			writeJSONErrorCampaign(w, http.StatusInternalServerError, "create_campaign_failed", "Failed to create campaign")
			return
		}
		if !ok {
			writeJSONErrorCampaign(w, http.StatusForbidden, "forbidden", "Invalid advertiser")
			return
		}
	}

	campaign := &models.Campaign{
		Name:             req.Name,
		Status:           models.CampaignStatusDraft,
		Cities:           req.Cities,
		StartDate:        req.StartDate,
		EndDate:          req.EndDate,
		Budget:           req.Budget,
		ImpressionsBased: req.ImpressionsBased,
		AdvertiserID:     req.AdvertiserID,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	log.Println("Campaign created:", campaign)
	if err := h.repo.Create(r.Context(), campaign); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			if pqErr.Code == "23503" {
				if pqErr.Constraint == "campaigns_advertiser_id_fkey" {
					writeJSONErrorCampaign(w, http.StatusBadRequest, "invalid_advertiser_id", "Advertiser not found")
					return
				}
				writeJSONErrorCampaign(w, http.StatusBadRequest, "foreign_key_violation", "Invalid reference")
				return
			}
		}
		writeJSONErrorCampaign(w, http.StatusInternalServerError, "create_campaign_failed", "Failed to create campaign")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(campaign)
}

// @Tags Campaigns
// @Summary Get campaign
// @Security BearerAuth
// @Produce json
// @Param id path string true "Campaign ID"
// @Success 200 {object} models.Campaign
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/campaigns/{id}/ [get]
func (h *CampaignHandler) GetCampaign(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "id")
	if campaignID == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "Campaign ID is required")
		return
	}

	callerID, _ := r.Context().Value(authmw.CtxUserID).(string)
	isGlobalAdmin, err := h.isGlobalAdmin(r.Context(), callerID)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_campaign_failed", "Failed to fetch campaign")
		return
	}
	if !isGlobalAdmin {
		ok, err := h.ensureCampaignOwnedByCaller(r.Context(), campaignID, callerID)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "get_campaign_failed", "Failed to fetch campaign")
			return
		}
		if !ok {
			writeJSONErrorResponse(w, http.StatusNotFound, "campaign_not_found", "Campaign not found")
			return
		}
	}

	campaign, err := h.repo.GetByID(r.Context(), campaignID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "campaign_not_found", "Campaign not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_campaign_failed", "Failed to fetch campaign")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(campaign)
}

// @Tags Campaigns
// @Summary List campaigns
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/campaigns/ [get]
func (h *CampaignHandler) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	log.Println("=== ListCampaigns handler called ===")

	p, err := parsePaginationParams(r, 50, 200)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "invalid pagination parameters")
		return
	}

	includeImpressions := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("include_impressions")), "true")
	if includeImpressions && h.popAPI == nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "pop_not_configured", "POP API client is not configured")
		return
	}

	filter := interfaces.CampaignFilter{Limit: p.limit, Offset: p.offset}

	// If not a global admin, scope list to campaigns whose advertisers were created by this user.
	userID, _ := r.Context().Value(authmw.CtxUserID).(string)
	isGlobalAdmin := false
	if h.userRoles != nil && strings.TrimSpace(userID) != "" {
		isSuper, err := h.userRoles.IsSuperAdmin(r.Context(), userID)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "list_campaigns_failed", "Failed to list campaigns")
			return
		}
		isAdmin, err := h.userRoles.IsAdmin(r.Context(), userID)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "list_campaigns_failed", "Failed to list campaigns")
			return
		}
		isGlobalAdmin = isSuper || isAdmin
	}
	if !isGlobalAdmin {
		filter.CreatedByUserID = &userID
	}

	total, err := h.repo.Count(r.Context(), filter)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_campaigns_failed", "Failed to list campaigns")
		return
	}

	summary, err := h.repo.Summary(r.Context(), filter)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_campaigns_failed", "Failed to list campaigns")
		return
	}

	campaigns, err := h.repo.List(r.Context(), filter)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_campaigns_failed", "Failed to list campaigns")
		return
	}

	if campaigns == nil {
		campaigns = []*models.Campaign{} // Return empty array instead of null
	}

	if includeImpressions {
		for _, c := range campaigns {
			if c == nil || strings.TrimSpace(c.ID) == "" {
				continue
			}
			imps, err := h.popAPI.CampaignImpressions(r.Context(), c.ID)
			if err != nil {
				writeJSONErrorResponse(w, http.StatusInternalServerError, "pop_request_failed", "Failed to fetch impressions")
				return
			}
			if imps != nil {
				c.LifetimeImpressions = &imps.Impressions
			}
		}
	}

	data := map[string]any{
		"active_campaign_count": summary.ActiveCampaignCount,
		"total_budget":          summary.TotalBudget,
		"total_impression":      summary.TotalImpression,
		"served_impression":     summary.ServedImpression,
		"campaigns":             campaigns,
	}
	writePaginatedResponse(w, http.StatusOK, data, p.page, p.pageSize, total)
}

// @Tags Campaigns
// @Summary Search campaigns
// @Security BearerAuth
// @Produce json
// @Param query query string true "Search text (matches name, cities, start/end dates, budget, spent, clicks)"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/campaigns/search [get]
func (h *CampaignHandler) SearchCampaigns(w http.ResponseWriter, r *http.Request) {
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

	userID, _ := r.Context().Value(authmw.CtxUserID).(string)
	isGlobalAdmin := false
	if h.userRoles != nil && strings.TrimSpace(userID) != "" {
		isSuper, err := h.userRoles.IsSuperAdmin(r.Context(), userID)
		if err == nil {
			isAdmin, errAdmin := h.userRoles.IsAdmin(r.Context(), userID)
			if errAdmin == nil {
				isGlobalAdmin = isSuper || isAdmin
			}
		}
	}
	var createdByFilter *string
	if !isGlobalAdmin {
		createdByFilter = &userID
	}

	campaigns, total, err := h.repo.Search(r.Context(), query, p.limit, p.offset, createdByFilter)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "search_campaigns_failed", "Failed to search campaigns")
		return
	}

	if campaigns == nil {
		campaigns = []*models.Campaign{}
	}

	writePaginatedResponse(w, http.StatusOK, campaigns, p.page, p.pageSize, total)
}

// @Tags Campaigns
// @Summary List campaigns by advertiser
// @Security BearerAuth
// @Produce json
// @Param advertiserID path string true "Advertiser ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/campaigns/advertiser/{advertiserID} [get]
func (h *CampaignHandler) ListCampaignsByAdvertiser(w http.ResponseWriter, r *http.Request) {
	advertiserID := chi.URLParam(r, "advertiserID")
	if advertiserID == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "advertiserID is required")
		return
	}

	if _, err := uuid.Parse(advertiserID); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "advertiserID must be a valid UUID")
		return
	}

	p, err := parsePaginationParams(r, 50, 200)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "invalid pagination parameters")
		return
	}

	callerID, _ := r.Context().Value(authmw.CtxUserID).(string)
	isGlobalAdmin, err := h.isGlobalAdmin(r.Context(), callerID)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_campaigns_failed", "Failed to list campaigns")
		return
	}

	filter := interfaces.CampaignFilter{
		AdvertiserID: advertiserID,
		Limit:        p.limit,
		Offset:       p.offset,
	}

	if !isGlobalAdmin {
		ok, err := h.ensureAdvertiserOwnedByCaller(r.Context(), advertiserID, callerID)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "list_campaigns_failed", "Failed to list campaigns")
			return
		}
		if !ok {
			writeJSONErrorResponse(w, http.StatusNotFound, "advertiser_not_found", "Advertiser not found")
			return
		}
		filter.CreatedByUserID = &callerID
	}

	total, err := h.repo.Count(r.Context(), filter)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_campaigns_failed", "Failed to list campaigns")
		return
	}

	summary, err := h.repo.Summary(r.Context(), filter)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_campaigns_failed", "Failed to list campaigns")
		return
	}

	campaigns, err := h.repo.List(r.Context(), filter)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_campaigns_failed", "Failed to list campaigns")
		return
	}
	if campaigns == nil {
		campaigns = []*models.Campaign{}
	}

	data := map[string]any{
		"active_campaign_count": summary.ActiveCampaignCount,
		"total_budget":          summary.TotalBudget,
		"total_impression":      summary.TotalImpression,
		"campaigns":             campaigns,
	}
	writePaginatedResponse(w, http.StatusOK, data, p.page, p.pageSize, total)
}

// @Tags Campaigns
// @Summary Update campaign
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Campaign ID"
// @Param body body models.UpdateCampaignRequest true "Update campaign request"
// @Success 200 {object} models.Campaign
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/campaigns/{id}/ [put]
func (h *CampaignHandler) UpdateCampaign(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "Campaign ID is required")
		return
	}

	var req models.UpdateCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	callerID, _ := r.Context().Value(authmw.CtxUserID).(string)
	isGlobalAdmin, err := h.isGlobalAdmin(r.Context(), callerID)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "update_campaign_failed", "Failed to update campaign")
		return
	}
	if !isGlobalAdmin {
		ok, err := h.ensureCampaignOwnedByCaller(r.Context(), id, callerID)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "update_campaign_failed", "Failed to update campaign")
			return
		}
		if !ok {
			writeJSONErrorResponse(w, http.StatusNotFound, "campaign_not_found", "Campaign not found")
			return
		}
	}

	// First, get the existing campaign
	existingCampaign, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "campaign_not_found", "Campaign not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_campaign_failed", "Failed to get campaign")
		return
	}
	// Update the existing campaign with the new values
	if req.Name != nil {
		existingCampaign.Name = *req.Name
	}
	if req.Status != nil {
		existingCampaign.Status = models.CampaignStatus(*req.Status)
	}
	if req.Cities != nil {
		existingCampaign.Cities = *req.Cities
	}
	if req.StartDate != nil {
		existingCampaign.StartDate = *req.StartDate
	}
	if req.EndDate != nil {
		existingCampaign.EndDate = *req.EndDate
	}
	if req.Budget != nil {
		existingCampaign.Budget = *req.Budget
	}
	if req.ImpressionsBased != nil {
		existingCampaign.ImpressionsBased = *req.ImpressionsBased
	}

	// Update the campaign in the database
	err = h.repo.Update(r.Context(), id, existingCampaign)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "update_campaign_failed", "Failed to update campaign")
		return
	}

	// Get the updated campaign to return
	updatedCampaign, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "campaign_not_found", "Campaign not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_campaign_failed", "Failed to get updated campaign")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedCampaign)
}

// @Tags Campaigns
// @Summary Delete campaign
// @Security BearerAuth
// @Produce json
// @Param id path string true "Campaign ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/campaigns/{id}/ [delete]
func (h *CampaignHandler) DeleteCampaign(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "id")
	log.Println("Deleting campaign with ID:", campaignID)

	if campaignID == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "Campaign ID is required")
		return
	}

	callerID, _ := r.Context().Value(authmw.CtxUserID).(string)
	isGlobalAdmin, err := h.isGlobalAdmin(r.Context(), callerID)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "delete_campaign_failed", "Failed to delete campaign")
		return
	}
	if !isGlobalAdmin {
		ok, err := h.ensureCampaignOwnedByCaller(r.Context(), campaignID, callerID)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "delete_campaign_failed", "Failed to delete campaign")
			return
		}
		if !ok {
			writeJSONErrorResponse(w, http.StatusNotFound, "campaign_not_found", "Campaign not found")
			return
		}
	}

	err = h.repo.Delete(r.Context(), campaignID)
	if err != nil {
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
			writeJSONErrorResponse(w, http.StatusNotFound, "campaign_not_found", "Campaign not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "delete_campaign_failed", "Failed to delete campaign")
		return
	}

	writeJSONMessage(w, http.StatusOK, "campaign deleted successfully")
}
