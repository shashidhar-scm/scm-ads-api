package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/lib/pq"
	"scm/internal/interfaces"
	"scm/internal/middleware"
	"scm/internal/models"
	"scm/internal/repository"
)

type AdvertiserHandler struct {
    repo      interfaces.AdvertiserRepository
    db        *sql.DB
    validator *validator.Validate
}

func NewAdvertiserHandler(repo interfaces.AdvertiserRepository, db *sql.DB) *AdvertiserHandler {
    return &AdvertiserHandler{
        repo:      repo,
        db:        db,
        validator: validator.New(),
    }
}

func (h *AdvertiserHandler) listAdvertisersCreatedByUser(ctx context.Context, userID string, limit int, offset int) ([]models.Advertiser, error) {
	if h.db == nil || strings.TrimSpace(userID) == "" {
		return []models.Advertiser{}, nil
	}
	query := `
		SELECT id, name, email, created_by, created_at, updated_at
		FROM advertisers
		WHERE created_by = $1
		ORDER BY name
	`
	args := []any{userID}
	argPos := 2
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, limit)
		argPos++
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, offset)
	}

	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Advertiser
	for rows.Next() {
		var adv models.Advertiser
		var createdBy sql.NullString
		if err := rows.Scan(&adv.ID, &adv.Name, &adv.Email, &createdBy, &adv.CreatedAt, &adv.UpdatedAt); err != nil {
			return nil, err
		}
		if createdBy.Valid {
			adv.CreatedBy = createdBy.String
		}
		out = append(out, adv)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []models.Advertiser{}
	}
	return out, nil
}

func (h *AdvertiserHandler) countAdvertisersCreatedByUser(ctx context.Context, userID string) (int, error) {
	if h.db == nil || strings.TrimSpace(userID) == "" {
		return 0, nil
	}
	var total int
	if err := h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM advertisers WHERE created_by = $1`, userID).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (h *AdvertiserHandler) searchAdvertisersCreatedByUser(ctx context.Context, userID string, term string, limit int, offset int) ([]models.Advertiser, int, error) {
	if h.db == nil || strings.TrimSpace(userID) == "" {
		return []models.Advertiser{}, 0, nil
	}
	likeTerm := fmt.Sprintf("%%%s%%", strings.ToLower(strings.TrimSpace(term)))

	var total int
	countQuery := `
		SELECT COUNT(*)
		FROM advertisers
		WHERE created_by = $1
		  AND (LOWER(name) LIKE $2 OR LOWER(email) LIKE $2)
	`
	if err := h.db.QueryRowContext(ctx, countQuery, userID, likeTerm).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, name, email, created_by, created_at, updated_at
		FROM advertisers
		WHERE created_by = $1
		  AND (LOWER(name) LIKE $2 OR LOWER(email) LIKE $2)
		ORDER BY name
	`
	args := []any{userID, likeTerm}
	argPos := 3
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, limit)
		argPos++
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, offset)
	}

	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []models.Advertiser
	for rows.Next() {
		var adv models.Advertiser
		var createdBy sql.NullString
		if err := rows.Scan(&adv.ID, &adv.Name, &adv.Email, &createdBy, &adv.CreatedAt, &adv.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if createdBy.Valid {
			adv.CreatedBy = createdBy.String
		}
		out = append(out, adv)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if out == nil {
		out = []models.Advertiser{}
	}
	return out, total, nil
}

func (h *AdvertiserHandler) getAdvertiserCreatedByUser(ctx context.Context, advertiserID string, userID string) (*models.Advertiser, error) {
	if h.db == nil || strings.TrimSpace(userID) == "" || strings.TrimSpace(advertiserID) == "" {
		return nil, sql.ErrNoRows
	}
	query := `
		SELECT id, name, email, created_by, created_at, updated_at
		FROM advertisers
		WHERE id = $1
		  AND created_by = $2
	`
	var adv models.Advertiser
	var createdBy sql.NullString
	if err := h.db.QueryRowContext(ctx, query, advertiserID, userID).Scan(&adv.ID, &adv.Name, &adv.Email, &createdBy, &adv.CreatedAt, &adv.UpdatedAt); err != nil {
		return nil, err
	}
	if createdBy.Valid {
		adv.CreatedBy = createdBy.String
	}
	return &adv, nil
}

func (h *AdvertiserHandler) assignCreatorAdvertiserRole(ctx context.Context, userID string, advertiserID string) error {
	return nil
}

// @Tags Advertisers
// @Summary Create advertiser
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body models.CreateAdvertiserRequest true "Create advertiser request"
// @Success 201 {object} models.Advertiser
// @Failure 400 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/advertisers/ [post]
func (h *AdvertiserHandler) CreateAdvertiser(w http.ResponseWriter, r *http.Request) {
	log.Println("=== CreateAdvertiser handler called ===")
	createdBy, _ := r.Context().Value(middleware.CtxUserID).(string)
	if strings.TrimSpace(createdBy) == "" {
		writeJSONErrorResponse(w, http.StatusUnauthorized, "unauthorized", "missing user identity")
		return
	}
	
	var req models.CreateAdvertiserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	advertiser := &models.Advertiser{
		Name:      req.Name,
		Email:     req.Email,
		CreatedBy: createdBy,
	}

	if err := h.repo.Create(r.Context(), advertiser); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			// 23505 = unique_violation
			if pqErr.Code == "23505" {
				writeJSONErrorResponse(w, http.StatusConflict, "advertiser_already_exists", "Advertiser already exists")
				return
			}
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "create_advertiser_failed", "Failed to create advertiser")
		return
	}

	// No role assignment needed: advertiser access is determined by advertisers.created_by.

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(advertiser)
}

