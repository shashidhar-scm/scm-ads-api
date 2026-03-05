package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ReplicatorAdPoster struct {
	City       string          `json:"city"`
	ExternalID string          `json:"external_id"`
	Status     *string         `json:"status,omitempty"`
	CreatedAt  *time.Time      `json:"created_at,omitempty"`
	UpdatedAt  *time.Time      `json:"updated_at,omitempty"`
	Data       json.RawMessage `json:"data"`
}

type ReplicatorHandler struct {
	replicaDB *sql.DB
}

func NewReplicatorHandler(replicaDB *sql.DB) *ReplicatorHandler {
	return &ReplicatorHandler{replicaDB: replicaDB}
}

func (h *ReplicatorHandler) ListAdPosters(w http.ResponseWriter, r *http.Request) {
	if h.replicaDB == nil {
		writeJSONErrorResponse(w, http.StatusServiceUnavailable, "replicator_db_not_configured", "replicator database is not configured")
		return
	}

	p, err := parsePaginationParams(r, 50, 200)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_pagination", "invalid pagination params")
		return
	}

	q := r.URL.Query()
	city := strings.TrimSpace(q.Get("city"))
	status := strings.TrimSpace(q.Get("status"))

	where := "WHERE 1=1"
	args := []any{}
	argPos := 1
	if city != "" {
		where += " AND city = $" + strconv.Itoa(argPos)
		args = append(args, city)
		argPos++
	}
	if status != "" {
		where += " AND status = $" + strconv.Itoa(argPos)
		args = append(args, status)
		argPos++
	}

	countQuery := "SELECT COUNT(*) FROM citypost.ad_posters " + where
	var total int
	if err := h.replicaDB.QueryRowContext(r.Context(), countQuery, args...).Scan(&total); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to count ad_posters")
		return
	}

	listQuery := "SELECT city, external_id, status, created_at, updated_at, data FROM citypost.ad_posters " + where + " ORDER BY updated_at DESC NULLS LAST, id DESC LIMIT $" + strconv.Itoa(argPos) + " OFFSET $" + strconv.Itoa(argPos+1)
	args = append(args, p.limit, p.offset)

	rows, err := h.replicaDB.QueryContext(r.Context(), listQuery, args...)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to list ad_posters")
		return
	}
	defer rows.Close()

	out := []ReplicatorAdPoster{}
	for rows.Next() {
		var (
			cityVal       string
			externalIDVal string
			statusVal     sql.NullString
			createdAtVal  sql.NullTime
			updatedAtVal  sql.NullTime
			dataVal       []byte
		)
		if err := rows.Scan(&cityVal, &externalIDVal, &statusVal, &createdAtVal, &updatedAtVal, &dataVal); err != nil {
			writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to read ad_posters")
			return
		}
		item := ReplicatorAdPoster{City: cityVal, ExternalID: externalIDVal, Data: json.RawMessage(dataVal)}
		if statusVal.Valid {
			v := statusVal.String
			item.Status = &v
		}
		if createdAtVal.Valid {
			v := createdAtVal.Time
			item.CreatedAt = &v
		}
		if updatedAtVal.Valid {
			v := updatedAtVal.Time
			item.UpdatedAt = &v
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to list ad_posters")
		return
	}

	writePaginatedResponse(w, http.StatusOK, out, p.page, p.pageSize, total)
}
