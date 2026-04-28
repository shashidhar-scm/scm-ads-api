package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

type RegionSyncHandler struct {
	db        *sql.DB // Primary writable database
	replicaDB *sql.DB // Read-only replica with citypost schema
}

func NewRegionSyncHandler(db *sql.DB, replicaDB *sql.DB) *RegionSyncHandler {
	return &RegionSyncHandler{
		db:        db,
		replicaDB: replicaDB,
	}
}

type RegionSyncRequest struct {
	Source RegionSpec `json:"source"`
	Target RegionSpec `json:"target"`
}

type RegionSpec struct {
	Region string `json:"region"`
	City   string `json:"city"`
}

type RegionSyncResponse struct {
	Success bool              `json:"success"`
	Stats   RegionSyncStats   `json:"stats"`
	Errors  []string          `json:"errors,omitempty"`
}

type RegionSyncStats struct {
	PostersProcessed   int `json:"posters_processed"`
	PostersCopied      int `json:"posters_copied"`
	PostersUpdated     int `json:"posters_updated"`
	AdPostersProcessed int `json:"adposters_processed"`
	AdPostersCopied    int `json:"adposters_copied"`
	AdPostersUpdated   int `json:"adposters_updated"`
}

// SyncPosters handles POST /sync/posters - Copy posters from source region to target region
// @Summary Sync posters between regions
// @Description Copies all ACTIVE and SCHEDULED posters from source region to target region, updating location-specific fields
// @Tags Sync
// @Accept json
// @Produce json
// @Param request body RegionSyncRequest true "Source and target region specifications"
// @Success 200 {object} RegionSyncResponse "Sync completed with statistics"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /sync/posters [post]
func (h *RegionSyncHandler) SyncPosters(w http.ResponseWriter, r *http.Request) {
	var req RegionSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if req.Source.Region == "" || req.Target.Region == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "missing_params", "source and target regions are required")
		return
	}

	resp := RegionSyncResponse{
		Success: true,
		Stats:   RegionSyncStats{},
		Errors:  []string{},
	}

	// Query all ACTIVE and SCHEDULED posters from source region
	query := `
		SELECT mongo_id, data
		FROM citypost.posters
		WHERE upper(btrim(COALESCE(NULLIF(status, ''), NULLIF(data->>'status', ''), 'ACTIVE'))) IN ('ACTIVE', 'SCHEDULED')
			AND lower(btrim(COALESCE(NULLIF(region, ''), NULLIF(data->>'region', ''), NULLIF(data->>'region_name', '')))) = lower($1)
	`

	// Read from replicaDB (has MongoDB-synced citypost schema)
	if h.replicaDB == nil {
		writeJSONErrorResponse(w, http.StatusServiceUnavailable, "replica_not_configured", "replicator database is not configured")
		return
	}

	rows, err := h.replicaDB.QueryContext(r.Context(), query, req.Source.Region)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "query_failed", "failed to query source posters: "+err.Error())
		return
	}
	defer rows.Close()

	// Write to primary DB (writable PostgreSQL)
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "transaction_failed", "failed to start transaction")
		return
	}
	defer tx.Rollback()

	for rows.Next() {
		var mongoID string
		var dataBytes []byte
		if err := rows.Scan(&mongoID, &dataBytes); err != nil {
			resp.Errors = append(resp.Errors, "scan error for "+mongoID+": "+err.Error())
			continue
		}
		resp.Stats.PostersProcessed++

		// Parse the data
		var data map[string]interface{}
		if err := json.Unmarshal(dataBytes, &data); err != nil {
			resp.Errors = append(resp.Errors, "parse error for "+mongoID+": "+err.Error())
			continue
		}

		// Update location-specific fields
		data["region"] = req.Target.Region
		if req.Target.City != "" {
			data["city"] = req.Target.City
		}
		
		// Update other location fields if they exist
		if _, ok := data["region_name"]; ok {
			data["region_name"] = req.Target.Region
		}
		if _, ok := data["city_name"]; ok && req.Target.City != "" {
			data["city_name"] = req.Target.City
		}

		// Marshal updated data
		updatedData, err := json.Marshal(data)
		if err != nil {
			resp.Errors = append(resp.Errors, "marshal error for "+mongoID+": "+err.Error())
			continue
		}

		// Upsert into target region (INSERT ... ON CONFLICT UPDATE)
		upsertQuery := `
			INSERT INTO citypost.posters (mongo_id, data, region, city, status, title)
			VALUES ($1, $2, $3, $4, 
				COALESCE(NULLIF($2->>'status', ''), 'ACTIVE'),
				COALESCE(NULLIF($2->>'title', ''), ''))
			ON CONFLICT (mongo_id) DO UPDATE SET
				data = EXCLUDED.data,
				region = EXCLUDED.region,
				city = EXCLUDED.city,
				status = EXCLUDED.status,
				title = EXCLUDED.title,
				updated_at = NOW()
		`

		result, err := tx.ExecContext(r.Context(), upsertQuery, mongoID, updatedData, req.Target.Region, req.Target.City)
		if err != nil {
			resp.Errors = append(resp.Errors, "upsert error for "+mongoID+": "+err.Error())
			continue
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			resp.Stats.PostersCopied++
		}
	}

	if err := tx.Commit(); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "commit_failed", "failed to commit transaction")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// SyncAdPosters handles POST /sync/adposters - Copy ad_posters from source region to target region
