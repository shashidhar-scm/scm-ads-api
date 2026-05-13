package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

func usernameBaseFromName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "user"
	}
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			// drop
		}
	}
	out := b.String()
	if out == "" {
		out = "user"
	}
	if len(out) > 24 {
		out = out[:24]
	}
	return out
}

func (h *AuthHandler) generateUniqueUserName(ctx context.Context, base string) string {
	base = strings.TrimSpace(strings.ToLower(base))
	if base == "" {
		base = "user"
	}
	if h.db == nil {
		return base
	}

	// Try base, then base2..base99
	for i := 0; i < 100; i++ {
		candidate := base
		if i > 0 {
			candidate = base + strconv.Itoa(i+1)
		}
		// Keep within a safe length.
		if len(candidate) > 30 {
			candidate = candidate[:30]
		}
		var exists bool
		err := h.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(user_name) = LOWER($1))`, candidate).Scan(&exists)
		if err != nil {
			return candidate
		}
		if !exists {
			return candidate
		}
	}
	return base + strconv.FormatInt(time.Now().Unix()%100000, 10)
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
	users        repository.UserRepository
	resets       repository.PasswordResetRepository
	userRoles    repository.UserRoleRepository
	mailer       services.EmailSender
	db           *sql.DB
	cfg          *config.Config
	v            *validator.Validate
	googleVerify func(ctx context.Context, idToken string) (googleTokenClaims, error)
	tempPassword func() (string, error)
}

type googleTokenClaims struct {
	Email string
	Name  string
}

func NewAuthHandler(db *sql.DB, cfg *config.Config, mailer services.EmailSender, userRepo repository.UserRepository, userRoleRepo repository.UserRoleRepository) *AuthHandler {
	v := validator.New()
	_ = v.RegisterValidation("strongpassword", strongPassword)
	return &AuthHandler{
		users:     userRepo,
		resets:    repository.NewPasswordResetRepository(db),
		userRoles: userRoleRepo,
		mailer:    mailer,
		db:        db,
		cfg:       cfg,
		v:         v,
		googleVerify: func(ctx context.Context, idToken string) (googleTokenClaims, error) {
			return verifyGoogleIDTokenWithTokenInfo(ctx, strings.TrimSpace(cfg.GoogleClientID), idToken)
		},
		tempPassword: generateTemporaryPassword,
	}
}

func generateTemporaryPassword() (string, error) {
	// Generate a strong temporary password: >= 16 chars with upper/lower/number/special.
	// Avoid ambiguous characters to reduce support issues (e.g. 0 vs O, l vs I).
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	const n = 18
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	// Ensure required character classes.
	return string(out) + "Aa1!", nil
}

func buildWelcomeEmailBody(name string, dashboardURL string, userName string, tempPassword *string, resetURL string, createdWithGoogle bool) string {
	name = strings.TrimSpace(name)
	dashboardURL = strings.TrimSpace(dashboardURL)
	userName = strings.TrimSpace(userName)
	resetURL = strings.TrimSpace(resetURL)

	body := "<html><body style=\"font-family:Arial,sans-serif; color:#111;\">" +
		"<h2 style=\"margin:0 0 12px 0;\">Welcome, " + name + "</h2>"

	if createdWithGoogle {
		body += "<p style=\"margin:0 0 16px 0;\">Your account was created using Google Sign-In.</p>"
	} else {
		body += "<p style=\"margin:0 0 16px 0;\">Your account has been created successfully.</p>" +
			"<p style=\"margin:0 0 16px 0;\">You can log in using your <b>username</b>, <b>email</b>, or <b>phone number</b>.</p>" +
			"<p style=\"margin:0 0 8px 0;\">Your username:</p>" +
			"<p style=\"margin:0 0 16px 0; font-family:ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;\">" + userName + "</p>"
	}

	body += "<p style=\"margin:0 0 16px 0;\"><b>Status:</b> Your account is pending approval. We will activate it soon.</p>"

	if tempPassword != nil {
		body += "<p style=\"margin:0 0 8px 0;\">Temporary password (only needed if you want to login without Google):</p>" +
			"<p style=\"margin:0 0 16px 0; font-family:ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;\">" + strings.TrimSpace(*tempPassword) + "</p>"
	}

	if dashboardURL != "" {
		body += "<p style=\"margin:0 0 20px 0;\">Dashboard: <a href=\"" + dashboardURL + "\">" + dashboardURL + "</a></p>"
	}
	if tempPassword != nil && resetURL != "" {
		body += "<p style=\"margin:0 0 16px 0;\">You can change your password anytime using \"Forgot password\": <a href=\"" + resetURL + "\">" + resetURL + "</a></p>"
	}
	body += "</body></html>"
	return body
}

func verifyGoogleIDTokenWithTokenInfo(ctx context.Context, clientID string, idToken string) (googleTokenClaims, error) {
	idToken = strings.TrimSpace(idToken)
	if idToken == "" {
		return googleTokenClaims{}, fmt.Errorf("id_token is required")
	}
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return googleTokenClaims{}, fmt.Errorf("GOOGLE_CLIENT_ID is required")
	}

	u, err := url.Parse("https://oauth2.googleapis.com/tokeninfo")
	if err != nil {
		return googleTokenClaims{}, err
	}
	q := u.Query()
	q.Set("id_token", idToken)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return googleTokenClaims{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return googleTokenClaims{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return googleTokenClaims{}, fmt.Errorf("google tokeninfo failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		Email         string `json:"email"`
		EmailVerified string `json:"email_verified"`
		Name          string `json:"name"`
		Aud           string `json:"aud"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return googleTokenClaims{}, fmt.Errorf("google tokeninfo: invalid json: %w", err)
	}
	if strings.TrimSpace(out.Aud) != clientID {
		return googleTokenClaims{}, fmt.Errorf("google token aud mismatch")
	}
	if !strings.EqualFold(strings.TrimSpace(out.EmailVerified), "true") {
		return googleTokenClaims{}, fmt.Errorf("google email not verified")
	}
	if strings.TrimSpace(out.Email) == "" {
		return googleTokenClaims{}, fmt.Errorf("google token missing email")
	}
	return googleTokenClaims{Email: out.Email, Name: out.Name}, nil
}

