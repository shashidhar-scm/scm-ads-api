package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"scm/internal/models"
	"scm/internal/repository"
)

// generateRevisionID creates a revision ID from data bytes
// Format: 1-{first 8 chars of SHA256 hash}
// Similar to CouchDB/Sync Gateway revision IDs
func generateRevisionID(data []byte) string {
	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])
	// Use first 8 characters of hash for brevity
	return "1-" + hashStr[:8]
}

func normalizeLegacyDateString(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	// If there is a fractional second component, pad/trim it to microseconds (6 digits)
	// to align with legacy responses like "2022-03-18T05:53:35.848000Z".
	idxDot := strings.IndexByte(s, '.')
	if idxDot == -1 {
		return s
	}

	idxTZ := strings.IndexAny(s[idxDot:], "Z+-")
	if idxTZ == -1 {
		return s
	}
	idxTZ += idxDot

	fraction := s[idxDot+1 : idxTZ]
	if fraction == "" {
		return s
	}

	if len(fraction) < 6 {
		fraction = fraction + strings.Repeat("0", 6-len(fraction))
	} else if len(fraction) > 6 {
		fraction = fraction[:6]
	}

	return s[:idxDot+1] + fraction + s[idxTZ:]
}

func normalizeLegacyDoc(raw []byte) ([]byte, error) {
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}

	var normalize func(any) any
	normalize = func(x any) any {
		switch t := x.(type) {
		case map[string]any:
			if len(t) == 1 {
				if oid, ok := t["$oid"]; ok {
					if s, ok := oid.(string); ok {
						return s
					}
				}
				if dt, ok := t["$date"]; ok {
					if s, ok := dt.(string); ok {
						return normalizeLegacyDateString(s)
					}
				}
			}

			out := make(map[string]any, len(t))
			for k, vv := range t {
				out[k] = normalize(vv)
			}
			if idVal, ok := out["_id"]; ok {
				if s, ok := idVal.(string); ok && s != "" {
					out["id"] = s
					delete(out, "_id")
				}
			}
			return out
		case []any:
			arr := make([]any, 0, len(t))
			for _, item := range t {
				arr = append(arr, normalize(item))
			}
			return arr
		default:
			return x
		}
	}

	norm := normalize(v)
	b, err := json.Marshal(norm)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal normalized document: %w", err)
	}
	return b, nil
}

type LegacyHandler struct {
	db        *sql.DB
	replicaDB *sql.DB
	creative  repository.CreativeRepository
}

func NewLegacyHandler(db *sql.DB, replicaDB *sql.DB) *LegacyHandler {
	var creativeRepo repository.CreativeRepository
	if db != nil {
		creativeRepo = repository.NewCreativeRepository(db)
	}
	return &LegacyHandler{db: db, replicaDB: replicaDB, creative: creativeRepo}
}

