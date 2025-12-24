package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"scm/internal/models"
	"scm/internal/repository"
)

type VenueHandler struct {
	repo repository.VenueRepository
}

func NewVenueHandler(repo repository.VenueRepository) *VenueHandler {
	return &VenueHandler{repo: repo}
}

// @Tags Venues
// @Summary Create venue
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body models.Venue true "Create venue request"
// @Success 201 {object} models.Venue
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/venues [post]
func (h *VenueHandler) Create(w http.ResponseWriter, r *http.Request) {
	var venue models.Venue
	if err := json.NewDecoder(r.Body).Decode(&venue); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_json", "Invalid JSON: "+err.Error())
		return
	}

	// Validate required fields
	if venue.Name == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Venue name is required")
		return
	}

	if err := h.repo.Create(r.Context(), &venue); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to create venue: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(venue)
}

// @Tags Venues
// @Summary Get venue
// @Security BearerAuth
// @Produce json
// @Param id path int true "Venue ID"
// @Success 200 {object} models.Venue
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/venues/{id} [get]
func (h *VenueHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Venue ID is required")
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid venue ID")
		return
	}

	venue, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if err.Error() == "venue not found" {
			writeJSONErrorResponse(w, http.StatusNotFound, "not_found", "Venue not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to get venue: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(venue)
}

// @Tags Venues
// @Summary List venues
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/venues [get]
func (h *VenueHandler) List(w http.ResponseWriter, r *http.Request) {
	pagination, err := parsePaginationParams(r, 20, 100)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_pagination", "Invalid pagination: "+err.Error())
		return
	}

	venues, err := h.repo.List(r.Context(), pagination.limit, pagination.offset)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to list venues: "+err.Error())
		return
	}

	total, err := h.repo.Count(r.Context())
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to count venues: "+err.Error())
		return
	}

	writePaginatedResponse(w, http.StatusOK, venues, pagination.page, pagination.pageSize, total)
}

// @Tags Venues
// @Summary Update venue
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "Venue ID"
// @Param body body models.Venue true "Update venue request"
// @Success 200 {object} models.Venue
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/venues/{id} [put]
func (h *VenueHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Venue ID is required")
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid venue ID")
		return
	}

	var venue models.Venue
	if err := json.NewDecoder(r.Body).Decode(&venue); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_json", "Invalid JSON: "+err.Error())
		return
	}

	// Validate required fields
	if venue.Name == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Venue name is required")
		return
	}

	venue.ID = id
	if err := h.repo.Update(r.Context(), &venue); err != nil {
		if err.Error() == "venue not found" {
			writeJSONErrorResponse(w, http.StatusNotFound, "not_found", "Venue not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to update venue: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(venue)
}

// @Tags Venues
// @Summary Delete venue
// @Security BearerAuth
// @Produce json
// @Param id path int true "Venue ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/venues/{id} [delete]
func (h *VenueHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Venue ID is required")
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid venue ID")
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		if err.Error() == "venue not found" {
			writeJSONErrorResponse(w, http.StatusNotFound, "not_found", "Venue not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to delete venue: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Venue deleted successfully"})
}

// @Tags Venues
// @Summary List venues by device
// @Security BearerAuth
// @Produce json
// @Param deviceID path int true "Device ID"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/devices/{deviceID}/venues [get]
func (h *VenueHandler) ListByDevice(w http.ResponseWriter, r *http.Request) {
	deviceIDStr := chi.URLParam(r, "deviceID")
	if deviceIDStr == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Device ID is required")
		return
	}

	deviceID, err := strconv.Atoi(deviceIDStr)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid device ID")
		return
	}

	pagination, err := parsePaginationParams(r, 20, 100)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_pagination", "Invalid pagination: "+err.Error())
		return
	}

	venues, err := h.repo.GetVenuesByDeviceID(r.Context(), deviceID, pagination.limit, pagination.offset)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to list venues by device: "+err.Error())
		return
	}

	total, err := h.repo.CountVenuesByDeviceID(r.Context(), deviceID)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to count venues by device: "+err.Error())
		return
	}

	writePaginatedResponse(w, http.StatusOK, venues, pagination.page, pagination.pageSize, total)
}

// Bulk operations for many-to-many relationships

