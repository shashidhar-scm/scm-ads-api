// internal/handlers/campaign_handler.go
package handlers

import (
    "database/sql"
    "encoding/json"
    "errors"
    "log"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-playground/validator/v10"
    "github.com/lib/pq"
    "scm/internal/interfaces"
    "scm/internal/models"
)

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
}

func NewCampaignHandler(repo interfaces.CampaignRepository) *CampaignHandler {
    return &CampaignHandler{
        repo:      repo,
        validator: validator.New(),
    }
}

// CreateCampaign handles POST /api/v1/campaigns
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

    campaign := &models.Campaign{
        Name:         req.Name,
        Status:       models.CampaignStatusDraft,
        Cities:       req.Cities,
        StartDate:    req.StartDate,
        EndDate:      req.EndDate,
        Budget:       req.Budget,
        AdvertiserID: req.AdvertiserID,
        CreatedAt:    time.Now().UTC(),
        UpdatedAt:    time.Now().UTC(),
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

// GetCampaign handles GET /api/v1/campaigns/{id}
func (h *CampaignHandler) GetCampaign(w http.ResponseWriter, r *http.Request) {
    campaignID := chi.URLParam(r, "id")
    if campaignID == "" {
        http.Error(w, "Campaign ID is required", http.StatusBadRequest)
        return
    }

    campaign, err := h.repo.GetByID(r.Context(), campaignID)
    if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "campaign not found",
				})
				return
		}
        http.Error(w, "Failed to fetch campaign: "+err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(campaign)
}

// ListCampaigns handles GET /api/v1/campaigns
func (h *CampaignHandler) ListCampaigns(w http.ResponseWriter, r *http.Request) {
    log.Println("=== ListCampaigns handler called ===")
    
    // Create a default filter
    filter := interfaces.CampaignFilter{
        Limit: 100, // Default limit to prevent loading too many records
    }
    
    campaigns, err := h.repo.List(r.Context(), filter)
    if err != nil {
        http.Error(w, "Failed to list campaigns: "+err.Error(), http.StatusInternalServerError)
        return
    }

    if campaigns == nil {
        campaigns = []*models.Campaign{} // Return empty array instead of null
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(campaigns)
}

// UpdateCampaign handles PUT /api/v1/campaigns/{id}
func (h *CampaignHandler) UpdateCampaign(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if id == "" {
        http.Error(w, "Campaign ID is required", http.StatusBadRequest)
        return
    }

    var req models.UpdateCampaignRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if err := h.validator.Struct(req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // First, get the existing campaign
    existingCampaign, err := h.repo.GetByID(r.Context(), id)
    if err != nil {
        if err == sql.ErrNoRows {
            w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "campaign not found",
			})
			return
        }
        http.Error(w, "Failed to get campaign: "+err.Error(), http.StatusInternalServerError)
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

    // Update the campaign in the database
    err = h.repo.Update(r.Context(), id, existingCampaign)
    if err != nil {
        http.Error(w, "Failed to update campaign: "+err.Error(), http.StatusInternalServerError)
        return
    }

    // Get the updated campaign to return
    updatedCampaign, err := h.repo.GetByID(r.Context(), id)
    if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "campaign not found",
				})
				return
		}
        http.Error(w, "Failed to get updated campaign: "+err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(updatedCampaign)
}

// DeleteCampaign handles DELETE /api/v1/campaigns/{id}
func (h *CampaignHandler) DeleteCampaign(w http.ResponseWriter, r *http.Request) {
    campaignID := chi.URLParam(r, "id")
    log.Println("Deleting campaign with ID:", campaignID)

    if campaignID == "" {
        http.Error(w, "Campaign ID is required", http.StatusBadRequest)
        return
    }

    err := h.repo.Delete(r.Context(), campaignID)
    if err != nil {
        log.Println("Error deleting campaign with ID:", campaignID, "Error:", err)

        if err == sql.ErrNoRows {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusNotFound)
            _ = json.NewEncoder(w).Encode(map[string]string{
                "error": "campaign not found",
            })
            return
        }

        http.Error(w, "Failed to delete campaign: "+err.Error(), http.StatusInternalServerError)
        return
    }

    // Success response
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(map[string]string{
        "message": "campaign deleted successfully",
        "id":      campaignID,
    })
}
