package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

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

func validationMessage(err error) string {
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return err.Error()
	}
	for _, fe := range verrs {
		switch fe.Tag() {
		case "strongpassword":
			return "password must be at least 8 characters and contain at least 1 uppercase, 1 lowercase, 1 number, and 1 special character"
		case "alphanum":
			return "user_name must contain only letters and numbers"
		}
	}
	return err.Error()
}

type AuthHandler struct {
	users  repository.UserRepository
	resets repository.PasswordResetRepository
	userRoles repository.UserRoleRepository
	mailer services.EmailSender
	db     *sql.DB
	cfg    *config.Config
	v      *validator.Validate
}

func NewAuthHandler(db *sql.DB, cfg *config.Config, mailer services.EmailSender) *AuthHandler {
	v := validator.New()
	_ = v.RegisterValidation("strongpassword", strongPassword)
	return &AuthHandler{
		users:  repository.NewUserRepository(db),
		resets: repository.NewPasswordResetRepository(db),
		userRoles: repository.NewUserRoleRepository(db),
		mailer: mailer,
		db:     db,
		cfg:    cfg,
		v:      v,
	}
}

func strongPassword(fl validator.FieldLevel) bool {
	s, ok := fl.Field().Interface().(string)
	if !ok {
		return false
	}
	if len(s) < 8 {
		return false
	}
	hasUpper := false
	hasLower := false
	hasNumber := false
	hasSpecial := false
	for _, r := range s {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasNumber = true
		default:
			hasSpecial = true
		}
	}
	return hasUpper && hasLower && hasNumber && hasSpecial
}

func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req models.SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.v.Struct(req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "validation_error", validationMessage(err))
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

	// Assign the default global advertiser role to every new signup.
	if h.db != nil {
		var roleID string
		if err := h.db.QueryRowContext(r.Context(), `SELECT id FROM roles WHERE name = 'advertiser' LIMIT 1`).Scan(&roleID); err != nil {
			_, _ = h.db.ExecContext(r.Context(), `DELETE FROM users WHERE id = $1`, u.ID)
			writeJSONError(w, http.StatusInternalServerError, "create_user_failed", "Failed to create user")
			return
		}
		if _, err := h.db.ExecContext(r.Context(), `INSERT INTO user_roles (user_id, role_id, advertiser_id) VALUES ($1, $2, NULL) ON CONFLICT DO NOTHING`, u.ID, roleID); err != nil {
			_, _ = h.db.ExecContext(r.Context(), `DELETE FROM users WHERE id = $1`, u.ID)
			writeJSONError(w, http.StatusInternalServerError, "create_user_failed", "Failed to create user")
			return
		}
	}

	subject := "Welcome to SCM Ads"
	dashboardURL := strings.TrimSpace(h.cfg.DashboardBaseURL)
	body := "<html><body style=\"font-family:Arial,sans-serif; color:#111;\">" +
		"<h2 style=\"margin:0 0 12px 0;\">Welcome, " + req.Name + "</h2>" +
		"<p style=\"margin:0 0 16px 0;\">Your account has been created successfully.</p>" +
		"<p style=\"margin:0 0 16px 0;\">You can log in using your <b>username</b>, <b>email</b>, or <b>phone number</b>.</p>" +
		"<p style=\"margin:0 0 8px 0;\">Your username:</p>" +
		"<p style=\"margin:0 0 16px 0;\"><b>Username:</b> " + req.UserName + "</p>"
	if dashboardURL != "" {
		body += "<p style=\"margin:0 0 20px 0;\">Dashboard: <a href=\"" + dashboardURL + "\">" + dashboardURL + "</a></p>"
	}
	body += "</body></html>"
	if err := h.mailer.Send(u.Email, subject, body); err != nil {
		log.Printf("signup: failed to send welcome email to %s: %v", u.Email, err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"id": u.ID, "email": u.Email, "created_at": u.CreatedAt})
}

