package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/go-chi/chi/v5"
    "github.com/go-playground/validator/v10"
    "github.com/google/uuid"
    "scm/internal/config"
    "scm/internal/interfaces"
    "scm/internal/models"
    "scm/internal/repository"
)



type CreativeHandler struct {
    repo      repository.CreativeRepository
    campaignRepo interfaces.CampaignRepository
    s3Client  *s3.Client
    validator *validator.Validate
    bucket    string
    publicBaseURL string
}


func NewCreativeHandler(repo repository.CreativeRepository, campaignRepo interfaces.CampaignRepository, s3Config *config.S3Config) *CreativeHandler {
    return &CreativeHandler{
        repo:      repo,
        campaignRepo: campaignRepo,
        s3Client:  s3Config.Client,
        bucket:    s3Config.Bucket,
        publicBaseURL: s3Config.PublicBaseURL,
        validator: validator.New(),
    }
}

// generateUUID generates a new UUID
func generateUUID() string {
    return uuid.New().String()
}

func parseFormList(r *http.Request, key string) []string {
    if r.MultipartForm == nil {
        return nil
    }

    var out []string
    if vs := r.MultipartForm.Value[key]; len(vs) > 0 {
        for _, v := range vs {
            for _, part := range strings.Split(v, ",") {
                part = strings.TrimSpace(part)
                if part == "" {
                    continue
                }
                out = append(out, part)
            }
        }
    }
    return out
}

// UploadCreative handles multiple file uploads to S3
// @Tags Creatives
// @Summary Upload creatives
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param campaign_id formData string true "Campaign ID"
// @Param selected_days formData string true "Selected days (comma separated or repeated)"
// @Param time_slots formData string true "Time slots (comma separated or repeated)"
// @Param devices formData string false "Devices (comma separated or repeated)"
// @Param files formData file true "Creative files"
// @Success 201 {array} models.Creative
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/creatives/upload [post]
func (h *CreativeHandler) UploadCreative(w http.ResponseWriter, r *http.Request) {
    // 1. Parse the multipart form
    const maxMemory = 32 << 20 // 32MB max memory
    if err := r.ParseMultipartForm(maxMemory); err != nil {
        writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Failed to parse form")
        return
    }

    campaignID := r.FormValue("campaign_id")
    if campaignID == "" {
        writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "campaign_id is required")
        return
    }

    if _, err := uuid.Parse(campaignID); err != nil {
        writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "campaign_id must be a valid UUID")
        return
    }

    if h.campaignRepo == nil {
        writeJSONErrorResponse(w, http.StatusInternalServerError, "server_error", "campaign repository not configured")
        return
    }

    if _, err := h.campaignRepo.GetByID(r.Context(), campaignID); err != nil {
        if err == sql.ErrNoRows {
            writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "campaign_id not found")
            return
        }
        log.Printf("Failed to validate campaign %s: %v", campaignID, err)
        writeJSONErrorResponse(w, http.StatusInternalServerError, "server_error", "Failed to validate campaign")
        return
    }

    selectedDays := parseFormList(r, "selected_days")
    if len(selectedDays) == 0 {
        writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "selected_days is required")
        return
    }

    timeSlots := parseFormList(r, "time_slots")
    if len(timeSlots) == 0 {
        writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "time_slots is required")
        return
    }

    devices := parseFormList(r, "devices")

    // 2. Get the files from the form
    files := r.MultipartForm.File["files"]
    if len(files) == 0 {
        writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "No files uploaded")
        return
    }

    var uploadedCreatives []*models.Creative
    uploader := manager.NewUploader(h.s3Client)

    // 5. Process each file
    for _, fileHeader := range files {
        // Open the file
        file, err := fileHeader.Open()
        if err != nil {
            log.Printf("Failed to open file %s: %v", fileHeader.Filename, err)
            continue
        }

        // Create a new creative
        creative := &models.Creative{
            ID:           generateUUID(),
            Name:         fileHeader.Filename,
            Type:         getFileType(fileHeader),
            Size:         fileHeader.Size,
            CampaignID:   campaignID,
            SelectedDays: selectedDays,
            TimeSlots:    timeSlots,
            Devices:      devices,
            UploadedAt:   time.Now().UTC(),
        }

        // Upload to S3
        key := filepath.Join("creatives", creative.ID+filepath.Ext(fileHeader.Filename))
        
        _, err = uploader.Upload(r.Context(), &s3.PutObjectInput{
            Bucket: aws.String(h.bucket),
            Key:    aws.String(key),
            Body:   file,
        })
        file.Close() // Close the file when done

        if err != nil {
            log.Printf("Failed to upload file %s to S3: %v", fileHeader.Filename, err)
            continue
        }

        // Set the URL
        creative.URL = strings.TrimRight(h.publicBaseURL, "/") + "/" + key

        // Store the object key internally
        creative.FilePath = key

        // Save to database
        if err := h.repo.Create(r.Context(), creative); err != nil {
            log.Printf("Failed to save creative %s: %v", fileHeader.Filename, err)
            continue
        }

        uploadedCreatives = append(uploadedCreatives, creative)
    }

    // 6. Return the uploaded creatives
    if len(uploadedCreatives) == 0 {
        writeJSONErrorResponse(w, http.StatusInternalServerError, "upload_failed", "Failed to upload any files")
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    if err := json.NewEncoder(w).Encode(uploadedCreatives); err != nil {
        log.Printf("Error encoding response: %v", err)
    }
}