// GetTheme handles /scm-api/theme?theme_id=...&rev=...
// @Summary Get theme configuration
// @Description Returns theme data with revision support for conditional fetching. Returns no_changes if client revision matches.
// @Tags Legacy
// @Produce json
// @Param theme_id query string true "Theme identifier (e.g., jc_jct_kiosk_6.0)"
// @Param rev query string false "Client revision for conditional fetch"
// @Success 200 {object} map[string]interface{} "Theme data with status, rev, and theme array"
// @Failure 400 {object} map[string]string "Missing theme_id parameter"
// @Failure 404 {object} map[string]string "Theme not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /scm-api/theme [get]
func (h *LegacyHandler) GetTheme(w http.ResponseWriter, r *http.Request) {
	if h.replicaDB == nil {
		writeJSONErrorResponse(w, http.StatusServiceUnavailable, "replicator_db_not_configured", "replicator database is not configured")
		return
	}

	q := r.URL.Query()
	themeID := strings.TrimSpace(q.Get("theme_id"))
	clientRev := strings.TrimSpace(q.Get("rev"))

	if themeID == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "missing_param", "theme_id is required")
		return
	}

	query := `SELECT data FROM citypost.theme WHERE data->>'theme_id' = $1`

	var dataVal []byte
	if err := h.replicaDB.QueryRowContext(r.Context(), query, themeID).Scan(&dataVal); err != nil {
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "not_found", "theme not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to query theme")
		return
	}

	// Generate revision from theme data
	currentRev := generateRevisionID(dataVal)

	// Check if client has current version
	if clientRev != "" && clientRev == currentRev {
		resp := map[string]any{
			"status": "no_changes",
			"rev":    currentRev,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	var themeData json.RawMessage = dataVal
	{
		var doc any
		dec := json.NewDecoder(bytes.NewReader(themeData))
		dec.UseNumber()
		if err := dec.Decode(&doc); err == nil {
			if m, ok := doc.(map[string]any); ok {
				m["_id"] = nil
				if b, err := json.Marshal(m); err == nil {
					themeData = b
				}
			}
		}
	}

	// The legacy response is an object with a 'theme' key containing an array with the single theme object.
	// The theme object itself is the 'data' JSONB from the DB.
	resp := map[string]any{
		"status": "ok",
		"rev":    currentRev,
		"theme":  []json.RawMessage{themeData},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *LegacyHandler) pickDeviceCreative(ctx context.Context, device string) *models.Creative {
	device = strings.TrimSpace(device)
	if device == "" || h.creative == nil {
		return nil
	}

	now := time.Now().UTC()
	items, err := h.creative.ListByDevice(ctx, device, true, now, 0, 0)
	if err != nil || len(items) == 0 {
		items, _ = h.creative.ListByDevice(ctx, device, false, now, 0, 0)
	}
	if len(items) == 0 {
		return nil
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
	if len(byCampaign) == 0 {
		return nil
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
			if c != nil {
				candidateIDs = append(candidateIDs, c.ID)
			}
		}
		nextID, err := h.creative.PickNextRotationalCreative(ctx, device, campaignID, candidateIDs)
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
	if len(selected) == 0 {
		return nil
	}
	return selected[0]
}

func (h *LegacyHandler) pickDeviceCreatives(ctx context.Context, device string) []*models.Creative {
	device = strings.TrimSpace(device)
	if device == "" || h.creative == nil {
		return nil
	}

	now := time.Now().UTC()
	items, err := h.creative.ListByDevice(ctx, device, true, now, 0, 0)
	if err != nil || len(items) == 0 {
		items, _ = h.creative.ListByDevice(ctx, device, false, now, 0, 0)
	}
	if len(items) == 0 {
		return nil
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
	if len(byCampaign) == 0 {
		return nil
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
			if c != nil {
				candidateIDs = append(candidateIDs, c.ID)
			}
		}
		nextID, err := h.creative.PickNextRotationalCreative(ctx, device, campaignID, candidateIDs)
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

	return selected
}

func isDefaultAdPosterID(id string) bool {
	id = strings.TrimSpace(id)
	if !strings.HasPrefix(id, "ad_poster_default_") {
		return false
	}
	idx := strings.LastIndexByte(id, '_')
	if idx == -1 || idx == len(id)-1 {
		return false
	}
	_, err := strconv.Atoi(id[idx+1:])
	return err == nil
}

func patchLegacyAdPosterWithCreative(raw []byte, placeholderID string, c *models.Creative) ([]byte, bool) {
	if len(raw) == 0 || c == nil {
		return raw, false
	}
	var doc any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return raw, false
	}
	m, ok := doc.(map[string]any)
	if !ok {
		return raw, false
	}

	// Ensure this is the expected placeholder ad poster.
	if v, _ := m["adPosterId"].(string); strings.TrimSpace(v) != strings.TrimSpace(placeholderID) {
		return raw, false
	}

	m["adPosterId"] = c.ID
	m["creative_id"] = c.ID
	m["name"] = c.Name

	bc, _ := m["broadCast"].(map[string]any)
	if bc == nil {
		bc = map[string]any{}
	}
	bc["fileUrl"] = c.URL
	bc["mobileUrl"] = c.URL
	bc["fileName"] = c.Name
	if c.Type == models.CreativeTypeVideo {
		bc["mimetype"] = "video"
	} else {
		bc["mimetype"] = "image"
	}
	m["broadCast"] = bc

	b, err := json.Marshal(m)
	if err != nil {
		return raw, false
	}
	return b, true
}

// GetContent handles /scm-api/getContent?city=...&region=...&rev=...
// @Summary Get posters and ad_posters by city and region
// @Description Returns all active posters and ad_posters with revision support. Returns no_changes if client revision matches.
// @Tags Legacy
// @Produce json
// @Param city query string true "City code (e.g., jc)"
// @Param region query string true "Region code (e.g., jct)"
// @Param rev query string false "Client revision for conditional fetch"
// @Success 200 {object} map[string]interface{} "Content data with status, rev, posters array, and ad_posters array"
// @Failure 400 {object} map[string]string "Missing city or region parameter"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /scm-api/getContent [get]
func (h *LegacyHandler) GetContent(w http.ResponseWriter, r *http.Request) {
	if h.replicaDB == nil {
		writeJSONErrorResponse(w, http.StatusServiceUnavailable, "replicator_db_not_configured", "replicator database is not configured")
		return
	}

	q := r.URL.Query()
	city := strings.TrimSpace(q.Get("city"))
	region := strings.TrimSpace(q.Get("region"))
	clientRev := strings.TrimSpace(q.Get("rev"))

	if city == "" || region == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "missing_params", "city and region are required")
		return
	}

	// Fetch posters
	postersQuery := `SELECT
			data,
			mongo_id,
			COALESCE(title, NULLIF(data->>'title', '')) AS title,
			application_id::text,
			COALESCE(show_in_loop, NULLIF(data->>'showInLoop', '')::boolean) AS show_in_loop,
			COALESCE(beacon_only, NULLIF(data->>'beaconOnly', '')::boolean) AS beacon_only,
			COALESCE(NULLIF(status, ''), NULLIF(data->>'status', ''), 'ACTIVE') AS status,
			COALESCE(NULLIF(region, ''), NULLIF(data->>'region', '')) AS region
		FROM citypost.posters
		WHERE city = $1
			AND lower(btrim(COALESCE(NULLIF(region, ''), NULLIF(data->>'region', ''), NULLIF(data->>'region_name', '')))) = lower(btrim($2))
			AND upper(btrim(COALESCE(NULLIF(status, ''), NULLIF(data->>'status', ''), 'ACTIVE'))) = 'ACTIVE'`
	rows, err := h.replicaDB.QueryContext(r.Context(), postersQuery, city, region)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to query posters")
		return
	}
	defer rows.Close()

	posters := []json.RawMessage{}
	for rows.Next() {
		var dataVal []byte
		var mongoID sql.NullString
		var title sql.NullString
		var applicationID sql.NullString
		var showInLoop sql.NullBool
		var beaconOnly sql.NullBool
		var status sql.NullString
		var regionVal sql.NullString
		if err := rows.Scan(&dataVal, &mongoID, &title, &applicationID, &showInLoop, &beaconOnly, &status, &regionVal); err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to scan poster")
			return
		}

		norm, err := normalizeLegacyDoc(dataVal)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to normalize poster")
			return
		}

		var doc any
		dec := json.NewDecoder(bytes.NewReader(norm))
		dec.UseNumber()
		if err := dec.Decode(&doc); err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to normalize poster")
			return
		}
		m, ok := doc.(map[string]any)
		if ok {
			if _, hasID := m["id"]; !hasID || m["id"] == nil || m["id"] == "" {
				if mongoID.Valid {
					m["id"] = mongoID.String
				}
			}
			if title.Valid {
				m["title"] = title.String
			}
			if applicationID.Valid {
				m["applicationId"] = applicationID.String
			} else {
				m["applicationId"] = nil
			}
			if showInLoop.Valid {
				m["showInLoop"] = showInLoop.Bool
			}
			if beaconOnly.Valid {
				m["beaconOnly"] = beaconOnly.Bool
			}
			if status.Valid {
				m["status"] = status.String
			}
			if regionVal.Valid {
				m["region"] = regionVal.String
			}
			b, err := json.Marshal(m)
			if err != nil {
				writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to normalize poster")
				return
			}
			norm = b
		}

		posters = append(posters, json.RawMessage(norm))
	}
	if err := rows.Err(); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to iterate posters")
		return
	}

	// Fetch ad_posters
	adPostersQuery := `SELECT data
		FROM citypost.ad_posters
		WHERE city = $1
			AND lower(btrim(COALESCE(NULLIF(data->>'region', ''), NULLIF(data->>'region_name', '')))) = lower(btrim($2))
			AND upper(btrim(COALESCE(NULLIF(status, ''), NULLIF(data->>'status', ''), 'ACTIVE'))) = 'ACTIVE'`
	adRows, err := h.replicaDB.QueryContext(r.Context(), adPostersQuery, city, region)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to query ad_posters")
		return
	}
	defer adRows.Close()

	adPosters := []json.RawMessage{}
	for adRows.Next() {
		var dataVal []byte
		if err := adRows.Scan(&dataVal); err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to scan ad_poster")
			return
		}
		norm, err := normalizeLegacyDoc(dataVal)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to normalize ad_poster")
			return
		}
		adPosters = append(adPosters, json.RawMessage(norm))
	}
	if err := adRows.Err(); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to iterate ad_posters")
		return
	}

	// Generate revision from combined content (posters + ad_posters)
	combinedData := map[string]any{
		"posters":    posters,
		"ad_posters": adPosters,
	}
	combinedBytes, _ := json.Marshal(combinedData)
	currentRev := generateRevisionID(combinedBytes)

	// Check if client has current version
	if clientRev != "" && clientRev == currentRev {
		resp := map[string]any{
			"status": "no_changes",
			"rev":    currentRev,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	resp := map[string]any{
		"status":     "ok",
		"rev":        currentRev,
		"posters":    posters,
		"ad_posters": adPosters,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetPosterByID handles /posters/{id} - Get individual poster by ID
// @Summary Get poster by ID
// @Description Returns a single poster document with _id and _rev fields in CouchDB/Sync Gateway style. Searches by mongo_id, poster_id (UUID), or posterId field.
// @Tags Legacy
// @Produce json
// @Param id path string true "Poster ID (mongo_id, poster_id UUID, or posterId)"
// @Success 200 {object} map[string]interface{} "Poster document with _id, _rev, and all poster fields"
// @Failure 404 {object} map[string]string "Poster not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /posters/{id} [get]
func (h *LegacyHandler) GetPosterByID(w http.ResponseWriter, r *http.Request) {
	if h.replicaDB == nil {
		writeJSONErrorResponse(w, http.StatusServiceUnavailable, "replicator_db_not_configured", "replicator database is not configured")
		return
	}

	// Extract ID from URL path
	posterID := strings.TrimPrefix(r.URL.Path, "/posters/")
	posterID = strings.TrimSpace(posterID)

	if posterID == "" || posterID == "_all_docs" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "missing_param", "poster ID is required")
		return
	}

	// Query poster by mongo_id, poster_id (UUID), or posterId field in JSONB
	query := `SELECT data FROM citypost.posters 
		WHERE mongo_id = $1 
		   OR poster_id::text = $1 
		   OR data->>'posterId' = $1 
		LIMIT 1`
	
	var dataBytes []byte
	if err := h.replicaDB.QueryRowContext(r.Context(), query, posterID).Scan(&dataBytes); err != nil {
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "not_found", "poster not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to query poster")
		return
	}

	// Generate revision from data
	rev := generateRevisionID(dataBytes)

	// Parse and add CouchDB-style fields
	var doc map[string]interface{}
	if err := json.Unmarshal(dataBytes, &doc); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to parse poster data")
		return
	}

	doc["_id"] = posterID
	doc["_rev"] = rev

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(doc)
}