// @Tags Advertisers
// @Summary Get advertiser
// @Security BearerAuth
// @Produce json
// @Param id path string true "Advertiser ID"
// @Success 200 {object} models.Advertiser
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/advertisers/{id}/ [get]
func (h *AdvertiserHandler) GetAdvertiser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "id is required")
		return
	}

	userID, _ := r.Context().Value(middleware.CtxUserID).(string)
	isSuper := false
	isAdmin := false
	if h.db != nil && strings.TrimSpace(userID) != "" {
		ur := repository.NewUserRoleRepository(h.db)
		v, err := ur.IsSuperAdmin(r.Context(), userID)
		if err == nil {
			isSuper = v
		}
		va, err := ur.IsAdmin(r.Context(), userID)
		if err == nil {
			isAdmin = va
		}
	}

	var advertiser *models.Advertiser
	var err error
	if isSuper || isAdmin {
		advertiser, err = h.repo.GetByID(r.Context(), id)
	} else {
		advertiser, err = h.getAdvertiserCreatedByUser(r.Context(), id, userID)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "advertiser_not_found", "Advertiser not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_advertiser_failed", "Failed to get advertiser")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(advertiser)
}

// @Tags Advertisers
// @Summary List advertisers
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Advertiser
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/advertisers/ [get]
func (h *AdvertiserHandler) ListAdvertisers(w http.ResponseWriter, r *http.Request) {
	p, err := parsePaginationParams(r, 50, 200)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "invalid pagination parameters")
		return
	}

	permissionGlobal, _ := r.Context().Value(middleware.CtxPermissionGlobal).(bool)
	userID, _ := r.Context().Value(middleware.CtxUserID).(string)

	isSuper := false
	isAdmin := false
	if h.db != nil && strings.TrimSpace(userID) != "" {
		ur := repository.NewUserRoleRepository(h.db)
		v, err := ur.IsSuperAdmin(r.Context(), userID)
		if err == nil {
			isSuper = v
		}
		va, err := ur.IsAdmin(r.Context(), userID)
		if err == nil {
			isAdmin = va
		}
	}
	isGlobalAdmin := isSuper || isAdmin

	var total int
	if permissionGlobal {
		if isGlobalAdmin {
			total, err = h.repo.Count(r.Context())
		} else if h.db != nil {
			total, err = h.countAdvertisersCreatedByUser(r.Context(), userID)
		} else {
			total, err = h.repo.Count(r.Context())
		}
	}
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_advertisers_failed", "Failed to list advertisers")
		return
	}

	var advertisers []models.Advertiser
	if permissionGlobal {
		if isGlobalAdmin {
			advertisers, err = h.repo.List(r.Context(), p.limit, p.offset)
		} else if h.db != nil {
			advertisers, err = h.listAdvertisersCreatedByUser(r.Context(), userID, p.limit, p.offset)
		} else {
			advertisers, err = h.repo.List(r.Context(), p.limit, p.offset)
		}
	}
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "list_advertisers_failed", "Failed to list advertisers")
		return
	}

	if advertisers == nil {
		advertisers = []models.Advertiser{} // Return empty array instead of null
	}

	writePaginatedResponse(w, http.StatusOK, advertisers, p.page, p.pageSize, total)
}