// Helper function to determine file type
func getFileType(header *multipart.FileHeader) models.CreativeType {
    switch header.Header.Get("Content-Type") {
    case "image/jpeg", "image/png", "image/gif":
        return models.CreativeTypeImage
    case "video/mp4", "video/quicktime":
        return models.CreativeTypeVideo
    default:
        return models.CreativeTypeImage
    }
}

// @Tags Creatives
// @Summary List creatives by campaign
// @Security BearerAuth
// @Produce json
// @Param campaignID path string true "Campaign ID"
// @Success 200 {array} models.Creative
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/creatives/campaign/{campaignID} [get]
func (h *CreativeHandler) ListCreativesByCampaign(w http.ResponseWriter, r *http.Request) {
    campaignID := chi.URLParam(r, "campaignID")
    if campaignID == "" {
        writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "campaignID is required")
        return
    }

	p, err := parsePaginationParams(r, 50, 200)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "invalid pagination parameters")
		return
	}

	total, err := h.repo.CountByCampaign(r.Context(), campaignID)
	if err != nil {
		log.Printf("Failed to count creatives: %v", err)
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_creatives_failed", "Failed to list creatives")
		return
	}

    creatives, err := h.repo.ListByCampaign(r.Context(), campaignID, p.limit, p.offset)
    if err != nil {
        log.Printf("Failed to list creatives: %v", err)
        writeJSONErrorResponse(w, http.StatusInternalServerError, "list_creatives_failed", "Failed to list creatives")
        return
    }

	if creatives == nil {
		creatives = []*models.Creative{}
	}

	writePaginatedResponse(w, http.StatusOK, creatives, p.page, p.pageSize, total)
}

// @Tags Creatives
// @Summary List creatives
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Creative
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/creatives/ [get]
func (h *CreativeHandler) ListCreatives(w http.ResponseWriter, r *http.Request) {
	p, err := parsePaginationParams(r, 50, 200)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "invalid pagination parameters")
		return
	}

	total, err := h.repo.CountAll(r.Context())
	if err != nil {
		log.Printf("Failed to count creatives: %v", err)
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_creatives_failed", "Failed to list creatives")
		return
	}

    creatives, err := h.repo.ListAll(r.Context(), p.limit, p.offset)
    if err != nil {
        log.Printf("Failed to list creatives: %v", err)
        writeJSONErrorResponse(w, http.StatusInternalServerError, "list_creatives_failed", "Failed to list creatives")
        return
    }

	if creatives == nil {
		creatives = []*models.Creative{}
	}

	writePaginatedResponse(w, http.StatusOK, creatives, p.page, p.pageSize, total)
}

// @Tags Creatives
// @Summary List creatives by device
// @Produce json
// @Param device path string true "Device name"
// @Param active_now query bool false "Filter by current day and time"
// @Success 200 {array} models.Creative
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/creatives/device/{device} [get]
func (h *CreativeHandler) ListCreativesByDevice(w http.ResponseWriter, r *http.Request) {
	device := chi.URLParam(r, "device")
	if device == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "device is required")
		return
	}

	p, err := parsePaginationParams(r, 50, 200)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "invalid pagination parameters")
		return
	}

	activeNow := strings.EqualFold(r.URL.Query().Get("active_now"), "true") || r.URL.Query().Get("active_now") == "1"

	now := time.Now().UTC()

	total, err := h.repo.CountByDevice(r.Context(), device, activeNow, now)
	if err != nil {
		log.Printf("Failed to count creatives by device: %v", err)
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_creatives_failed", "Failed to list creatives")
		return
	}

	creatives, err := h.repo.ListByDevice(r.Context(), device, activeNow, now, p.limit, p.offset)
	if err != nil {
		log.Printf("Failed to list creatives by device: %v", err)
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_creatives_failed", "Failed to list creatives")
		return
	}

	if creatives == nil {
		creatives = []*models.Creative{}
	}

	writePaginatedResponse(w, http.StatusOK, creatives, p.page, p.pageSize, total)
}

// GetCreative handles GET /creatives/{id}
// @Tags Creatives
// @Summary Get creative
// @Security BearerAuth
// @Produce json
// @Param id path string true "Creative ID"
// @Success 200 {object} models.Creative
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/creatives/{id}/ [get]
func (h *CreativeHandler) GetCreative(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if id == "" {
        writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "Creative ID is required")
        return
    }

    creative, err := h.repo.GetByID(r.Context(), id)
    if err != nil {
        if err == sql.ErrNoRows {
            writeJSONErrorResponse(w, http.StatusNotFound, "creative_not_found", "Creative not found")
            return
        }
        log.Printf("Failed to get creative: %v", err)
        writeJSONErrorResponse(w, http.StatusInternalServerError, "get_creative_failed", "Failed to get creative")
        return
    }

    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(creative); err != nil {
        log.Printf("Error encoding response: %v", err)
    }
}