// GetAllPosters handles /posters/_all_docs - CouchDB-style listing of all posters
// @Summary Get all posters metadata
// @Description Returns CouchDB-style _all_docs response with all active and scheduled posters. Supports include_docs and region filter.
// @Tags Legacy
// @Produce json
// @Param include_docs query boolean false "Include full document in response"
// @Param limit query integer false "Limit number of results"
// @Param skip query integer false "Skip number of results"
// @Param region query string false "Filter by region (e.g., au, jct)"
// @Success 200 {object} map[string]interface{} "CouchDB-style response with total_rows, offset, and rows array"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /posters/_all_docs [get]
func (h *LegacyHandler) GetAllPosters(w http.ResponseWriter, r *http.Request) {
	if h.replicaDB == nil {
		writeJSONErrorResponse(w, http.StatusServiceUnavailable, "replicator_db_not_configured", "replicator database is not configured")
		return
	}

	q := r.URL.Query()
	includeDocs := q.Get("include_docs") == "true"
	region := strings.TrimSpace(q.Get("region"))
	limit := 1000 // Default limit
	skip := 0

	if limitStr := q.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if skipStr := q.Get("skip"); skipStr != "" {
		if s, err := strconv.Atoi(skipStr); err == nil && s >= 0 {
			skip = s
		}
	}

	// Build query with optional region filter
	var query string
	var args []interface{}
	
	if region != "" {
		query = `SELECT mongo_id, data
			FROM citypost.posters
			WHERE upper(btrim(COALESCE(NULLIF(status, ''), NULLIF(data->>'status', ''), 'ACTIVE'))) IN ('ACTIVE', 'SCHEDULED')
				AND lower(btrim(COALESCE(NULLIF(region, ''), NULLIF(data->>'region', ''), NULLIF(data->>'region_name', '')))) = lower(btrim($1))
			ORDER BY mongo_id
			LIMIT $2 OFFSET $3`
		args = []interface{}{region, limit, skip}
	} else {
		query = `SELECT mongo_id, data
			FROM citypost.posters
			WHERE upper(btrim(COALESCE(NULLIF(status, ''), NULLIF(data->>'status', ''), 'ACTIVE'))) IN ('ACTIVE', 'SCHEDULED')
			ORDER BY mongo_id
			LIMIT $1 OFFSET $2`
		args = []interface{}{limit, skip}
	}

	rows, err := h.replicaDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to query posters")
		return
	}
	defer rows.Close()

	// Get total count with same filter
	var totalRows int
	var countQuery string
	var countArgs []interface{}
	
	if region != "" {
		countQuery = `SELECT COUNT(*) FROM citypost.posters
			WHERE upper(btrim(COALESCE(NULLIF(status, ''), NULLIF(data->>'status', ''), 'ACTIVE'))) IN ('ACTIVE', 'SCHEDULED')
				AND lower(btrim(COALESCE(NULLIF(region, ''), NULLIF(data->>'region', ''), NULLIF(data->>'region_name', '')))) = lower(btrim($1))`
		countArgs = []interface{}{region}
	} else {
		countQuery = `SELECT COUNT(*) FROM citypost.posters
			WHERE upper(btrim(COALESCE(NULLIF(status, ''), NULLIF(data->>'status', ''), 'ACTIVE'))) IN ('ACTIVE', 'SCHEDULED')`
		countArgs = []interface{}{}
	}
	
	if err := h.replicaDB.QueryRowContext(r.Context(), countQuery, countArgs...).Scan(&totalRows); err != nil {
		totalRows = 0
	}

	type Row struct {
		ID    string                 `json:"id"`
		Key   string                 `json:"key"`
		Value map[string]string      `json:"value"`
		Doc   map[string]interface{} `json:"doc,omitempty"`
	}

	result := []Row{}
	for rows.Next() {
		var mongoID string
		var dataBytes []byte
		if err := rows.Scan(&mongoID, &dataBytes); err != nil {
			continue
		}

		// Generate revision from data
		rev := generateRevisionID(dataBytes)

		row := Row{
			ID:  mongoID,
			Key: mongoID,
			Value: map[string]string{
				"rev": rev,
			},
		}

		if includeDocs {
			var doc map[string]interface{}
			if err := json.Unmarshal(dataBytes, &doc); err == nil {
				doc["_id"] = mongoID
				doc["_rev"] = rev
				row.Doc = doc
			}
		}

		result = append(result, row)
	}

	resp := map[string]interface{}{
		"rows":       result,
		"total_rows": totalRows,
		"update_seq": totalRows,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetAdPosterByID handles /adposters/{id} - Get individual ad_poster by ID
// @Summary Get ad_poster by ID
// @Description Returns a single ad_poster document with _id and _rev fields in CouchDB/Sync Gateway style. Searches by external_id or adPosterId field.
// @Tags Legacy
// @Produce json
// @Param id path string true "Ad Poster ID (external_id or adPosterId)"
// @Success 200 {object} map[string]interface{} "Ad Poster document with _id, _rev, and all ad_poster fields"
// @Failure 404 {object} map[string]string "Ad Poster not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /adposters/{id} [get]
func (h *LegacyHandler) GetAdPosterByID(w http.ResponseWriter, r *http.Request) {
	if h.replicaDB == nil {
		writeJSONErrorResponse(w, http.StatusServiceUnavailable, "replicator_db_not_configured", "replicator database is not configured")
		return
	}

	// Extract ID from URL path
	adPosterID := strings.TrimPrefix(r.URL.Path, "/adposters/")
	adPosterID = strings.TrimSpace(adPosterID)

	if adPosterID == "" || adPosterID == "_all_docs" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "missing_param", "ad_poster ID is required")
		return
	}

	// Query ad_poster by external_id or adPosterId field in JSONB
	query := `SELECT data FROM citypost.ad_posters 
		WHERE external_id = $1 
		   OR data->>'adPosterId' = $1 
		LIMIT 1`
	
	var dataBytes []byte
	if err := h.replicaDB.QueryRowContext(r.Context(), query, adPosterID).Scan(&dataBytes); err != nil {
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "not_found", "ad_poster not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to query ad_poster")
		return
	}

	// Generate revision from data
	rev := generateRevisionID(dataBytes)

	// Parse and add CouchDB-style fields
	var doc map[string]interface{}
	if err := json.Unmarshal(dataBytes, &doc); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to parse ad_poster data")
		return
	}

	doc["_id"] = adPosterID
	doc["_rev"] = rev

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(doc)
}

