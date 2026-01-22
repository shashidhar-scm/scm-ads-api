package handlers


import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/rekognition/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"scm/internal/config"
	"scm/internal/interfaces"
	"scm/internal/models"
	"scm/internal/repository"
)



type CreativeHandler struct {
    repo      repository.CreativeRepository
    campaignRepo interfaces.CampaignRepository
    s3Client  *s3.Client
	rekognitionClient *rekognition.Client
    validator *validator.Validate
    bucket    string
    publicBaseURL string
	db        *sql.DB
}


func NewCreativeHandler(repo repository.CreativeRepository, campaignRepo interfaces.CampaignRepository, s3Config *config.S3Config, db *sql.DB) *CreativeHandler {
    return &CreativeHandler{
        repo:      repo,
        campaignRepo: campaignRepo,
        s3Client:  s3Config.Client,
        bucket:    s3Config.Bucket,
        publicBaseURL: s3Config.PublicBaseURL,
        validator: validator.New(),
		db:        db,
    }
}

func getEnvOrDefault(key string, defaultValue string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return defaultValue
	}
	return v
}

func (h *CreativeHandler) ListCreativesByDevice(w http.ResponseWriter, r *http.Request) {
	device := strings.TrimSpace(chi.URLParam(r, "device"))
	if device == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "device is required")
		return
	}

	p, err := parsePaginationParams(r, 50, 200)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "invalid pagination parameters")
		return
	}

	activeNow := false
	if raw := strings.TrimSpace(r.URL.Query().Get("active_now")); raw != "" {
		activeNow = strings.EqualFold(raw, "true") || raw == "1" || strings.EqualFold(raw, "yes")
	}

	now := time.Now().UTC()
	items, err := h.repo.ListByDevice(r.Context(), device, activeNow, now, 0, 0)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_creatives_failed", "Failed to list creatives")
		return
	}
	if items == nil {
		items = []*models.Creative{}
	}

	byCampaign := make(map[string][]*models.Creative)
	for _, c := range items {
		if c == nil {
			continue
		}
		cid := strings.TrimSpace(c.CampaignID)
		if cid == "" {
			continue
		}
		byCampaign[cid] = append(byCampaign[cid], c)
	}

	campaignIDs := make([]string, 0, len(byCampaign))
	for cid := range byCampaign {
		campaignIDs = append(campaignIDs, cid)
	}
	sort.Strings(campaignIDs)

	selected := make([]*models.Creative, 0, len(campaignIDs))
	for _, campaignID := range campaignIDs {
		cs := byCampaign[campaignID]
		if len(cs) == 0 {
			continue
		}
		if len(cs) == 1 {
			selected = append(selected, cs[0])
			continue
		}
		// Stable order for the rotation cycle.
		sort.Slice(cs, func(i, j int) bool {
			if cs[i] == nil {
				return false
			}
			if cs[j] == nil {
				return true
			}
			if cs[i].UploadedAt.Equal(cs[j].UploadedAt) {
				return cs[i].ID < cs[j].ID
			}
			return cs[i].UploadedAt.Before(cs[j].UploadedAt)
		})

		candidateIDs := make([]string, 0, len(cs))
		for _, c := range cs {
			if c == nil {
				continue
			}
			candidateIDs = append(candidateIDs, c.ID)
		}
		nextID, err := h.repo.PickNextRotationalCreative(r.Context(), device, campaignID, candidateIDs)
		if err != nil {
			selected = append(selected, cs[0])
			continue
		}
		picked := cs[0]
		for _, c := range cs {
			if c != nil && c.ID == nextID {
				picked = c
				break
			}
		}
		selected = append(selected, picked)
	}

	total := len(selected)
	start := p.offset
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}
	end := start + p.limit
	if end > total {
		end = total
	}

	pageItems := selected[start:end]
	if pageItems == nil {
		pageItems = []*models.Creative{}
	}

	writePaginatedResponse(w, http.StatusOK, pageItems, p.page, p.pageSize, total)
}