// @Summary Sync ad_posters between regions
// @Description Copies all ACTIVE and SCHEDULED ad_posters from source region to target region, updating location-specific fields
// @Tags Sync
// @Accept json
// @Produce json
// @Param request body RegionSyncRequest true "Source and target region specifications"
// @Success 200 {object} RegionSyncResponse "Sync completed with statistics"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /sync/adposters [post]
func (h *RegionSyncHandler) SyncAdPosters(w http.ResponseWriter, r *http.Request) {
	var req RegionSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if req.Source.Region == "" || req.Target.Region == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "missing_params", "source and target regions are required")
		return
	}

	resp := RegionSyncResponse{
		Success: true,
		Stats:   RegionSyncStats{},
		Errors:  []string{},
	}

	// Query all ACTIVE and SCHEDULED ad_posters from source region
	query := `
		SELECT external_id, data
		FROM citypost.ad_posters
		WHERE upper(btrim(COALESCE(NULLIF(status, ''), NULLIF(data->>'status', ''), 'ACTIVE'))) IN ('ACTIVE', 'SCHEDULED')
			AND lower(btrim(COALESCE(NULLIF(data->>'region', ''), NULLIF(data->>'region_name', ''), NULLIF(data->>'city', '')))) = lower($1)
	`

	// Read from replicaDB (has MongoDB-synced citypost schema)
	if h.replicaDB == nil {
		writeJSONErrorResponse(w, http.StatusServiceUnavailable, "replica_not_configured", "replicator database is not configured")
		return
	}

	rows, err := h.replicaDB.QueryContext(r.Context(), query, req.Source.Region)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "query_failed", "failed to query source ad_posters: "+err.Error())
		return
	}
	defer rows.Close()

	// Write to primary DB (writable PostgreSQL)
	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "transaction_failed", "failed to start transaction")
		return
	}
	defer tx.Rollback()

	for rows.Next() {
		var externalID string
		var dataBytes []byte
		if err := rows.Scan(&externalID, &dataBytes); err != nil {
			resp.Errors = append(resp.Errors, "scan error for "+externalID+": "+err.Error())
			continue
		}
		resp.Stats.AdPostersProcessed++

		// Parse the data
		var data map[string]interface{}
		if err := json.Unmarshal(dataBytes, &data); err != nil {
			resp.Errors = append(resp.Errors, "parse error for "+externalID+": "+err.Error())
			continue
		}

		// Update location-specific fields
		data["region"] = req.Target.Region
		if req.Target.City != "" {
			data["city"] = req.Target.City
		}
		
		// Update other location fields if they exist
		if _, ok := data["region_name"]; ok {
			data["region_name"] = req.Target.Region
		}
		if _, ok := data["city_name"]; ok && req.Target.City != "" {
			data["city_name"] = req.Target.City
		}

		// Marshal updated data
		updatedData, err := json.Marshal(data)
		if err != nil {
			resp.Errors = append(resp.Errors, "marshal error for "+externalID+": "+err.Error())
			continue
		}

		// Upsert into target region
		upsertQuery := `
			INSERT INTO citypost.ad_posters (external_id, data, status)
			VALUES ($1, $2, COALESCE(NULLIF($2->>'status', ''), 'ACTIVE'))
			ON CONFLICT (external_id) DO UPDATE SET
				data = EXCLUDED.data,
				status = EXCLUDED.status,
				updated_at = NOW()
		`

		result, err := tx.ExecContext(r.Context(), upsertQuery, externalID, updatedData)
		if err != nil {
			resp.Errors = append(resp.Errors, "upsert error for "+externalID+": "+err.Error())
			continue
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			resp.Stats.AdPostersCopied++
		}
	}

	if err := tx.Commit(); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "commit_failed", "failed to commit transaction")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