// @Tags Venues
// @Summary Add devices to venue
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "Venue ID"
// @Param body body object{device_ids=[]int} true "Add devices to venue request"
// @Success 200 {object} map[string]interface{}
// @Success 207 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/venues/{id}/devices [post]
func (h *VenueHandler) AddDevicesToVenue(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Venue ID is required")
		return
	}

	venueID, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid venue ID")
		return
	}

	var request struct {
		DeviceIDs []int `json:"device_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_json", "Invalid JSON: "+err.Error())
		return
	}

	if len(request.DeviceIDs) == 0 {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Device IDs array is required")
		return
	}

	// Add each device to the venue
	var errors []interface{}
	successCount := 0
	venueNotFoundError := false
	deviceNotFoundError := false
	for _, deviceID := range request.DeviceIDs {
		if err := h.repo.AddDeviceToVenue(r.Context(), venueID, deviceID); err != nil {
			if err.Error() == "venue not found" {
				venueNotFoundError = true
			} else if err.Error() == "device not found" {
				deviceNotFoundError = true
			}
			errors = append(errors, map[string]interface{}{
				"device_id": deviceID,
				"error":     err.Error(),
			})
		} else {
			successCount++
		}
	}

	response := map[string]interface{}{
		"added_count": successCount,
		"total_count": len(request.DeviceIDs),
	}
	
	if len(errors) > 0 {
		response["errors"] = errors
		if successCount == 0 {
			// All failed - check for specific errors
			if venueNotFoundError {
				writeJSONErrorResponse(w, http.StatusNotFound, "venue_not_found", "Venue not found")
				return
			}
			if deviceNotFoundError && len(request.DeviceIDs) == 1 {
				// Single device not found
				writeJSONErrorResponse(w, http.StatusNotFound, "device_not_found", "Device not found")
				return
			}
			writeJSONErrorResponse(w, http.StatusBadRequest, "validation_failed", "All devices failed to be added")
			return
		} else {
			// Partial success - use 207 Multi-Status
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMultiStatus)
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Success - follow user_handler pattern
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// @Tags Venues
// @Summary Remove devices from venue
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "Venue ID"
// @Param body body object{device_ids=[]int} true "Remove devices from venue request"
// @Success 200 {object} map[string]interface{}
// @Success 207 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/venues/{id}/devices [delete]
func (h *VenueHandler) RemoveDevicesFromVenue(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Venue ID is required")
		return
	}

	venueID, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid venue ID")
		return
	}

	var request struct {
		DeviceIDs []int `json:"device_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_json", "Invalid JSON: "+err.Error())
		return
	}

	if len(request.DeviceIDs) == 0 {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Device IDs array is required")
		return
	}

	// Remove each device from the venue
	var errors []interface{}
	successCount := 0
	venueNotFoundError := false
	deviceNotFoundError := false
	for _, deviceID := range request.DeviceIDs {
		if err := h.repo.RemoveDeviceFromVenue(r.Context(), venueID, deviceID); err != nil {
			if err.Error() == "venue not found" {
				venueNotFoundError = true
			} else if err.Error() == "device not found" {
				deviceNotFoundError = true
			}
			errors = append(errors, map[string]interface{}{
				"device_id": deviceID,
				"error":     err.Error(),
			})
		} else {
			successCount++
		}
	}

	response := map[string]interface{}{
		"removed_count": successCount,
		"total_count":   len(request.DeviceIDs),
	}
	
	if len(errors) > 0 {
		response["errors"] = errors
		if successCount == 0 {
			// All failed - check for specific errors
			if venueNotFoundError {
				writeJSONErrorResponse(w, http.StatusNotFound, "venue_not_found", "Venue not found")
				return
			}
			if deviceNotFoundError && len(request.DeviceIDs) == 1 {
				// Single device not found
				writeJSONErrorResponse(w, http.StatusNotFound, "device_not_found", "Device not found")
				return
			}
			writeJSONErrorResponse(w, http.StatusBadRequest, "validation_failed", "All devices failed to be removed")
			return
		} else {
			// Partial success - use 207 Multi-Status
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMultiStatus)
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Success - follow user_handler pattern
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// @Tags Venues
// @Summary Get devices by venue
// @Security BearerAuth
// @Produce json
// @Param id path int true "Venue ID"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/venues/{id}/devices [get]
func (h *VenueHandler) GetDevicesByVenue(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Venue ID is required")
		return
	}

	venueID, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid venue ID")
		return
	}

	pagination, err := parsePaginationParams(r, 20, 100)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_pagination", "Invalid pagination: "+err.Error())
		return
	}

	devices, err := h.repo.GetDevicesByVenueID(r.Context(), venueID, pagination.limit, pagination.offset)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to list devices by venue: "+err.Error())
		return
	}

	total, err := h.repo.CountDevicesByVenueID(r.Context(), venueID)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to count devices by venue: "+err.Error())
		return
	}

	writePaginatedResponse(w, http.StatusOK, devices, pagination.page, pagination.pageSize, total)
}
