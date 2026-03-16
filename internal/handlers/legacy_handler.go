package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"scm/internal/models"
	"scm/internal/repository"
)

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

// GetTheme handles /scm-api/theme?theme_id=...
func (h *LegacyHandler) GetTheme(w http.ResponseWriter, r *http.Request) {
	if h.replicaDB == nil {
		writeJSONErrorResponse(w, http.StatusServiceUnavailable, "replicator_db_not_configured", "replicator database is not configured")
		return
	}

	themeID := strings.TrimSpace(r.URL.Query().Get("theme_id"))
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
	resp := map[string][]json.RawMessage{
		"theme": {themeData},
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

// GetContent handles /scm-api/getContent?city=...&region=...
func (h *LegacyHandler) GetContent(w http.ResponseWriter, r *http.Request) {
	if h.replicaDB == nil {
		writeJSONErrorResponse(w, http.StatusServiceUnavailable, "replicator_db_not_configured", "replicator database is not configured")
		return
	}

	q := r.URL.Query()
	city := strings.TrimSpace(q.Get("city"))
	region := strings.TrimSpace(q.Get("region"))

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

	resp := map[string]any{
		"posters":    posters,
		"ad_posters": adPosters,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetLoopPostersWeb handles /scm-api/getLoopPostersWeb?city=...&device=...
func (h *LegacyHandler) GetLoopPostersWeb(w http.ResponseWriter, r *http.Request) {
	if h.replicaDB == nil {
		writeJSONErrorResponse(w, http.StatusServiceUnavailable, "replicator_db_not_configured", "replicator database is not configured")
		return
	}

	q := r.URL.Query()
	city := strings.TrimSpace(q.Get("city"))
	device := strings.TrimSpace(q.Get("device"))

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
		ORDER BY updated_at DESC NULLS LAST, created_at DESC NULLS LAST
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

	var loopDoc struct {
		Cards []string `json:"cards"`
	}
	_ = json.Unmarshal(loopDataBytes, &loopDoc)

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

		if strings.HasPrefix(id, "ad_poster_default_") && strings.HasSuffix(id, "_0") {
			c := h.pickDeviceCreative(r.Context(), device)
			if c != nil {
				if patched, ok := patchLegacyAdPosterWithCreative(norm, id, c); ok {
					norm = patched
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

	resp := map[string][]json.RawMessage{"loop_poster": resolved}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
