package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"scm/internal/models"
	"scm/internal/repository"
	"scm/internal/services"
)

type SyncHandler struct {
	projectRepo repository.ProjectRepository
	deviceRepo  repository.DeviceRepository
	client      *services.CityPostConsoleClient
}

func NewSyncHandler(projectRepo repository.ProjectRepository, deviceRepo repository.DeviceRepository, client *services.CityPostConsoleClient) *SyncHandler {
	return &SyncHandler{
		projectRepo: projectRepo,
		deviceRepo:  deviceRepo,
		client:      client,
	}
}

type SyncConsoleResponse struct {
	Synced SyncCounts `json:"synced"`
	Errors []string   `json:"errors"`
}

type SyncCounts struct {
	Projects int `json:"projects"`
	Devices  int `json:"devices"`
}

// @Tags Sync
// @Summary Sync projects and devices from CityPost Console
// @Security BearerAuth
// @Produce json
// @Success 200 {object} SyncConsoleResponse
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/sync/console [post]
// SyncConsole orchestrates fetching projects and devices from CityPost Console API and upserting them
func (h *SyncHandler) SyncConsole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resp := SyncConsoleResponse{
		Synced: SyncCounts{},
		Errors: []string{},
	}

	// 1. Fetch projects (production + non-production)
	projectsRaw, err := h.client.ListProjects(ctx)
	if err != nil {
		resp.Errors = append(resp.Errors, "fetch projects: "+err.Error())
		writeJSONErrorResponse(w, http.StatusInternalServerError, "sync_failed", "sync failed: "+err.Error())
		return
	}

	// 2. Upsert projects
	for _, pRaw := range projectsRaw {
		project, err := mapRawToProject(pRaw)
		if err != nil {
			resp.Errors = append(resp.Errors, "map project: "+err.Error())
			continue
		}
		if err := h.projectRepo.Upsert(ctx, project); err != nil {
			resp.Errors = append(resp.Errors, "upsert project "+project.Name+": "+err.Error())
			continue
		}
		resp.Synced.Projects++
	}

	// 3. Login to console API (ensureToken is called per-project in ListDevicesByProject)

	// 4. Loop projects → fetch devices
	for _, pRaw := range projectsRaw {
		projectNameRaw, ok := pRaw["name"]
		if !ok {
			resp.Errors = append(resp.Errors, "project missing 'name'")
			continue
		}
		projectName, ok := projectNameRaw.(string)
		if !ok {
			resp.Errors = append(resp.Errors, "project 'name' not a string")
			continue
		}

		devicesRaw, err := h.client.ListDevicesByProject(ctx, projectName)
		if err != nil {
			resp.Errors = append(resp.Errors, "fetch devices for project "+projectName+": "+err.Error())
			continue
		}

		// 5. Upsert devices
		for _, dRaw := range devicesRaw {
			device, err := mapRawToDevice(dRaw)
			if err != nil {
				resp.Errors = append(resp.Errors, "map device for project "+projectName+": "+err.Error())
				continue
			}
			if err := h.deviceRepo.Upsert(ctx, device); err != nil {
				resp.Errors = append(resp.Errors, "upsert device "+device.HostName+": "+err.Error())
				continue
			}
			resp.Synced.Devices++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// mapRawToProject converts a raw map from the console API to a Project model
func mapRawToProject(raw map[string]any) (*models.Project, error) {
	p := &models.Project{
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if id, ok := raw["id"].(float64); ok {
		p.ID = int(id)
	}
	if name, ok := raw["name"].(string); ok {
		p.Name = name
	}
	if company, ok := raw["company"].(string); ok {
		p.Company = &company
	}
	if desc, ok := raw["description"].(string); ok {
		p.Description = &desc
	}
	if maxDevices, ok := raw["max_devices"].(float64); ok {
		p.MaxDevices = int(maxDevices)
	}
	if profileImg, ok := raw["profile_img"].(string); ok {
		p.ProfileImg = profileImg
	}
	if header, ok := raw["header"].(bool); ok {
		p.Header = header
	}
	if subType, ok := raw["sub_type"].(string); ok {
		p.SubType = subType
	}
	if production, ok := raw["production"].(bool); ok {
		p.Production = production
	}
	if cpf, ok := raw["city_poster_frequency"].(float64); ok {
		p.CityPosterFrequency = int(cpf)
	}
	if apf, ok := raw["ad_poster_frequency"].(float64); ok {
		p.AdPosterFrequency = int(apf)
	}
	if cppt, ok := raw["city_poster_play_time"].(float64); ok {
		p.CityPosterPlayTime = int(cppt)
	}
	if loopLength, ok := raw["loop_length"].(float64); ok {
		p.LoopLength = int(loopLength)
	}
	if smallbizSupport, ok := raw["smallbiz_support"].(bool); ok {
		p.SmallbizSupport = smallbizSupport
	}
	if proxy, ok := raw["proxy"].(string); ok {
		p.Proxy = &proxy
	}
	if address, ok := raw["address"].(string); ok {
		p.Address = &address
	}
	if latitude, ok := raw["latitude"].(string); ok {
		p.Latitude = latitude
	}
	if longitude, ok := raw["longitude"].(string); ok {
		p.Longitude = longitude
	}
	if isTransit, ok := raw["is_transit"].(bool); ok {
		p.IsTransit = isTransit
	}
	if scmHealth, ok := raw["scm_health"].(bool); ok {
		p.ScmHealth = scmHealth
	}
	if priority, ok := raw["priority"].(float64); ok {
		p.Priority = int(priority)
	}
	if replicas, ok := raw["replicas"].(float64); ok {
		p.Replicas = int(replicas)
	}
	if status, ok := raw["status"].(string); ok {
		p.Status = status
	}
	if role, ok := raw["role"].(string); ok {
		p.Role = role
	}

	// Handle owner
	if ownerRaw, ok := raw["owner"].(map[string]any); ok {
		if username, ok := ownerRaw["username"].(string); ok {
			p.Owner = models.Owner{Username: username}
		}
	}

	// Handle languages (array)
	if langsRaw, ok := raw["languages"].([]any); ok {
		languages := make([]string, 0, len(langsRaw))
		for _, item := range langsRaw {
			if s, ok := item.(string); ok {
				languages = append(languages, s)
			}
		}
		p.Languages = languages
	}

	// Handle region (array of ints)
	if regionRaw, ok := raw["region"].([]any); ok {
		region := make([]int, 0, len(regionRaw))
		for _, item := range regionRaw {
			if f, ok := item.(float64); ok {
				region = append(region, int(f))
			}
		}
		p.Region = region
	}

	return p, nil
}

// mapRawToDevice converts a raw map from the console API to a Device model
func mapRawToDevice(raw map[string]any) (*models.Device, error) {
	d := &models.Device{
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if id, ok := raw["id"].(float64); ok {
		d.ID = int(id)
	}
	if name, ok := raw["name"].(string); ok {
		d.Name = name
	}
	if hostName, ok := raw["host_name"].(string); ok {
		d.HostName = hostName
	}
	if description, ok := raw["description"].(string); ok {
		d.Description = description
	}
	if change, ok := raw["change"].(bool); ok {
		d.Change = change
	}
	if project, ok := raw["project"].(float64); ok {
		d.Project = int(project)
	}
	if rttyData, ok := raw["rtty_data"].(float64); ok {
		d.RttyData = int64(rttyData)
	}

	// Handle timestamps
	if lastSyncedRaw, ok := raw["last_synced_at"]; ok {
		if lastSyncedStr, ok := lastSyncedRaw.(string); ok && lastSyncedStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, lastSyncedStr); err == nil {
				d.LastSyncedAt = &t
			}
		}
	}
	if syncStatus, ok := raw["sync_status"].(string); ok {
		d.SyncStatus = &syncStatus
	}

	// Handle nested objects as JSONB
	if dtRaw, ok := raw["device_type"].(map[string]any); ok {
		if bytes, err := json.Marshal(dtRaw); err == nil {
			d.DeviceType = models.DeviceType{}
			if err := json.Unmarshal(bytes, &d.DeviceType); err == nil {
				// Already populated
			}
		}
	}
	if regionRaw, ok := raw["region"].(map[string]any); ok {
		if bytes, err := json.Marshal(regionRaw); err == nil {
			d.Region = models.Region{}
			if err := json.Unmarshal(bytes, &d.Region); err == nil {
				// Already populated
			}
		}
	}
	if configRaw, ok := raw["device_config"].(map[string]any); ok {
		if bytes, err := json.Marshal(configRaw); err == nil {
			d.DeviceConfig = bytes
		}
	}

	return d, nil
}
