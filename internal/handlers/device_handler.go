package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

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

type DeviceQueryRequest struct {
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
	Filters  DeviceQueryFilters  `json:"filters"`
	Fields   []string            `json:"fields"`
}

type DeviceQueryFilters struct {
	ProjectID  *int    `json:"project_id"`
	City       *string `json:"city"`
	Region     *string `json:"region"`
	DeviceType *string `json:"device_type"`
	Test       *bool   `json:"test"`
}

func hasDeviceFilters(filters repository.DeviceFilters) bool {
	return filters.ProjectID != nil ||
		filters.City != nil ||
		filters.Region != nil ||
		filters.DeviceType != nil ||
		filters.Test != nil
}

// @Tags Devices
// @Summary Query devices with filters and selectable fields
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body DeviceQueryRequest true "Device query request with filters and optional fields"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/devices/query [post]
func (h *DeviceReadHandler) Query(w http.ResponseWriter, r *http.Request) {
	var req DeviceQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "invalid JSON body: "+err.Error())
		return
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	limit := pageSize
	offset := (page - 1) * pageSize

	filters := repository.DeviceFilters{
		ProjectID:  req.Filters.ProjectID,
		City:       req.Filters.City,
		Region:     req.Filters.Region,
		DeviceType: req.Filters.DeviceType,
		Test:       req.Filters.Test,
	}

	var (
		devices []*models.Device
		total   int
		err     error
	)

	if hasDeviceFilters(filters) {
		devices, err = h.repo.ListWithFilters(r.Context(), filters, limit, offset)
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
		devices, err = h.repo.List(r.Context(), limit, offset)
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

	data := applyDeviceFieldFilter(devices, req.Fields)
	writePaginatedResponse(w, http.StatusOK, data, page, pageSize, total)
}

type fieldNode struct {
	children map[string]*fieldNode
	terminal bool
}

func applyDeviceFieldFilter(devices []*models.Device, fields []string) any {
	cleanFields := sanitizeFields(fields)
	if len(cleanFields) == 0 {
		return devices
	}

	tree := buildFieldTree(cleanFields)
	if len(tree) == 0 {
		return devices
	}

	var filtered []map[string]any
	for _, device := range devices {
		raw := deviceToMap(device)
		if raw == nil {
			continue
		}
		pruned := filterMapWithTree(raw, tree)
		if len(pruned) == 0 {
			continue
		}
		filtered = append(filtered, pruned)
	}

	return filtered
}

func sanitizeFields(fields []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		out = append(out, field)
	}
	return out
}

func buildFieldTree(fields []string) map[string]*fieldNode {
	root := make(map[string]*fieldNode)
	for _, field := range fields {
		parts := strings.Split(field, ".")
		curr := root
		var node *fieldNode
		for _, part := range parts {
			if curr[part] == nil {
				curr[part] = &fieldNode{children: make(map[string]*fieldNode)}
			}
			node = curr[part]
			curr = node.children
		}
		if node != nil {
			node.terminal = true
		}
	}
	return root
}

func deviceToMap(device *models.Device) map[string]any {
	if device == nil {
		return nil
	}
	bytes, err := json.Marshal(device)
	if err != nil {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(bytes, &raw); err != nil {
		return nil
	}
	return raw
}

func filterMapWithTree(raw map[string]any, nodes map[string]*fieldNode) map[string]any {
	out := make(map[string]any)
	for key, node := range nodes {
		value, ok := raw[key]
		if !ok {
			continue
		}
		if filtered := filterValueWithNode(value, node); filtered != nil {
			out[key] = filtered
		}
	}
	return out
}

func filterValueWithNode(value any, node *fieldNode) any {
	if node == nil || node.terminal || len(node.children) == 0 {
		return value
	}

	switch v := value.(type) {
	case map[string]any:
		child := filterMapWithTree(v, node.children)
		if len(child) == 0 {
			return nil
		}
		return child
	case []any:
		var arr []any
		for _, item := range v {
			if itemMap, ok := item.(map[string]any); ok {
				child := filterMapWithTree(itemMap, node.children)
				if len(child) > 0 {
					arr = append(arr, child)
				}
			}
		}
		if len(arr) == 0 {
			return nil
		}
		return arr
	default:
		return nil
	}
}

// @Tags Devices
// @Summary Search devices
// @Security BearerAuth
// @Produce json
// @Param query query string true "Search text (matches host_name, name, description, address)"
// @Param limit query int false "Maximum results to return" default(25)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/devices/search [get]
func (h *DeviceReadHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "query is required")
		return
	}

	pagination, err := parsePaginationParams(r, 25, 100)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_pagination", "invalid pagination: "+err.Error())
		return
	}

	devices, total, err := h.repo.Search(r.Context(), query, pagination.limit, pagination.offset)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to search devices: "+err.Error())
		return
	}

	writePaginatedResponse(w, http.StatusOK, devices, pagination.page, pagination.pageSize, total)
}

// @Tags Devices
// @Summary Region-wise device counts
// @Security BearerAuth
// @Produce json
// @Param city query string false "Filter by city"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/devices/counts/regions [get]
func (h *DeviceReadHandler) CountByRegion(w http.ResponseWriter, r *http.Request) {
	var city *string
	if v := r.URL.Query().Get("city"); v != "" {
		city = &v
	}

	items, err := h.repo.CountByRegion(r.Context(), city)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to count devices by region: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": items,
	})
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
// @Param test query bool false "Filter by test flag (device_config.test)"
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

	if testParam := r.URL.Query().Get("test"); testParam != "" {
		value, err := strconv.ParseBool(testParam)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_test_filter", "test must be true or false")
			return
		}
		filters.Test = &value
	}

	var devices []*models.Device
	var total int

	// Use filters if any are provided, otherwise use basic list
	if filters.ProjectID != nil || filters.City != nil || filters.Region != nil || filters.DeviceType != nil || filters.Test != nil {
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
