package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lib/pq"
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
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.v.Struct(req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "hash_failed", "Failed to create user")
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
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			// 23505 = unique_violation
			if pqErr.Code == "23505" {
				switch pqErr.Constraint {
				case "users_email_key":
					writeJSONError(w, http.StatusBadRequest, "email_already_exists", "Email already exists")
					return
				case "users_user_name_key":
					writeJSONError(w, http.StatusBadRequest, "user_name_already_exists", "User name already exists")
					return
				case "users_phone_number_key":
					writeJSONError(w, http.StatusBadRequest, "phone_number_already_exists", "Phone number already exists")
					return
				default:
					writeJSONError(w, http.StatusBadRequest, "unique_violation", "User already exists")
					return
				}
			}
			// 42P01 = undefined_table (migrations not applied)
			if pqErr.Code == "42P01" {
				writeJSONError(w, http.StatusInternalServerError, "schema_missing", "Database schema not initialized (missing table)")
				return
			}
		}

		if h.cfg.AuthVerboseErrors {
			writeJSONError(w, http.StatusInternalServerError, "create_user_failed", err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "create_user_failed", "Failed to create user")
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
		writeJSONError(w, http.StatusInternalServerError, "token_sign_failed", "Failed to login")
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
