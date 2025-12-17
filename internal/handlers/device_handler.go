package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"scm/internal/config"
	"scm/internal/services"
)

type DeviceHandler struct {
	client *services.CityPostConsoleClient
}

type DeviceResponse struct {
	DeviceName string `json:"device_name"`
	HostName   string `json:"host_name"`
}

type DevicesListResponse struct {
	Total   int             `json:"total"`
	Devices []DeviceResponse `json:"devices"`
}

func NewDeviceHandler(client *services.CityPostConsoleClient) *DeviceHandler {
	return &DeviceHandler{client: client}
}


func NewDeviceHandlerFromConfig(cfg *config.Config) *DeviceHandler {
	if cfg == nil {
		return NewDeviceHandler(nil)
	}
	baseURL := strings.TrimRight(cfg.CityPostConsoleBaseURL, "/")
	client := services.NewCityPostConsoleClient(baseURL, cfg.CityPostConsoleUsername, cfg.CityPostConsolePassword)
	if strings.TrimSpace(cfg.CityPostConsoleAuthScheme) != "" {
		client.SetAuthScheme(cfg.CityPostConsoleAuthScheme)
	}
	return NewDeviceHandler(client)
}

// @Tags Devices
// @Summary List devices
// @Security BearerAuth
// @Produce json
// @Param target query []string false "Repeatable project:region pairs (e.g. target=kcmo:kc). If provided, project/region are ignored."
// @Param project query string false "Project (e.g. kcmo)"
// @Param region query string false "Region (e.g. kc)"
// @Success 200 {object} handlers.DevicesListResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 502 {object} map[string]interface{}
// @Router /api/v1/devices [get]
func (h *DeviceHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	if h.client == nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "server_error", "device client not configured")
		return
	}

	targets := r.URL.Query()["target"]
	if len(targets) == 0 {
		project := strings.TrimSpace(r.URL.Query().Get("project"))
		region := strings.TrimSpace(r.URL.Query().Get("region"))
		if project == "" || region == "" {
			writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "either target=project:region (repeatable) or project and region are required")
			return
		}
		targets = []string{project + ":" + region}
	}

	seen := make(map[string]struct{})
	resp := make([]DeviceResponse, 0)

	for _, t := range targets {
		parts := strings.SplitN(strings.TrimSpace(t), ":", 2)
		if len(parts) != 2 {
			writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "invalid target format; expected project:region")
			return
		}
		project := strings.TrimSpace(parts[0])
		region := strings.TrimSpace(parts[1])
		if project == "" || region == "" {
			writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "invalid target; project and region are required")
			return
		}

		devices, err := h.client.ListDevices(r.Context(), project, region)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusBadGateway, "upstream_error", err.Error())
			return
		}

		for _, d := range devices {
			key := d.DeviceName + "|" + d.HostName
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			resp = append(resp, DeviceResponse{DeviceName: d.DeviceName, HostName: d.HostName})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(DevicesListResponse{Total: len(resp), Devices: resp})
}