func (h *CreativeHandler) SearchCreatives(w http.ResponseWriter, r *http.Request) {
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

	items, total, err := h.repo.Search(r.Context(), query, p.limit, p.offset, nil)
	if err != nil {
		log.Printf("SearchCreatives failed: query=%q err=%v", query, err)
		writeJSONErrorResponse(w, http.StatusInternalServerError, "search_creatives_failed", "Failed to search creatives")
		return
	}
	if items == nil {
		items = []*models.Creative{}
	}
	writePaginatedResponse(w, http.StatusOK, items, p.page, p.pageSize, total)
}

func (h *CreativeHandler) ListCreatives(w http.ResponseWriter, r *http.Request) {
	p, err := parsePaginationParams(r, 50, 200)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "invalid pagination parameters")
		return
	}

	items, err := h.repo.ListAll(r.Context(), p.limit, p.offset, nil)
	if err != nil {
		log.Printf("ListCreatives failed: err=%v", err)
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_creatives_failed", "Failed to list creatives")
		return
	}
	total, err := h.repo.CountAll(r.Context(), nil)
	if err != nil {
		log.Printf("ListCreatives count failed: err=%v", err)
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_creatives_failed", "Failed to list creatives")
		return
	}
	if items == nil {
		items = []*models.Creative{}
	}
	writePaginatedResponse(w, http.StatusOK, items, p.page, p.pageSize, total)
}

func (h *CreativeHandler) ListCreativesByCampaign(w http.ResponseWriter, r *http.Request) {
	campaignID := strings.TrimSpace(chi.URLParam(r, "campaignID"))
	if campaignID == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "campaignID is required")
		return
	}

	p, err := parsePaginationParams(r, 50, 200)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "invalid pagination parameters")
		return
	}

	items, err := h.repo.ListByCampaign(r.Context(), campaignID, p.limit, p.offset)
	if err != nil {
		log.Printf("ListCreativesByCampaign failed: campaign_id=%s err=%v", campaignID, err)
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_creatives_failed", "Failed to list creatives")
		return
	}
	total, err := h.repo.CountByCampaign(r.Context(), campaignID)
	if err != nil {
		log.Printf("ListCreativesByCampaign count failed: campaign_id=%s err=%v", campaignID, err)
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_creatives_failed", "Failed to list creatives")
		return
	}
	if items == nil {
		items = []*models.Creative{}
	}
	writePaginatedResponse(w, http.StatusOK, items, p.page, p.pageSize, total)
}

func (h *CreativeHandler) GetCreative(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
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
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_creative_failed", "Failed to get creative")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(creative)
}

func isSupportedImageContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	switch ct {
	case "image/jpeg", "image/jpg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

func tokenizeForSuggestions(text string) []string {
	text = strings.ToLower(text)
	var b strings.Builder
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' {
			b.WriteRune(r)
			continue
		}
		b.WriteRune(' ')
	}
	parts := strings.Fields(b.String())
	stop := map[string]struct{}{
		"the": {}, "and": {}, "or": {}, "a": {}, "an": {}, "to": {}, "for": {}, "of": {}, "in": {}, "on": {}, "at": {}, "with": {},
		"is": {}, "are": {}, "be": {}, "this": {}, "that": {}, "your": {}, "you": {}, "we": {}, "our": {},
	}
	seen := make(map[string]struct{}, len(parts))
	var out []string
	for _, p := range parts {
		if len(p) < 3 {
			continue
		}
		if _, ok := stop[p]; ok {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func normalizeToken(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	if t == "" {
		return ""
	}
	// very small stemmer / normalizer
	if strings.HasSuffix(t, "es") && len(t) > 4 {
		t = strings.TrimSuffix(t, "es")
	} else if strings.HasSuffix(t, "s") && len(t) > 3 {
		t = strings.TrimSuffix(t, "s")
	}
	return t
}

func applyTokenSynonyms(t string) string {
	switch t {
	case "recreation", "rec", "park", "parks":
		return "recreational"
	case "target":
		return "mall"
	case "store", "stores", "shop", "shopping":
		return "retail"
	case "grocery", "groceries":
		return "grocery"
	case "pharmacy", "pharmacies", "drugstore", "drugstores":
		return "pharmacy"
	case "gas", "gasoline", "fuel":
		return "station"
	case "dispensary", "dispensaries":
		return "dispensary"
	case "liquor", "beer", "wine":
		return "liquor"
	case "sale", "deal", "discount", "coupon", "promo", "promotion":
		return "retail"
	case "movie", "movies", "cinema":
		return "movie"
	case "theatre", "theater", "theaters":
		return "theater"
	case "hotel", "hotels":
		return "hotel"
	case "bar", "bars":
		return "bar"
	case "dining", "restaurant", "restaurants", "qsr", "fastfood":
		return "dining"
	case "gym", "gyms", "fitness", "workout":
		return "gym"
	case "salon", "salons":
		return "salon"
	case "spa", "spas":
		return "spa"
	case "bank", "banks", "credit", "loan", "loans", "mortgage", "apr":
		return "bank"
	case "apartment", "apartments", "rent", "rental", "leasing", "lease":
		return "apartment"
	case "office", "offices", "coworking":
		return "office"
	case "dmv", "license", "licensing":
		return "dmv"
	case "military", "base", "bases":
		return "military"
	case "airport", "airports":
		return "airport"
	case "subway", "metro":
		return "subway"
	case "train", "trains", "station", "stations":
		return "station"
	case "bus", "buses":
		return "bus"
	case "taxi", "rideshare", "uber", "lyft":
		return "taxi"
	case "billboard", "billboards":
		return "billboard"
	case "shelter", "shelters":
		return "shelter"
	case "camp", "camps":
		return "school"
	case "sport", "sports":
		return "sports"
	case "college", "university", "universities":
		return "college"
	case "school", "schools":
		return "school"
	default:
		return t
	}
}

func venueTokenSet(v *models.Venue) map[string]struct{} {
	set := make(map[string]struct{})
	for _, t := range tokenizeForSuggestions(v.Name) {
		nt := applyTokenSynonyms(normalizeToken(t))
		if nt != "" {
			set[nt] = struct{}{}
		}
	}
	for _, sc := range v.SubCategory {
		for _, t := range tokenizeForSuggestions(sc) {
			nt := applyTokenSynonyms(normalizeToken(t))
			if nt != "" {
				set[nt] = struct{}{}
			}
		}
	}
	return set
}

func bigrams(tokens []string) []string {
	if len(tokens) < 2 {
		return nil
	}
	var out []string
	for i := 0; i < len(tokens)-1; i++ {
		out = append(out, tokens[i]+" "+tokens[i+1])
	}
	return out
}

func scoreVenues(venues []*models.Venue, extractedText string, topK int) ([]models.VenueSuggestion, []string) {
	if topK <= 0 {
		topK = 5
	}

	tokens := tokenizeForSuggestions(extractedText)
	phrases := bigrams(tokens)
	keywords := append([]string{}, tokens...)

	lowerText := strings.ToLower(extractedText)
	retailIntent := strings.Contains(lowerText, "only at") || strings.Contains(lowerText, "available at") || strings.Contains(lowerText, "exclusive")
	familyIntent := strings.Contains(lowerText, "potty") || strings.Contains(lowerText, "diaper") || strings.Contains(lowerText, "toddler") || strings.Contains(lowerText, "baby") || strings.Contains(lowerText, "kid") || strings.Contains(lowerText, "kids")

	keywordSet := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		nt := applyTokenSynonyms(normalizeToken(t))
		if nt == "" {
			continue
		}
		keywordSet[nt] = struct{}{}
	}

	type scored struct {
		s models.VenueSuggestion
	}
	var scoredList []scored

	for _, v := range venues {
		if v == nil {
			continue
		}
		vTokens := venueTokenSet(v)
		venueNameLower := strings.ToLower(strings.TrimSpace(v.Name))

		var score float64
		var reasons []string
		var matchedSubs []string

		// Intent boosts (helps when brand/store logos aren't captured in OCR)
		if retailIntent {
			if venueNameLower == "retail" {
				score += 2
				reasons = append(reasons, "intent: retail")
			}
			if _, ok := vTokens["retail"]; ok {
				score += 1
				reasons = append(reasons, "intent: retail")
			}
			if _, ok := vTokens["mall"]; ok {
				score += 1
				reasons = append(reasons, "intent: retail")
			}
		}
		if familyIntent {
			switch venueNameLower {
			case "residential":
				score += 1
				reasons = append(reasons, "intent: family")
			case "entertainment":
				score += 1
				reasons = append(reasons, "intent: family")
			case "health and beauty":
				score += 0.5
				reasons = append(reasons, "intent: family")
			}
		}

		// Phrase matches: both tokens exist in venue tokens.
		for _, ph := range phrases {
			ph = strings.TrimSpace(ph)
			if ph == "" {
				continue
			}
			parts := strings.Fields(ph)
			if len(parts) != 2 {
				continue
			}
			a := applyTokenSynonyms(normalizeToken(parts[0]))
			b := applyTokenSynonyms(normalizeToken(parts[1]))
			if a == "" || b == "" {
				continue
			}
			if a == b {
				continue
			}
			if _, ok := vTokens[a]; !ok {
				continue
			}
			if _, ok := vTokens[b]; !ok {
				continue
			}
			score += 2
			reasons = append(reasons, "phrase: "+a+" "+b)
		}

		// Keyword matches
		for t := range keywordSet {
			if _, ok := vTokens[t]; ok {
				score += 1
				reasons = append(reasons, "keyword: "+t)
			}
		}

		// Sub-category match reporting
		for _, sc := range v.SubCategory {
			scl := strings.ToLower(sc)
			if scl == "" {
				continue
			}
			if strings.Contains(lowerText, scl) {
				matchedSubs = append(matchedSubs, sc)
			}
		}

		if score <= 0 {
			continue
		}
		// Normalize score into 0..1-ish for UI
		norm := score / (score + 3)
		scoredList = append(scoredList, scored{s: models.VenueSuggestion{
			VenueID:             v.ID,
			Name:                v.Name,
			MatchedSubCategories: matchedSubs,
			Score:               norm,
			Reasons:             reasons,
		}})
	}

	sort.Slice(scoredList, func(i, j int) bool {
		return scoredList[i].s.Score > scoredList[j].s.Score
	})

	if len(scoredList) > topK {
		scoredList = scoredList[:topK]
	}

	res := make([]models.VenueSuggestion, 0, len(scoredList))
	for _, s := range scoredList {
		res = append(res, s.s)
	}
	return res, keywords
}

func (h *CreativeHandler) ensureRekognitionClient(ctx context.Context) (*rekognition.Client, error) {
	if h.rekognitionClient != nil {
		return h.rekognitionClient, nil
	}
	region := getEnvOrDefault("AWS_REGION", "us-east-1")
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, err
	}
	h.rekognitionClient = rekognition.NewFromConfig(cfg)
	return h.rekognitionClient, nil
}