// @Tags Advertisers
// @Summary Search advertisers
// @Security BearerAuth
// @Produce json
// @Param query query string true "Search text (matches name or email)"
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/advertisers/search [get]
func (h *AdvertiserHandler) SearchAdvertisers(w http.ResponseWriter, r *http.Request) {
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

	permissionGlobal, _ := r.Context().Value(middleware.CtxPermissionGlobal).(bool)
	userID, _ := r.Context().Value(middleware.CtxUserID).(string)

	isSuper := false
	isAdmin := false
	if h.db != nil && strings.TrimSpace(userID) != "" {
		ur := repository.NewUserRoleRepository(h.db)
		v, err := ur.IsSuperAdmin(r.Context(), userID)
		if err == nil {
			isSuper = v
		}
		va, err := ur.IsAdmin(r.Context(), userID)
		if err == nil {
			isAdmin = va
		}
	}
	isGlobalAdmin := isSuper || isAdmin

	var advertisers []models.Advertiser
	var total int
	if permissionGlobal {
		if isGlobalAdmin {
			advertisers, total, err = h.repo.Search(r.Context(), query, p.limit, p.offset)
		} else if h.db != nil {
			advertisers, total, err = h.searchAdvertisersCreatedByUser(r.Context(), userID, query, p.limit, p.offset)
		} else {
			advertisers, total, err = h.repo.Search(r.Context(), query, p.limit, p.offset)
		}
	}
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "search_advertisers_failed", "Failed to search advertisers")
		return
	}

	if advertisers == nil {
		advertisers = []models.Advertiser{}
	}

	writePaginatedResponse(w, http.StatusOK, advertisers, p.page, p.pageSize, total)
}

// @Tags Advertisers
// @Summary Update advertiser
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Advertiser ID"
// @Param body body models.UpdateAdvertiserRequest true "Update advertiser request"
// @Success 200 {object} models.Advertiser
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/advertisers/{id}/ [put]
func (h *AdvertiserHandler) UpdateAdvertiser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "Advertiser ID is required")
		return
	}

	_, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "advertiser_not_found", "Advertiser not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_advertiser_failed", "Failed to get advertiser")
		return
	}

	var req models.UpdateAdvertiserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Name == nil && req.Email == nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "No fields to update")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := h.repo.Update(r.Context(), id, &req); err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "update_advertiser_failed", "Failed to update advertiser")
		return
	}

	advertiser, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "advertiser_not_found", "Advertiser not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "get_advertiser_failed", "Failed to get advertiser")
		return
	}


	w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(advertiser)
}

// @Tags Advertisers
// @Summary Delete advertiser
// @Security BearerAuth
// @Produce json
// @Param id path string true "Advertiser ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/advertisers/{id}/ [delete]
func (h *AdvertiserHandler) DeleteAdvertiser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "validation_error", "Advertiser ID is required")
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		var blocked *interfaces.DeletionBlockedError
		if errors.As(err, &blocked) {
			msg := fmt.Sprintf("Cannot delete %s: referenced by", blocked.Resource)
			for k, v := range blocked.References {
				msg += fmt.Sprintf(" %d %s", v, k)
			}
			writeJSONErrorResponse(w, http.StatusConflict, "delete_blocked", msg)
			return
		}
		if err == sql.ErrNoRows {
			writeJSONErrorResponse(w, http.StatusNotFound, "advertiser_not_found", "Advertiser not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "delete_advertiser_failed", "Failed to delete advertiser")
		return
	}
	// Success response
	writeJSONMessage(w, http.StatusOK, "advertiser deleted successfully")
}
