package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"scm/internal/middleware"
)

func TestSetUserRolesNonSuperAdminCannotChangeRoles(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT EXISTS\(`).
		WithArgs("caller-1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	mock.ExpectQuery(`SELECT role_id FROM user_roles WHERE user_id = \$1`).
		WithArgs("u1").
		WillReturnRows(sqlmock.NewRows([]string{"role_id"}).AddRow("role-old"))

	h := NewRBACHandler(db)
	r := chi.NewRouter()
	r.Put("/users/{id}/roles", h.SetUserRoles)

	payload := map[string]any{"role_id": "role-new"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/users/u1/roles", bytes.NewReader(b))
	req = req.WithContext(context.WithValue(req.Context(), middleware.CtxUserID, "caller-1"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d (%s)", w.Code, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestSetUserRolesNonSuperAdminNoOpAllowed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT EXISTS\(`).
		WithArgs("caller-1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	mock.ExpectQuery(`SELECT role_id FROM user_roles WHERE user_id = \$1`).
		WithArgs("u1").
		WillReturnRows(sqlmock.NewRows([]string{"role_id"}).AddRow("role-same"))

	h := NewRBACHandler(db)
	r := chi.NewRouter()
	r.Put("/users/{id}/roles", h.SetUserRoles)

	payload := map[string]any{"role_id": "role-same"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/users/u1/roles", bytes.NewReader(b))
	req = req.WithContext(context.WithValue(req.Context(), middleware.CtxUserID, "caller-1"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d (%s)", w.Code, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
