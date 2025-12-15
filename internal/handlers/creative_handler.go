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
    "scm/internal/models"
    "scm/internal/repository"
)



type CreativeHandler struct {
    repo      repository.CreativeRepository
    s3Client  *s3.Client
    validator *validator.Validate
    bucket    string
    publicBaseURL string
}


func NewCreativeHandler(repo repository.CreativeRepository, s3Config *config.S3Config) *CreativeHandler {
    return &CreativeHandler{
        repo:      repo,
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
func (h *CreativeHandler) UploadCreative(w http.ResponseWriter, r *http.Request) {
    // 1. Parse the multipart form
    const maxMemory = 32 << 20 // 32MB max memory
    if err := r.ParseMultipartForm(maxMemory); err != nil {
        http.Error(w, "Failed to parse form", http.StatusBadRequest)
        return
    }

    campaignID := r.FormValue("campaign_id")
    if campaignID == "" {
        http.Error(w, "Campaign ID is required", http.StatusBadRequest)
        return
    }

    selectedDays := parseFormList(r, "selected_days")
    if len(selectedDays) == 0 {
        http.Error(w, "selected_days is required", http.StatusBadRequest)
        return
    }

    timeSlots := parseFormList(r, "time_slots")
    if len(timeSlots) == 0 {
        http.Error(w, "time_slots is required", http.StatusBadRequest)
        return
    }

    devices := parseFormList(r, "devices")

    // 2. Get the files from the form
    files := r.MultipartForm.File["files"]
    if len(files) == 0 {
        http.Error(w, "No files uploaded", http.StatusBadRequest)
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
        http.Error(w, "Failed to upload any files", http.StatusInternalServerError)
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

func (h *CreativeHandler) ListCreativesByCampaign(w http.ResponseWriter, r *http.Request) {
    campaignID := chi.URLParam(r, "campaignID")
    if campaignID == "" {
        http.Error(w, "Campaign ID is required", http.StatusBadRequest)
        return
    }

    creatives, err := h.repo.ListByCampaign(r.Context(), campaignID)
    if err != nil {
        log.Printf("Failed to list creatives: %v", err)
        http.Error(w, "Failed to list creatives", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(creatives); err != nil {
        log.Printf("Error encoding response: %v", err)
    }
}

func (h *CreativeHandler) ListCreatives(w http.ResponseWriter, r *http.Request) {
    creatives, err := h.repo.ListAll(r.Context())
    if err != nil {
        log.Printf("Failed to list creatives: %v", err)
        http.Error(w, "Failed to list creatives", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(creatives); err != nil {
        log.Printf("Error encoding response: %v", err)
    }
}

// GetCreative handles GET /creatives/{id}
func (h *CreativeHandler) GetCreative(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if id == "" {
        http.Error(w, "Creative ID is required", http.StatusBadRequest)
        return
    }

    creative, err := h.repo.GetByID(r.Context(), id)
    if err != nil {
        if err == sql.ErrNoRows {
            http.Error(w, "Creative not found", http.StatusNotFound)
            return
        }
        log.Printf("Failed to get creative: %v", err)
        http.Error(w, "Failed to get creative", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(creative); err != nil {
        log.Printf("Error encoding response: %v", err)
    }
}

// UpdateCreative handles PUT /creatives/{id}
func (h *CreativeHandler) UpdateCreative(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if id == "" {
        http.Error(w, "Creative ID is required", http.StatusBadRequest)
        return
    }

    var req models.UpdateCreativeRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if err := h.validator.Struct(req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    if err := h.repo.Update(r.Context(), id, &req); err != nil {
        if err == sql.ErrNoRows {
            http.Error(w, "Creative not found", http.StatusNotFound)
            return
        }
        log.Printf("Failed to update creative: %v", err)
        http.Error(w, "Failed to update creative", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}
// DeleteCreative handles DELETE /creatives/{id}
func (h *CreativeHandler) DeleteCreative(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if id == "" {
        http.Error(w, "Creative ID is required", http.StatusBadRequest)
        return
    }

    if err := h.repo.Delete(r.Context(), id); err != nil {
        if err == sql.ErrNoRows {
            http.Error(w, "Creative not found", http.StatusNotFound)
            return
        }
        log.Printf("Failed to delete creative: %v", err)
        http.Error(w, "Failed to delete creative", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}