// GetThemeByID handles /themes/{id} - Get individual theme by ID
// @Summary Get theme by ID
// @Description Returns a single theme document with _id and _rev fields in CouchDB/Sync Gateway style
// @Tags Legacy
// @Produce json
// @Param id path string true "Theme ID (e.g., jc_jct_kiosk_6.0)"
// @Success 200 {object} map[string]interface{} "Theme document with _id, _rev, and all theme fields"
// @Failure 404 {object} map[string]string "Theme not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /themes/{id} [get]
func (h *LegacyHandler) GetThemeByID(w http.ResponseWriter, r *http.Request) {
	if h.replicaDB == nil {
		writeJSONErrorResponse(w, http.StatusServiceUnavailable, "replicator_db_not_configured", "replicator database is not configured")
		return
	}

	// Extract ID from URL path
	themeID := strings.TrimPrefix(r.URL.Path, "/themes/")
	themeID = strings.TrimSpace(themeID)

	if themeID == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "missing_param", "theme ID is required")
		return
	}

	// Query theme by theme_id from JSONB data
	query := `SELECT data FROM citypost.theme WHERE data->>'theme_id' = $1 LIMIT 1`
	
	var dataBytes []byte
	if err := h.replicaDB.QueryRowContext(r.Context(), query, themeID).Scan(&dataBytes); err != nil {
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "not_found", "theme not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to query theme")
		return
	}

	// Generate revision from data
	rev := generateRevisionID(dataBytes)

	// Parse and add CouchDB-style fields
	var doc map[string]interface{}
	if err := json.Unmarshal(dataBytes, &doc); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to parse theme data")
		return
	}

	doc["_id"] = themeID
	doc["_rev"] = rev

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(doc)
}

