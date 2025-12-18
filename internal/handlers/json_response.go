package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
)

type Pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type paginationParams struct {
	page     int
	pageSize int
	limit    int
	offset   int
}

func parsePaginationParams(r *http.Request, defaultPageSize int, maxPageSize int) (paginationParams, error) {
	q := r.URL.Query()

	page := 1
	if raw := q.Get("page"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			return paginationParams{}, strconv.ErrSyntax
		}
		page = v
	}

	pageSize := defaultPageSize
	if raw := q.Get("page_size"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			return paginationParams{}, strconv.ErrSyntax
		}
		pageSize = v
	}

	if maxPageSize > 0 && pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	offset := (page - 1) * pageSize
	return paginationParams{page: page, pageSize: pageSize, limit: pageSize, offset: offset}, nil
}

func buildPagination(page int, pageSize int, total int) Pagination {
	totalPages := 0
	if pageSize > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(pageSize)))
	}
	return Pagination{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}
}

func writePaginatedResponse(w http.ResponseWriter, status int, data any, page int, pageSize int, total int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":       data,
		"pagination": buildPagination(page, pageSize, total),
	})
}

func writeJSONMessage(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"message": message})
}

func writeJSONErrorResponse(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": code, "message": message})
}