// @Tags Account
// @Summary Login
// @Accept json
// @Produce json
// @Param body body models.LoginRequest true "Login request"
// @Success 200 {object} models.LoginResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.v.Struct(req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "validation_error", validationMessage(err))
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

	var roles []models.LoginRole
	if h.db != nil {
		rows, err := h.db.QueryContext(r.Context(), `
			SELECT ro.name, ur.advertiser_id
			FROM user_roles ur
			JOIN roles ro ON ro.id = ur.role_id
			WHERE ur.user_id = $1
			ORDER BY ro.name ASC, ur.advertiser_id ASC
		`, u.ID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "login_failed", "Failed to login")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			var advertiserID sql.NullString
			if err := rows.Scan(&name, &advertiserID); err != nil {
				writeJSONError(w, http.StatusInternalServerError, "login_failed", "Failed to login")
				return
			}
			var adv *string
			if advertiserID.Valid {
				v := advertiserID.String
				adv = &v
			}
			roles = append(roles, models.LoginRole{Name: name, AdvertiserID: adv})
		}
		if err := rows.Err(); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "login_failed", "Failed to login")
			return
		}

		// Best-effort update of last login timestamp.
		if _, err := h.db.ExecContext(r.Context(), `UPDATE users SET last_login_at = NOW() AT TIME ZONE 'UTC' WHERE id = $1`, u.ID); err != nil {
			log.Printf("login: failed to update last_login_at for user_id=%s: %v", u.ID, err)
		} else {
			now := time.Now().UTC()
			u.LastLoginAt = &now
		}
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
		ID:          u.ID,
		Email:       u.Email,
		Name:        u.Name,
		UserName:    u.UserName,
		PhoneNumber: u.PhoneNumber,
		LastLoginAt: u.LastLoginAt,
		Roles:       roles,
	})
}

// @Tags Account
// @Summary Forgot password
// @Accept json
// @Produce json
// @Param body body models.ForgotPasswordRequest true "Forgot password request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req models.ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.v.Struct(req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "validation_error", validationMessage(err))
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
	if err := h.resets.Create(r.Context(), prt); err != nil {
		log.Printf("forgot-password: failed to create reset token for user_id=%s: %v", u.ID, err)
	}

	subject := "Reset your password"
	resetURL := h.cfg.AuthResetPasswordURL
	if resetURL != "" {
		sep := "?"
		if strings.Contains(resetURL, "?") {
			sep = "&"
		}
		resetURL = resetURL + sep + "token=" + url.QueryEscape(rawToken)
	}
	expiresInMinutes := int64(30)
	if seconds := int64(1800); seconds > 0 {
		expiresInMinutes = seconds / 60
	}
	body := "<html><body style=\"font-family:Arial,sans-serif; color:#111;\">" +
		"<h2 style=\"margin:0 0 12px 0;\">Reset your password</h2>" +
		"<p style=\"margin:0 0 16px 0;\">We received a request to reset your password.</p>"
	if resetURL != "" {
		body += "<p style=\"margin:0 0 20px 0;\">" +
			"<a href=\"" + resetURL + "\" style=\"display:inline-block; background:#2563eb; color:#ffffff; text-decoration:none; padding:12px 18px; border-radius:8px; font-weight:600;\">Reset</a>" +
			"</p>"
	}
	body += "<p style=\"margin:0 0 8px 0;\">If the button doesn’t work, copy and paste this link into your browser:</p>"
	if resetURL != "" {
		body += "<p style=\"margin:0 0 16px 0;\"><a href=\"" + resetURL + "\">" + resetURL + "</a></p>"
	}
	body += "<p style=\"margin:0 0 8px 0;\">If you need it, here is your reset token:</p>" +
		"<p style=\"margin:0 0 16px 0; font-family:ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;\">" + rawToken + "</p>" +
		"<p style=\"margin:0; color:#444;\">This token expires in " + strconv.FormatInt(expiresInMinutes, 10) + " minutes.</p>" +
		"</body></html>"
	if err := h.mailer.Send(u.Email, subject, body); err != nil {
		log.Printf("forgot-password: failed to send email to %s: %v", u.Email, err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := map[string]any{"ok": true}
	if h.cfg.AuthReturnResetToken {
		resp["token"] = rawToken
		resp["expires_in_seconds"] = int64(1800)
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// @Tags Account
// @Summary Reset password
// @Accept json
// @Produce json
// @Param body body models.ResetPasswordRequest true "Reset password request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/auth/reset-password [post]
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req models.ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.v.Struct(req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "validation_error", validationMessage(err))
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