// GetLoopPosterByID handles /loop_posters/{id} - Get individual loop_poster by ID
// @Summary Get loop_poster by ID
// @Description Returns a single loop_poster document with _id and _rev fields in CouchDB/Sync Gateway style
// @Tags Legacy
// @Produce json
// @Param id path string true "Loop Poster ID (loopPosterId)"
// @Success 200 {object} map[string]interface{} "Loop Poster document with _id, _rev, and all loop_poster fields"
// @Failure 404 {object} map[string]string "Loop Poster not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /loop_posters/{id} [get]
func (h *LegacyHandler) GetLoopPosterByID(w http.ResponseWriter, r *http.Request) {
	if h.replicaDB == nil {
		writeJSONErrorResponse(w, http.StatusServiceUnavailable, "replicator_db_not_configured", "replicator database is not configured")
		return
	}

	// Extract ID from URL path
	loopPosterID := strings.TrimPrefix(r.URL.Path, "/loop_posters/")
	loopPosterID = strings.TrimSpace(loopPosterID)

	if loopPosterID == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "missing_param", "loop_poster ID is required")
		return
	}

	// Query loop_poster by loopPosterId or device_code
	query := `SELECT data FROM citypost.loop_posters 
		WHERE (status IS NULL OR btrim(status) = '' OR upper(btrim(status)) = 'ACTIVE')
			AND (lower(btrim(data->>'loopPosterId')) = lower($1) 
				OR lower(btrim(data->>'device_code')) = lower($1))
		LIMIT 1`
	
	var dataBytes []byte
	if err := h.replicaDB.QueryRowContext(r.Context(), query, loopPosterID).Scan(&dataBytes); err != nil {
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "not_found", "loop_poster not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to query loop_poster")
		return
	}

	// Generate revision from data
	rev := generateRevisionID(dataBytes)

	// Parse and add CouchDB-style fields
	var doc map[string]interface{}
	if err := json.Unmarshal(dataBytes, &doc); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to parse loop_poster data")
		return
	}

	doc["_id"] = loopPosterID
	doc["_rev"] = rev

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(doc)
}

