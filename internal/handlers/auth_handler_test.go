package handlers

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strings"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	"scm/internal/config"
	"scm/internal/services"
)

type noopMailer struct{}

type recordingMailer struct {
	to      string
	subject string
	body    string
	calls   int
}

func TestForgotPasswordReturnsTokenWhenEnabled(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT id, email, name, user_name, phone_number, password_hash, last_login_at, created_at\s+FROM users\s+WHERE email = \$1`).
		WithArgs("a@b.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "name", "user_name", "phone_number", "password_hash", "last_login_at", "created_at"}).
			AddRow("u1", "a@b.com", "A", "a", "999", "hash", nil, time.Now().UTC()))

	mock.ExpectQuery("INSERT INTO password_reset_tokens").WillReturnRows(
		sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now().UTC()),
	)

	h := NewAuthHandler(db, &config.Config{JWTSecret: "dev", AuthReturnResetToken: true}, services.EmailSender(&noopMailer{}))

	payload := map[string]any{"email": "a@b.com"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/forgot-password", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.ForgotPassword(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d (%s)", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp["ok"] != true {
		t.Fatalf("expected ok=true got %v", resp)
	}
	if resp["token"] == nil {
		t.Fatalf("expected token in response got %v", resp)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestSignupWeakPasswordReturnsJSON(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	h := NewAuthHandler(db, &config.Config{JWTSecret: "dev"}, services.EmailSender(&noopMailer{}))
	payload := map[string]any{
		"email":        "a@b.com",
		"password":     "password123",
		"name":         "A",
		"user_name":    "a",
		"phone_number": "9999999999",
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.Signup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d (%s)", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] == nil {
		t.Fatalf("expected json error, got %v", resp)
	}
	msg, _ := resp["message"].(string)
	if !strings.Contains(msg, "password must be at least 8") {
		t.Fatalf("expected password policy message, got %v", resp)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestSignupInvalidUserNameReturnsJSON(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	h := NewAuthHandler(db, &config.Config{JWTSecret: "dev"}, services.EmailSender(&noopMailer{}))
	payload := map[string]any{
		"email":        "a@b.com",
		"password":     "Password1!",
		"name":         "A",
		"user_name":    "a b",
		"phone_number": "9999999999",
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.Signup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d (%s)", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] == nil {
		t.Fatalf("expected json error, got %v", resp)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestResetPasswordSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	rawToken := "abcd"
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	mock.ExpectQuery(`SELECT id, user_id, token_hash, expires_at, used_at, created_at\s+FROM password_reset_tokens`).
		WithArgs(tokenHash).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "token_hash", "expires_at", "used_at", "created_at"}).
			AddRow("t1", "u1", tokenHash, time.Now().UTC().Add(10*time.Minute), nil, time.Now().UTC()))

	mock.ExpectExec("UPDATE users SET password_hash").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE password_reset_tokens SET used_at").WillReturnResult(sqlmock.NewResult(0, 1))

	h := NewAuthHandler(db, &config.Config{JWTSecret: "dev"}, services.EmailSender(&noopMailer{}))
	payload := map[string]any{"token": rawToken, "new_password": "Newpass1!"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.ResetPassword(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d (%s)", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp["ok"] != true {
		t.Fatalf("expected ok=true got %v", resp)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func (n *noopMailer) Send(to string, subject string, body string) error { return nil }

func (m *recordingMailer) Send(to string, subject string, body string) error {
	m.to = to
	m.subject = subject
	m.body = body
	m.calls++
	return nil
}

func TestSignupSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("INSERT INTO users").WillReturnRows(
		sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now().UTC()),
	)
	mock.ExpectQuery("SELECT id FROM roles WHERE name = 'advertiser'").WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow("role-adv"),
	)
	mock.ExpectExec("INSERT INTO user_roles").WillReturnResult(sqlmock.NewResult(1, 1))

	m := &recordingMailer{}
	h := NewAuthHandler(db, &config.Config{JWTSecret: "dev", DashboardBaseURL: "https://dashboard.example"}, services.EmailSender(m))

	payload := map[string]any{
		"email": "a@b.com",
		"password": "Password1!",
		"name": "A",
		"user_name": "a",
		"phone_number": "9999999999",
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.Signup(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d (%s)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}
	if m.calls != 1 {
		t.Fatalf("expected welcome email to be sent once, got %d", m.calls)
	}
	if m.to != "a@b.com" {
		t.Fatalf("expected email to a@b.com, got %q", m.to)
	}
	if !strings.Contains(m.subject, "Welcome") {
		t.Fatalf("expected welcome subject, got %q", m.subject)
	}
	if !strings.Contains(m.body, "https://dashboard.example") {
		t.Fatalf("expected dashboard URL in email body, got %q", m.body)
	}
	if !strings.Contains(m.body, "Username") {
		t.Fatalf("expected username in email body, got %q", m.body)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestSignupDuplicateEmailReturnsJSON(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("INSERT INTO users").WillReturnError(&pq.Error{Code: "23505", Constraint: "users_email_key"})

	h := NewAuthHandler(db, &config.Config{JWTSecret: "dev"}, services.EmailSender(&noopMailer{}))
	payload := map[string]any{
		"email": "a@b.com",
		"password": "Password1!",
		"name": "A",
		"user_name": "a",
		"phone_number": "9999999999",
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.Signup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d (%s)", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] == nil {
		t.Fatalf("expected json error, got %v", resp)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestLoginSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte("Password1!"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword: %v", err)
	}

	mock.ExpectQuery(`SELECT id, email, name, user_name, phone_number, password_hash, last_login_at, created_at\s+FROM users`).WithArgs("a@b.com").WillReturnRows(
		sqlmock.NewRows([]string{"id", "email", "name", "user_name", "phone_number", "password_hash", "last_login_at", "created_at"}).
			AddRow("u1", "a@b.com", "A", "a", "999", string(hash), nil, time.Now().UTC()),
	)

	mock.ExpectQuery(`(?s)SELECT ro\.name, ur\.advertiser_id.*FROM user_roles ur.*JOIN roles ro ON ro\.id = ur\.role_id.*WHERE ur\.user_id = \$1`).WithArgs("u1").WillReturnRows(
		sqlmock.NewRows([]string{"name", "advertiser_id"}).
			AddRow("advertiser", nil),
	)

	mock.ExpectExec(`UPDATE users SET last_login_at`).WithArgs("u1").WillReturnResult(sqlmock.NewResult(0, 1))

	h := NewAuthHandler(db, &config.Config{JWTSecret: "dev"}, services.EmailSender(&noopMailer{}))
	payload := map[string]any{"identifier": "a@b.com", "password": "Password1!"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d (%s)", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp["access_token"] == nil {
		t.Fatalf("expected access_token, got %v", resp)
	}
	if resp["roles"] == nil {
		t.Fatalf("expected roles in response, got %v", resp)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
	_ = sql.ErrNoRows
}
