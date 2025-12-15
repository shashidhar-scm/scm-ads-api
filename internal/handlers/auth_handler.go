package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"scm/internal/config"
	"scm/internal/models"
	"scm/internal/repository"
	"scm/internal/services"
)

func writeJSONError(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":   code,
		"message": message,
	})
}

type AuthHandler struct {
	users  repository.UserRepository
	resets repository.PasswordResetRepository
	mailer services.EmailSender
	cfg    *config.Config
	v      *validator.Validate
}

func NewAuthHandler(db *sql.DB, cfg *config.Config, mailer services.EmailSender) *AuthHandler {
	return &AuthHandler{
		users:  repository.NewUserRepository(db),
		resets: repository.NewPasswordResetRepository(db),
		mailer: mailer,
		cfg:    cfg,
		v:      validator.New(),
	}
}

func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req models.SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.v.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	u := &models.User{
		ID:           uuid.NewString(),
		Email:        req.Email,
		Name:         req.Name,
		UserName:     req.UserName,
		PhoneNumber:  req.PhoneNumber,
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
	}

	if err := h.users.Create(r.Context(), u); err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"id": u.ID, "email": u.Email, "created_at": u.CreatedAt})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.v.Struct(req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	u, err := h.users.GetByIdentifier(r.Context(), req.Identifier)
	if err != nil {
		if h.cfg.AuthVerboseErrors {
			writeJSONError(w, http.StatusUnauthorized, "invalid_identifier", "Email/username/phone not found")
			return
		}
		writeJSONError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		if h.cfg.AuthVerboseErrors {
			writeJSONError(w, http.StatusUnauthorized, "invalid_password", "Password is incorrect")
			return
		}
		writeJSONError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid credentials")
		return
	}

	expiresIn := h.cfg.JWTExpiresInSeconds
	if expiresIn <= 0 {
		expiresIn = 86400
	}

	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub":   u.ID,
		"email": u.Email,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Duration(expiresIn) * time.Second).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.cfg.JWTSecret))
	if err != nil {
		http.Error(w, "Failed to login", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(models.LoginResponse{
		AccessToken: signed,
		ExpiresIn:   expiresIn,
		Email:       u.Email,
		Name:        u.Name,
		UserName:    u.UserName,
		PhoneNumber: u.PhoneNumber,
	})
}

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req models.ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.v.Struct(req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	// Always return 200 to avoid user enumeration
	u, err := h.users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		return
	}

	rawToken, tokenHash, err := generateResetToken()
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	expiresAt := time.Now().UTC().Add(30 * time.Minute)
	prt := &models.PasswordResetToken{
		ID:        uuid.NewString(),
		UserID:    u.ID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}
	_ = h.resets.Create(r.Context(), prt)

	subject := "Reset your password"
	body := "Use this token to reset your password:\n\n" + rawToken + "\n\nThis token expires in 30 minutes."
	_ = h.mailer.Send(u.Email, subject, body)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := map[string]any{"ok": true}
	if h.cfg.AuthReturnResetToken {
		resp["token"] = rawToken
		resp["expires_in_seconds"] = int64(1800)
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req models.ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.v.Struct(req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	hash := sha256.Sum256([]byte(req.Token))
	tokenHash := hex.EncodeToString(hash[:])

	token, err := h.resets.GetValidByTokenHash(r.Context(), tokenHash)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_token", "Invalid or expired token")
		return
	}

	pwHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "reset_failed", "Failed to reset password")
		return
	}

	if err := h.users.UpdatePasswordHash(r.Context(), token.UserID, string(pwHash)); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "reset_failed", "Failed to reset password")
		return
	}

	_ = h.resets.MarkUsed(r.Context(), token.ID, time.Now().UTC())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"message": "Password reset successful",
	})
}

func generateResetToken() (rawToken string, tokenHash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	rawToken = hex.EncodeToString(b)
	h := sha256.Sum256([]byte(rawToken))
	tokenHash = hex.EncodeToString(h[:])
	return rawToken, tokenHash, nil
}