// GetAllAdPosters handles /adposters/_all_docs - CouchDB-style listing of all ad_posters
// @Summary Get all ad_posters metadata
// @Description Returns CouchDB-style _all_docs response with all active and scheduled ad_posters. Supports include_docs and region filter.
// @Tags Legacy
// @Produce json
// @Param include_docs query boolean false "Include full document in response"
// @Param limit query integer false "Limit number of results"
// @Param skip query integer false "Skip number of results"
// @Param region query string false "Filter by region (e.g., au, jct)"
// @Success 200 {object} map[string]interface{} "CouchDB-style response with total_rows, offset, and rows array"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /adposters/_all_docs [get]
func (h *LegacyHandler) GetAllAdPosters(w http.ResponseWriter, r *http.Request) {
	if h.replicaDB == nil {
		writeJSONErrorResponse(w, http.StatusServiceUnavailable, "replicator_db_not_configured", "replicator database is not configured")
		return
	}

	q := r.URL.Query()
	includeDocs := q.Get("include_docs") == "true"
	region := strings.TrimSpace(q.Get("region"))
	limit := 1000 // Default limit
	skip := 0

	if limitStr := q.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if skipStr := q.Get("skip"); skipStr != "" {
		if s, err := strconv.Atoi(skipStr); err == nil && s >= 0 {
			skip = s
		}
	}

	// Build query with optional region filter
	var query string
	var args []interface{}
	
	if region != "" {
		query = `SELECT external_id, data
			FROM citypost.ad_posters
			WHERE upper(btrim(COALESCE(NULLIF(status, ''), NULLIF(data->>'status', ''), 'ACTIVE'))) IN ('ACTIVE', 'SCHEDULED')
				AND lower(btrim(COALESCE(NULLIF(data->>'region', ''), NULLIF(data->>'region_name', ''), NULLIF(data->>'city', '')))) = lower(btrim($1))
			ORDER BY external_id
			LIMIT $2 OFFSET $3`
		args = []interface{}{region, limit, skip}
	} else {
		query = `SELECT external_id, data
			FROM citypost.ad_posters
			WHERE upper(btrim(COALESCE(NULLIF(status, ''), NULLIF(data->>'status', ''), 'ACTIVE'))) IN ('ACTIVE', 'SCHEDULED')
			ORDER BY external_id
			LIMIT $1 OFFSET $2`
		args = []interface{}{limit, skip}
	}

	rows, err := h.replicaDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to query ad_posters")
		return
	}
	defer rows.Close()

	// Get total count with same filter
	var totalRows int
	var countQuery string
	var countArgs []interface{}
	
	if region != "" {
		countQuery = `SELECT COUNT(*) FROM citypost.ad_posters
			WHERE upper(btrim(COALESCE(NULLIF(status, ''), NULLIF(data->>'status', ''), 'ACTIVE'))) IN ('ACTIVE', 'SCHEDULED')
				AND lower(btrim(COALESCE(NULLIF(data->>'region', ''), NULLIF(data->>'region_name', ''), NULLIF(data->>'city', '')))) = lower(btrim($1))`
		countArgs = []interface{}{region}
	} else {
		countQuery = `SELECT COUNT(*) FROM citypost.ad_posters
			WHERE upper(btrim(COALESCE(NULLIF(status, ''), NULLIF(data->>'status', ''), 'ACTIVE'))) IN ('ACTIVE', 'SCHEDULED')`
		countArgs = []interface{}{}
	}
	
	if err := h.replicaDB.QueryRowContext(r.Context(), countQuery, countArgs...).Scan(&totalRows); err != nil {
		totalRows = 0
	}

	type Row struct {
		ID    string                 `json:"id"`
		Key   string                 `json:"key"`
		Value map[string]string      `json:"value"`
		Doc   map[string]interface{} `json:"doc,omitempty"`
	}

	result := []Row{}
	for rows.Next() {
		var externalID string
		var dataBytes []byte
		if err := rows.Scan(&externalID, &dataBytes); err != nil {
			continue
		}

		// Generate revision from data
		rev := generateRevisionID(dataBytes)

		row := Row{
			ID:  externalID,
			Key: externalID,
			Value: map[string]string{
				"rev": rev,
			},
		}

		if includeDocs {
			var doc map[string]interface{}
			if err := json.Unmarshal(dataBytes, &doc); err == nil {
				doc["_id"] = externalID
				doc["_rev"] = rev
				row.Doc = doc
			}
		}

		result = append(result, row)
	}

	resp := map[string]interface{}{
		"rows":       result,
		"total_rows": totalRows,
		"update_seq": totalRows,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetLoopByDevice handles /theme/{device} - RESTful endpoint for loop data
// @Summary Get loop configuration by device
// @Description Returns loop document with _id, _rev, and cards array in CouchDB/Sync Gateway style. Supports conditional fetching via If-None-Match header with ETag.
// @Tags Legacy
// @Produce json
// @Param device path string true "Device hostname or identifier (e.g., U696843)"
// @Param If-None-Match header string false "ETag from previous response for conditional fetch"
// @Success 200 {object} map[string]interface{} "Loop document with _id, _rev, cards, city, device_code, device_type, loopPosterId, region"
// @Success 304 "Not Modified - content unchanged since last fetch"
// @Failure 404 {object} map[string]string "Loop not found for device"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /theme/{device} [get]
func (h *LegacyHandler) GetLoopByDevice(w http.ResponseWriter, r *http.Request) {
	if h.replicaDB == nil {
		writeJSONErrorResponse(w, http.StatusServiceUnavailable, "replicator_db_not_configured", "replicator database is not configured")
		return
	}

	// Extract device from URL path (e.g., /theme/U696843)
	device := strings.TrimPrefix(r.URL.Path, "/theme/")
	device = strings.TrimSpace(device)

	if device == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "missing_param", "device is required")
		return
	}

	// Query loop_posters table to find matching device
	loopQuery := `SELECT data
		FROM citypost.loop_posters
		WHERE (status IS NULL OR btrim(status) = '' OR upper(btrim(status)) = 'ACTIVE')
			AND (
				lower(btrim(data->>'loopPosterId')) = lower($1)
				OR lower(btrim(data->>'device_code')) = lower($1)
				OR lower(btrim(data->>'device_type')) = lower($1)
				OR lower(btrim(data->>'kiosksId')) = lower($1)
				OR (
					jsonb_typeof(data->'kiosksId') = 'array'
					AND EXISTS (
						SELECT 1
						FROM jsonb_array_elements_text(data->'kiosksId') AS k(v)
						WHERE lower(btrim(k.v)) = lower($1)
					)
				)
			)
		LIMIT 1`

	var loopDataBytes []byte
	if err := h.replicaDB.QueryRowContext(r.Context(), loopQuery, device).Scan(&loopDataBytes); err != nil {
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "not_found", "loop not found for device")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to query loop_posters")
		return
	}

	// Generate revision from loop data
	currentRev := generateRevisionID(loopDataBytes)

	// Check If-None-Match header for conditional fetch
	ifNoneMatch := r.Header.Get("If-None-Match")
	if ifNoneMatch != "" && ifNoneMatch == currentRev {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Parse loop data and add _id and _rev fields
	var loopDoc map[string]any
	if err := json.Unmarshal(loopDataBytes, &loopDoc); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to parse loop data")
		return
	}

	// Add CouchDB-style fields
	loopDoc["_id"] = device
	loopDoc["_rev"] = currentRev

	// Set ETag header for caching
	w.Header().Set("ETag", currentRev)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(loopDoc)
}