func (h *AuthHandler) writeLoginResponse(w http.ResponseWriter, r *http.Request, u *models.User) {
	expiresIn := h.cfg.JWTExpiresInSeconds
	if expiresIn <= 0 {
		expiresIn = 86400
	}

	var roles []models.LoginRole
	if h.db != nil {
		rows, err := h.db.QueryContext(r.Context(), `
			SELECT ro.name
			FROM user_roles ur
			JOIN roles ro ON ro.id = ur.role_id
			WHERE ur.user_id = $1
			ORDER BY ro.name ASC
		`, u.ID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "login_failed", "Failed to login")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				writeJSONError(w, http.StatusInternalServerError, "login_failed", "Failed to login")
				return
			}
			roles = append(roles, models.LoginRole{Name: name})
			break
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

	role := ""
	if len(roles) > 0 {
		role = roles[0].Name
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
		Role:        role,
	})
}

// @Tags Account
// @Summary Login with Google
// @Accept json
// @Produce json
// @Param body body models.GoogleAuthRequest true "Google auth request"
// @Success 200 {object} models.LoginResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/auth/google [post]
func (h *AuthHandler) GoogleAuth(w http.ResponseWriter, r *http.Request) {
	var req models.GoogleAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}
	if err := h.v.Struct(req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "validation_error", validationMessage(err))
		return
	}
	if h.googleVerify == nil {
		writeJSONError(w, http.StatusInternalServerError, "google_not_configured", "Google auth is not configured")
		return
	}

	claims, err := h.googleVerify(r.Context(), req.IDToken)
	if err != nil {
		if h.cfg != nil && h.cfg.AuthVerboseErrors {
			writeJSONError(w, http.StatusUnauthorized, "invalid_google_token", err.Error())
			return
		}
		writeJSONError(w, http.StatusUnauthorized, "invalid_google_token", "Invalid Google token")
		return
	}

	email := strings.TrimSpace(claims.Email)
	if email == "" {
		writeJSONError(w, http.StatusUnauthorized, "invalid_google_token", "Invalid Google token")
		return
	}

	u, err := h.users.GetByEmail(r.Context(), email)
	if err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "not found") {
			writeJSONError(w, http.StatusInternalServerError, "google_auth_failed", "Failed to login")
			return
		}

		tmp := ""
		if h.tempPassword != nil {
			pw, pErr := h.tempPassword()
			if pErr != nil {
				writeJSONError(w, http.StatusInternalServerError, "google_auth_failed", "Failed to login")
				return
			}
			tmp = pw
		}
		if strings.TrimSpace(tmp) == "" {
			writeJSONError(w, http.StatusInternalServerError, "google_auth_failed", "Failed to login")
			return
		}

		hash, hErr := bcrypt.GenerateFromPassword([]byte(tmp), bcrypt.DefaultCost)
		if hErr != nil {
			writeJSONError(w, http.StatusInternalServerError, "google_auth_failed", "Failed to login")
			return
		}

		u = &models.User{
			ID:           uuid.NewString(),
			Email:        email,
			Name:         strings.TrimSpace(claims.Name),
			UserName:     h.generateUniqueUserName(r.Context(), usernameBaseFromName(claims.Name)),
			Status:       "pending",
			PasswordHash: string(hash),
			CreatedAt:    time.Now().UTC(),
		}
		if err := h.users.Create(r.Context(), u); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "google_auth_failed", "Failed to login")
			return
		}

		// Assign default global advertiser role.
		if h.db != nil {
			var roleID string
			if err := h.db.QueryRowContext(r.Context(), `SELECT id FROM roles WHERE name = 'advertiser' LIMIT 1`).Scan(&roleID); err != nil {
				_, _ = h.db.ExecContext(r.Context(), `DELETE FROM users WHERE id = $1`, u.ID)
				writeJSONError(w, http.StatusInternalServerError, "google_auth_failed", "Failed to login")
				return
			}
			if _, err := h.db.ExecContext(r.Context(), `INSERT INTO user_roles (user_id, role_id, advertiser_id) VALUES ($1, $2, NULL) ON CONFLICT DO NOTHING`, u.ID, roleID); err != nil {
				_, _ = h.db.ExecContext(r.Context(), `DELETE FROM users WHERE id = $1`, u.ID)
				writeJSONError(w, http.StatusInternalServerError, "google_auth_failed", "Failed to login")
				return
			}
		}

		if h.mailer != nil {
			subject := "Welcome to SCM Ads"
			dashboardURL := strings.TrimSpace(h.cfg.DashboardBaseURL)
			resetURL := strings.TrimSpace(h.cfg.AuthResetPasswordURL)
			body := buildWelcomeEmailBody(u.Name, dashboardURL, "", &tmp, resetURL, true)
			if err := h.mailer.Send(u.Email, subject, body); err != nil {
				log.Printf("google-auth: failed to send temp password email to %s: %v", u.Email, err)
			}
		}
	}

	if strings.TrimSpace(strings.ToLower(u.Status)) != "active" {
		writeJSONError(w, http.StatusForbidden, "account_pending", "Your account is pending approval. We will activate it soon.")
		return
	}

	h.writeLoginResponse(w, r, u)
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
		Status:       "pending",
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
	body := buildWelcomeEmailBody(req.Name, dashboardURL, req.UserName, nil, "", false)
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
		log.Printf("login: identifier not found: %q (%v)", strings.TrimSpace(req.Identifier), err)
		if h.cfg.AuthVerboseErrors {
			writeJSONError(w, http.StatusUnauthorized, "invalid_identifier", "Email/username/phone not found")
			return
		}
		writeJSONError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid credentials")
		return
	}

	if strings.TrimSpace(strings.ToLower(u.Status)) != "active" {
		writeJSONError(w, http.StatusForbidden, "account_pending", "Your account is pending approval. We will activate it soon.")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		log.Printf("login: invalid password for user_id=%s identifier=%q", u.ID, strings.TrimSpace(req.Identifier))
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
			SELECT ro.name
			FROM user_roles ur
			JOIN roles ro ON ro.id = ur.role_id
			WHERE ur.user_id = $1
			ORDER BY ro.name ASC
		`, u.ID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "login_failed", "Failed to login")
			return
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				writeJSONError(w, http.StatusInternalServerError, "login_failed", "Failed to login")
				return
			}
			roles = append(roles, models.LoginRole{Name: name})
			break
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

	role := ""
	if len(roles) > 0 {
		role = roles[0].Name
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
		Role:        role,
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
