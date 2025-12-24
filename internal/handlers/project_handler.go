package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"scm/internal/repository"
)

type ProjectHandler struct {
	repo repository.ProjectRepository
}

func NewProjectHandler(repo repository.ProjectRepository) *ProjectHandler {
	return &ProjectHandler{repo: repo}
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	pagination, err := parsePaginationParams(r, 20, 100)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_pagination", "invalid pagination: "+err.Error())
		return
	}

	projects, err := h.repo.List(r.Context(), pagination.limit, pagination.offset)
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to list projects: "+err.Error())
		return
	}

	total, err := h.repo.Count(r.Context())
	if err != nil {
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to count projects: "+err.Error())
		return
	}

	writePaginatedResponse(w, http.StatusOK, projects, pagination.page, pagination.pageSize, total)
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeJSONErrorResponse(w, http.StatusBadRequest, "invalid_request", "project name is required")
		return
	}

	_, err := h.repo.GetByName(r.Context(), name)
	if err != nil {
		if err.Error() == "project not found" {
			writeJSONErrorResponse(w, http.StatusNotFound, "not_found", "project not found")
			return
		}
		writeJSONErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to get project: "+err.Error())
		return
	}

	writeJSONMessage(w, http.StatusOK, "project retrieved")
}
