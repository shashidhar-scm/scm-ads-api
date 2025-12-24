package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"scm/internal/models"
	"scm/internal/repository"
)

type DeviceReadHandler struct {
	repo repository.DeviceRepository
}

func NewDeviceReadHandler(repo repository.DeviceRepository) *DeviceReadHandler {
	return &DeviceReadHandler{repo: repo}
}

// @Tags Devices
// @Summary List devices
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param project_id query int false "Filter by project ID"
// @Param city query string false "Filter by city"
// @Param region query string false "Filter by region"
// @Param device_type query string false "Filter by device type"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/devices [get]
func (h *DeviceReadHandler) List(w http.ResponseWriter, r *http.Request) {
	pagination, err := parsePaginationParams(r, 20, 100)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_pagination", "invalid pagination: "+err.Error())
		return
	}

	// Parse filter parameters
	filters := repository.DeviceFilters{}

	if projectIDStr := r.URL.Query().Get("project_id"); projectIDStr != "" {
		if projectID, err := strconv.Atoi(projectIDStr); err == nil {
			filters.ProjectID = &projectID
		}
	}

	if city := r.URL.Query().Get("city"); city != "" {
		filters.City = &city
	}

	if region := r.URL.Query().Get("region"); region != "" {
		filters.Region = &region
	}

	if deviceType := r.URL.Query().Get("device_type"); deviceType != "" {
		filters.DeviceType = &deviceType
	}

	var devices []*models.Device
	var total int

	// Use filters if any are provided, otherwise use basic list
	if filters.ProjectID != nil || filters.City != nil || filters.Region != nil || filters.DeviceType != nil {
		devices, err = h.repo.ListWithFilters(r.Context(), filters, pagination.limit, pagination.offset)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to list devices with filters: "+err.Error())
			return
		}

		total, err = h.repo.CountWithFilters(r.Context(), filters)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to count devices with filters: "+err.Error())
			return
		}
	} else {
		devices, err = h.repo.List(r.Context(), pagination.limit, pagination.offset)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to list devices: "+err.Error())
			return
		}

		total, err = h.repo.Count(r.Context())
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to count devices: "+err.Error())
			return
		}
	}

	writePaginatedResponse(w, http.StatusOK, devices, pagination.page, pagination.pageSize, total)
}

// @Tags Devices
// @Summary Get device
// @Security BearerAuth
// @Produce json
// @Param hostName path string true "Device host name"
// @Success 200 {object} models.Device
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/devices/{hostName} [get]
func (h *DeviceReadHandler) Get(w http.ResponseWriter, r *http.Request) {
	hostName := chi.URLParam(r, "hostName")
	if hostName == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "device hostName is required")
		return
	}

	device, err := h.repo.GetByHostName(r.Context(), hostName)
	if err != nil {
		if err.Error() == "device not found" {
			writeJSONErrorResponse(w, http.StatusNotFound, "not_found", "device not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to get device: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(device)
}

func (h *DeviceReadHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	projectIDStr := chi.URLParam(r, "projectID")
	if projectIDStr == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "projectID is required")
		return
	}
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "invalid projectID")
		return
	}

	pagination, err := parsePaginationParams(r, 20, 100)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_pagination", "invalid pagination: "+err.Error())
		return
	}

	devices, err := h.repo.ListByProject(r.Context(), projectID, pagination.limit, pagination.offset)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to list devices by project: "+err.Error())
		return
	}

	total, err := h.repo.CountByProject(r.Context(), projectID)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to count devices by project: "+err.Error())
		return
	}

	writePaginatedResponse(w, http.StatusOK, devices, pagination.page, pagination.pageSize, total)
}