// UpdateCreative handles PUT /creatives/{id}
// @Tags Creatives
// @Summary Update creative
// @Security BearerAuth
// @Accept json
// @Accept multipart/form-data
// @Produce json
// @Param id path string true "Creative ID"
// @Param body body models.UpdateCreativeRequest false "Update creative request (JSON)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/creatives/{id}/ [put]
func (h *CreativeHandler) UpdateCreative(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if id == "" {
        writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "Creative ID is required")
        return
    }

    if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
        const maxMemory = 32 << 20
        if err := r.ParseMultipartForm(maxMemory); err != nil {
            writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Failed to parse form")
            return
        }

        var req models.UpdateCreativeRequest

        if name := r.FormValue("name"); name != "" {
            req.Name = &name
        }

        if r.MultipartForm != nil {
            if _, ok := r.MultipartForm.Value["selected_days"]; ok {
                v := parseFormList(r, "selected_days")
                req.SelectedDays = &v
            }
            if _, ok := r.MultipartForm.Value["time_slots"]; ok {
                v := parseFormList(r, "time_slots")
                req.TimeSlots = &v
            }
            if _, ok := r.MultipartForm.Value["devices"]; ok {
                v := parseFormList(r, "devices")
                req.Devices = &v
            }
        }

        var fileHeader *multipart.FileHeader
        if r.MultipartForm != nil {
            if fhs := r.MultipartForm.File["file"]; len(fhs) > 0 {
                fileHeader = fhs[0]
            } else if fhs := r.MultipartForm.File["files"]; len(fhs) > 0 {
                fileHeader = fhs[0]
            }
        }

        if fileHeader != nil {
            file, err := fileHeader.Open()
            if err != nil {
                writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Failed to open uploaded file")
                return
            }
            defer file.Close()

            key := filepath.Join("creatives", id+filepath.Ext(fileHeader.Filename))
            uploader := manager.NewUploader(h.s3Client)
            _, err = uploader.Upload(r.Context(), &s3.PutObjectInput{
                Bucket: aws.String(h.bucket),
                Key:    aws.String(key),
                Body:   file,
            })
            if err != nil {
                log.Printf("Failed to upload file %s to S3: %v", fileHeader.Filename, err)
                writeJSONErrorResponse(w, http.StatusBadGateway, "upload_failed", "Failed to upload file")
                return
            }

            url := strings.TrimRight(h.publicBaseURL, "/") + "/" + key
            req.URL = &url
            req.FilePath = &key
            size := fileHeader.Size
            req.Size = &size
            t := getFileType(fileHeader)
            req.Type = &t

            if req.Name == nil {
                n := fileHeader.Filename
                req.Name = &n
            }
        }

        if err := h.validator.Struct(req); err != nil {
            writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error())
            return
        }

        if err := h.repo.Update(r.Context(), id, &req); err != nil {
            if err == sql.ErrNoRows {
                writeJSONErrorResponse(w, http.StatusNotFound, "creative_not_found", "Creative not found")
                return
            }
            log.Printf("Failed to update creative: %v", err)
            writeJSONErrorResponse(w, http.StatusInternalServerError, "update_creative_failed", "Failed to update creative")
            return
        }

        writeJSONMessage(w, http.StatusOK, "creative updated successfully")
        return
    }

    var req models.UpdateCreativeRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
        return
    }

    if err := h.validator.Struct(req); err != nil {
        writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error())
        return
    }

    if err := h.repo.Update(r.Context(), id, &req); err != nil {
        if err == sql.ErrNoRows {
            writeJSONErrorResponse(w, http.StatusNotFound, "creative_not_found", "Creative not found")
            return
        }
        log.Printf("Failed to update creative: %v", err)
        writeJSONErrorResponse(w, http.StatusInternalServerError, "update_creative_failed", "Failed to update creative")
        return
    }

    writeJSONMessage(w, http.StatusOK, "creative updated successfully")
}
// DeleteCreative handles DELETE /creatives/{id}
// @Tags Creatives
// @Summary Delete creative
// @Security BearerAuth
// @Produce json
// @Param id path string true "Creative ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/creatives/{id}/ [delete]
func (h *CreativeHandler) DeleteCreative(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if id == "" {
        writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "Creative ID is required")
        return
    }

    if err := h.repo.Delete(r.Context(), id); err != nil {
        if err == sql.ErrNoRows {
            writeJSONErrorResponse(w, http.StatusNotFound, "creative_not_found", "Creative not found")
            return
        }
        log.Printf("Failed to delete creative: %v", err)
        writeJSONErrorResponse(w, http.StatusInternalServerError, "delete_creative_failed", "Failed to delete creative")
        return
    }

    writeJSONMessage(w, http.StatusOK, "creative deleted successfully")
}