// GetLoopPostersWeb handles /scm-api/getLoopPostersWeb?city=...&device=...&rev=...
// @Summary Get loop posters for a device
// @Description Returns resolved loop poster objects with revision support. Returns no_changes if client revision matches.
// @Tags Legacy
// @Produce json
// @Param city query string true "City code (e.g., jc)"
// @Param device query string true "Device identifier"
// @Param rev query string false "Client revision for conditional fetch"
// @Success 200 {object} map[string]interface{} "Loop data with status, rev, and loop_poster array"
// @Failure 400 {object} map[string]string "Missing city or device parameter"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /scm-api/getLoopPostersWeb [get]
func (h *LegacyHandler) GetLoopPostersWeb(w http.ResponseWriter, r *http.Request) {
	if h.replicaDB == nil {
		writeJSONErrorResponse(w, http.StatusServiceUnavailable, "replicator_db_not_configured", "replicator database is not configured")
		return
	}

	q := r.URL.Query()
	city := strings.TrimSpace(q.Get("city"))
	device := strings.TrimSpace(q.Get("device"))
	clientRev := strings.TrimSpace(q.Get("rev"))

	if city == "" || device == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "missing_params", "city and device are required")
		return
	}

	// Legacy behavior: resolve the loop_poster cards into full poster objects.
	// The loop_posters table stores a list of card identifiers in data.cards.
	// The API returns the resolved poster objects in that order.
	loopQuery := `SELECT data
		FROM citypost.loop_posters
		WHERE city = $1
			AND (status IS NULL OR btrim(status) = '' OR upper(btrim(status)) = 'ACTIVE')
			AND (
				lower(btrim(data->>'loopPosterId')) = lower($2)
				OR lower(btrim(data->>'device_code')) = lower($2)
				OR lower(btrim(data->>'device_type')) = lower($2)
				OR lower(btrim(data->>'kiosksId')) = lower($2)
				OR (
					jsonb_typeof(data->'kiosksId') = 'array'
					AND EXISTS (
						SELECT 1
						FROM jsonb_array_elements_text(data->'kiosksId') AS k(v)
						WHERE lower(btrim(k.v)) = lower($2)
					)
				)
			)
		LIMIT 1`

	var loopDataBytes []byte
	if err := h.replicaDB.QueryRowContext(r.Context(), loopQuery, city, device).Scan(&loopDataBytes); err != nil {
		if err == sql.ErrNoRows {
			resp := map[string][]json.RawMessage{"loop_poster": {}}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to query loop_posters")
		return
	}

	// Generate revision ID from loop data hash (similar to CouchDB/Sync Gateway)
	// Format: {generation}-{hash} where hash is first 8 chars of SHA256
	currentRev := generateRevisionID(loopDataBytes)

	// If client provided rev and it matches current rev, return no_changes response
	if clientRev != "" && clientRev == currentRev {
		resp := map[string]any{
			"status": "no_changes",
			"rev":    currentRev,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	var loopDoc struct {
		Cards []string `json:"cards"`
	}
	_ = json.Unmarshal(loopDataBytes, &loopDoc)

	deviceCreatives := h.pickDeviceCreatives(r.Context(), device)
	creativeIdx := 0

	resolved := make([]json.RawMessage, 0, len(loopDoc.Cards))
	posterQuery := `SELECT
			data,
			COALESCE(title, NULLIF(data->>'title', '')) AS title,
			application_id::text,
			COALESCE(show_in_loop, NULLIF(data->>'showInLoop', '')::boolean) AS show_in_loop,
			COALESCE(beacon_only, NULLIF(data->>'beaconOnly', '')::boolean) AS beacon_only,
			COALESCE(NULLIF(status, ''), NULLIF(data->>'status', ''), 'ACTIVE') AS status
		FROM citypost.posters
		WHERE city = $1
			AND (status IS NULL OR btrim(status) = '' OR upper(btrim(status)) = 'ACTIVE')
			AND lower(btrim(poster_id)) = lower($2)
		LIMIT 1`
	adPosterQuery := `SELECT data
		FROM citypost.ad_posters
		WHERE city = $1
			AND upper(btrim(COALESCE(NULLIF(status, ''), NULLIF(data->>'status', ''), 'ACTIVE'))) = 'ACTIVE'
			AND (
				lower(btrim(NULLIF(data->>'adPosterId', ''))) = lower($2)
				OR lower(btrim(NULLIF(data->>'id', ''))) = lower($2)
				OR lower(btrim(NULLIF(data->>'external_id', ''))) = lower($2)
				OR lower(btrim(external_id)) = lower($2)
			)
		LIMIT 1`

	for _, cardID := range loopDoc.Cards {
		id := strings.TrimSpace(cardID)
		if id == "" {
			continue
		}

		var dataVal []byte
		var title sql.NullString
		var applicationID sql.NullString
		var showInLoop sql.NullBool
		var beaconOnly sql.NullBool
		var status sql.NullString

		err := h.replicaDB.QueryRowContext(r.Context(), posterQuery, city, id).Scan(&dataVal, &title, &applicationID, &showInLoop, &beaconOnly, &status)
		if err != nil {
			if err != sql.ErrNoRows {
				writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to query poster")
				return
			}

			err = h.replicaDB.QueryRowContext(r.Context(), adPosterQuery, city, id).Scan(&dataVal)
			if err != nil {
				if err == sql.ErrNoRows {
					continue
				}
				writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to query ad_poster")
				return
			}
		}

		norm, err := normalizeLegacyDoc(dataVal)
		if err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to normalize poster")
			return
		}

		if isDefaultAdPosterID(id) && creativeIdx < len(deviceCreatives) {
			c := deviceCreatives[creativeIdx]
			if c != nil {
				if patched, ok := patchLegacyAdPosterWithCreative(norm, id, c); ok {
					norm = patched
					creativeIdx++
				}
			}
		}

		if title.Valid || applicationID.Valid || showInLoop.Valid || beaconOnly.Valid || status.Valid {
			var doc any
			dec := json.NewDecoder(bytes.NewReader(norm))
			dec.UseNumber()
			if err := dec.Decode(&doc); err != nil {
				writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to normalize poster")
				return
			}
			m, ok := doc.(map[string]any)
			if ok {
				if title.Valid {
					m["title"] = title.String
				}
				if applicationID.Valid {
					m["applicationId"] = applicationID.String
				} else {
					m["applicationId"] = nil
				}
				if showInLoop.Valid {
					m["showInLoop"] = showInLoop.Bool
				}
				if beaconOnly.Valid {
					m["beaconOnly"] = beaconOnly.Bool
				}
				if status.Valid {
					m["status"] = status.String
				}
				b, err := json.Marshal(m)
				if err != nil {
					writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to normalize poster")
					return
				}
				norm = b
			}
		}

		resolved = append(resolved, json.RawMessage(norm))
	}

	resp := map[string]any{
		"status":      "ok",
		"rev":         currentRev,
		"loop_poster": resolved,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