func (h *CreativeHandler) detectText(ctx context.Context, imageBytes []byte) (string, error) {
	client, err := h.ensureRekognitionClient(ctx)
	if err != nil {
		return "", err
	}
	out, err := client.DetectText(ctx, &rekognition.DetectTextInput{
		Image: &types.Image{Bytes: imageBytes},
	})
	if err != nil {
		return "", err
	}
	if out == nil || len(out.TextDetections) == 0 {
		return "", nil
	}
	var lines []string
	for _, d := range out.TextDetections {
		if d.Type == types.TextTypesLine && d.DetectedText != nil {
			v := strings.TrimSpace(*d.DetectedText)
			if v != "" {
				lines = append(lines, v)
			}
		}
	}
	return strings.Join(lines, "\n"), nil
}

func isOCRNotConfiguredError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// Best-effort classification. In AWS SDK v2, missing credentials and config issues
	// often bubble up as generic errors; we treat these as "not configured".
	if strings.Contains(msg, "no credential") || strings.Contains(msg, "no credentials") {
		return true
	}
	if strings.Contains(msg, "failed to retrieve credentials") || strings.Contains(msg, "missing credentials") {
		return true
	}
	if strings.Contains(msg, "could not load") && strings.Contains(msg, "config") {
		return true
	}
	if strings.Contains(msg, "invalid endpoint") || strings.Contains(msg, "missing region") {
		return true
	}
	return false
}

