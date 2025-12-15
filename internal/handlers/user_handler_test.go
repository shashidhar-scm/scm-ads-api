package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"scm/internal/models"
)

type mockUserRepo struct {
	users map[string]*models.User
}

func (m *mockUserRepo) Create(ctx context.Context, user *models.User) error                 { return nil }
func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) { return nil, nil }
func (m *mockUserRepo) GetByIdentifier(ctx context.Context, identifier string) (*models.User, error) {
	return nil, nil
}
func (m *mockUserRepo) ListAll(ctx context.Context) ([]models.User, error) {
	var out []models.User
	for _, u := range m.users {
		out = append(out, *u)
	}
	return out, nil
}
func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	u := m.users[id]
	if u == nil {
		return nil, errUserNotFound
	}
	return u, nil
}
func (m *mockUserRepo) UpdateProfile(ctx context.Context, id string, req *models.UpdateUserRequest) error {
	u := m.users[id]
	if u == nil {
		return errUserNotFound
	}
	if req.Name != nil {
		u.Name = *req.Name
	}
	return nil
}
func (m *mockUserRepo) UpdatePasswordHash(ctx context.Context, userID string, passwordHash string) error {
	return nil
}
func (m *mockUserRepo) Delete(ctx context.Context, id string) error {
	if m.users[id] == nil {
		return errUserNotFound
	}
	delete(m.users, id)
	return nil
}

var errUserNotFound = &mockErr{"user not found"}

type mockErr struct{ s string }

func (e *mockErr) Error() string { return e.s }

func TestDeleteUserNotFoundReturnsJSON(t *testing.T) {
	repo := &mockUserRepo{users: map[string]*models.User{}}
	h := NewUserHandler(repo)

	r := chi.NewRouter()
	r.Delete("/users/{id}", h.DeleteUser)

	req := httptest.NewRequest(http.MethodDelete, "/users/does-not-exist", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (%s)", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp["error"] != "user_not_found" {
		t.Fatalf("expected error=user_not_found got %v", resp)
	}
}

func TestUpdateUserReturnsJSON(t *testing.T) {
	repo := &mockUserRepo{users: map[string]*models.User{"u1": {ID: "u1", Email: "a@b.com", CreatedAt: time.Now().UTC()}}}
	h := NewUserHandler(repo)

	r := chi.NewRouter()
	r.Put("/users/{id}", h.UpdateUser)

	payload := map[string]any{"name": "New"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/users/u1", bytes.NewReader(b))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d (%s)", w.Code, w.Body.String())
	}
	var u models.User
	if err := json.Unmarshal(w.Body.Bytes(), &u); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if u.Name != "New" {
		t.Fatalf("expected name updated, got %+v", u)
	}
}