func (h *CreativeHandler) listAllVenues(ctx context.Context) ([]*models.Venue, error) {
	if h.db == nil {
		return nil, fmt.Errorf("db not configured")
	}
	rows, err := h.db.QueryContext(ctx, `SELECT id, name, sub_category, created_at, updated_at FROM venues ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Venue
	for rows.Next() {
		var v models.Venue
		var sub pq.StringArray
		if err := rows.Scan(&v.ID, &v.Name, &sub, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		v.SubCategory = []string(sub)
		out = append(out, &v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// @Tags Creatives
// @Summary Suggest venues for creatives based on poster content
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param files formData file true "Creative image files"
// @Param top_k formData int false "Maximum venues to return per file" default(5)
// @Success 200 {object} models.CreativeSuggestionsResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/creatives/suggestions [post]
func (h *CreativeHandler) SuggestVenues(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 32 << 20
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Failed to parse form")
		return
	}

	topK := 5
	if raw := strings.TrimSpace(r.FormValue("top_k")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 50 {
			topK = v
		}
	}

	var files []*multipart.FileHeader
	if r.MultipartForm != nil {
		if fhs := r.MultipartForm.File["files"]; len(fhs) > 0 {
			files = append(files, fhs...)
		}
		if len(files) == 0 {
			if fhs := r.MultipartForm.File["file"]; len(fhs) > 0 {
				files = append(files, fhs...)
			}
		}
	}

	if len(files) == 0 {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "No files uploaded")
		return
	}

	results := make([]models.CreativeFileSuggestionResult, 0, len(files))
	needVenues := false
	for _, fh := range files {
		res := models.CreativeFileSuggestionResult{FileName: fh.Filename, Status: "ok"}
		ct := fh.Header.Get("Content-Type")
		if !isSupportedImageContentType(ct) {
			res.Status = "error"
			res.Error = "unsupported file type"
			results = append(results, res)
			continue
		}
		needVenues = true
		break
	}

	if !needVenues {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(models.CreativeSuggestionsResponse{Data: models.CreativeSuggestionsData{Results: results}})
		return
	}

	venues, err := h.listAllVenues(r.Context())
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "suggestions_failed", "Failed to load venues")
		return
	}

	// Rebuild results with full processing
	results = make([]models.CreativeFileSuggestionResult, 0, len(files))
	for _, fh := range files {
		res := models.CreativeFileSuggestionResult{FileName: fh.Filename, Status: "ok"}
		ct := fh.Header.Get("Content-Type")
		if !isSupportedImageContentType(ct) {
			res.Status = "error"
			res.Error = "unsupported file type"
			results = append(results, res)
			continue
		}

		f, err := fh.Open()
		if err != nil {
			res.Status = "error"
			res.Error = "failed to open file"
			results = append(results, res)
			continue
		}
		b, err := io.ReadAll(io.LimitReader(f, 8<<20))
		_ = f.Close()
		if err != nil {
			res.Status = "error"
			res.Error = "failed to read file"
			results = append(results, res)
			continue
		}

		// Best-effort detect text.
		extracted, err := h.detectText(r.Context(), b)
		if err != nil {
			res.Status = "error"
			log.Printf("rekognition detect text failed: file=%s err=%v", fh.Filename, err)
			if isOCRNotConfiguredError(err) {
				res.Error = "ocr_not_configured"
			} else {
				res.Error = "ocr_failed"
			}
			results = append(results, res)
			continue
		}
		res.ExtractedText = extracted
		suggestions, keywords := scoreVenues(venues, extracted, topK)
		res.Suggestions = suggestions
		res.Keywords = keywords
		if strings.TrimSpace(extracted) == "" {
			// fallback: use filename tokens
			res.Keywords = tokenizeForSuggestions(fh.Filename)
			res.Suggestions, _ = scoreVenues(venues, fh.Filename, topK)
		}
		results = append(results, res)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(models.CreativeSuggestionsResponse{Data: models.CreativeSuggestionsData{Results: results}})
}

// generateUUID generates a new UUID
func generateUUID() string {
    return uuid.New().String()
}

func parseFormList(r *http.Request, key string) []string {
    if r.MultipartForm == nil {
        return []string{}
    }

    var out = []string{}
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
// @Param X-Advertiser-Id header string false "Active advertiser scope (required for advertiser-scoped users)"
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

    campaign, err := h.campaignRepo.GetByID(r.Context(), campaignID)
    if err != nil {
        if err == sql.ErrNoRows {
            writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "campaign_id not found")
            return
        }
        log.Printf("Failed to validate campaign %s: %v", campaignID, err)
        writeJSONErrorResponse(w, http.StatusInternalServerError, "server_error", "Failed to validate campaign")
        return
    }

    var impressionCount *int64
    if campaign != nil && campaign.ImpressionsBased {
        raw := strings.TrimSpace(r.FormValue("impression_count"))
        if raw == "" {
            writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "impression_count is required for impressions-based campaigns")
            return
        }
        v, err := strconv.ParseInt(raw, 10, 64)
        if err != nil || v <= 0 {
            writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "impression_count must be a positive integer")
            return
        }
        impressionCount = &v
    }

    playWeight := 100
    if raw := strings.TrimSpace(r.FormValue("play_weight")); raw != "" {
        v, err := strconv.Atoi(raw)
        if err != nil || v < 0 {
            writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "play_weight must be a non-negative integer")
            return
        }
        playWeight = v
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
    var errors []string
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
            ImpressionCount: impressionCount,
            PlayWeight:   playWeight,
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
            errors = append(errors, fmt.Sprintf("Failed to upload %s to S3: %v", fileHeader.Filename, err))
            continue
        }

        // Set the URL
        creative.URL = strings.TrimRight(h.publicBaseURL, "/") + "/" + key

        // Store the object key internally
        creative.FilePath = key

        // Save to database
        if err := h.repo.Create(r.Context(), creative); err != nil {
            errors = append(errors, fmt.Sprintf("Failed to save %s: %v", fileHeader.Filename, err))
            continue
        }

        uploadedCreatives = append(uploadedCreatives, creative)
    }

    // 6. Return the uploaded creatives
    if len(uploadedCreatives) == 0 {
        if len(errors) > 0 {
            writeJSONErrorResponse(w, http.StatusInternalServerError, "upload_failed", strings.Join(errors, "; "))
        } else {
            writeJSONErrorResponse(w, http.StatusInternalServerError, "upload_failed", "Failed to upload any files")
        }
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    if err := json.NewEncoder(w).Encode(uploadedCreatives); err != nil {
        log.Printf("Error encoding response: %v", err)
    }
}

// UpdateCreative handles PUT /creatives/{id}
// @Tags Creatives
// @Summary Update creative
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param X-Advertiser-Id header string false "Active advertiser scope (required for advertiser-scoped users)"
// @Param id path string true "Creative ID"
// @Param body body models.UpdateCreativeRequest true "Update creative request"
// @Success 200 {object} models.Creative
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

        if raw := strings.TrimSpace(r.FormValue("impression_count")); raw != "" {
            if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
                req.ImpressionCount = &v
            } else {
                writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "impression_count must be a valid integer")
                return
            }
        }

        if raw := strings.TrimSpace(r.FormValue("play_weight")); raw != "" {
            if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
                req.PlayWeight = &v
            } else {
                writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "play_weight must be a non-negative integer")
                return
            }
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

    existing, err := h.repo.GetByID(r.Context(), id)
    if err != nil {
        if err == sql.ErrNoRows {
            writeJSONErrorResponse(w, http.StatusNotFound, "creative_not_found", "Creative not found")
            return
        }
        writeJSONErrorResponse(w, http.StatusInternalServerError, "update_creative_failed", "Failed to update creative")
        return
    }

    if h.campaignRepo != nil {
        campaign, err := h.campaignRepo.GetByID(r.Context(), existing.CampaignID)
        if err != nil {
            writeJSONErrorResponse(w, http.StatusInternalServerError, "update_creative_failed", "Failed to update creative")
            return
        }
        finalImpressionCount := existing.ImpressionCount
        if req.ImpressionCount != nil {
            finalImpressionCount = req.ImpressionCount
        }
        if campaign != nil && campaign.ImpressionsBased {
            if finalImpressionCount == nil || *finalImpressionCount <= 0 {
                writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "impression_count is required for impressions-based campaigns")
                return
            }
        }
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
// @Param X-Advertiser-Id header string false "Active advertiser scope (required for advertiser-scoped users)"
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