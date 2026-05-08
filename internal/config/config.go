// internal/config/config.go
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                   string
	Environment            string
	DatabaseURL            string
	ReplicatorDatabaseURL  string
	LogLevel               string
	AuthVerboseErrors      bool
	AuthReturnResetToken   bool
	AuthResetPasswordURL   string
	DashboardBaseURL       string
	RateLimitWindowSeconds int
	RateLimitMax           int

	VenuesCacheTTLSeconds          int
	SuggestVenuesWorkers           int
	RekognitionTimeoutMs           int
	EnableRekognitionLabelFallback bool

	GoogleClientID string

	CityPostConsoleBaseURL    string
	CityPostConsoleUsername   string
	CityPostConsolePassword   string
	CityPostConsoleAuthScheme string
	PopAPIBaseURL             string

	JWTSecret           string
	JWTExpiresInSeconds int64

	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string
	SMTPUseTLS   bool
}

func Load() (*Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	replicatorDatabaseURL := strings.TrimSpace(os.Getenv("REPLICATOR_DATABASE_URL"))
	if replicatorDatabaseURL == "" {
		replicatorDatabaseURL = "postgres://postgres:asterisk@localhost:5432/scm?sslmode=disable"
	}
	if databaseURL == "" {
		host := getEnv("PSQL_HOST", "localhost")
		port := getEnv("PSQL_PORT", "5432")
		user := getEnv("PSQL_USER", "postgres")
		password := getEnv("PSQL_PASSWORD", "postgres")
		dbName := getEnv("PSQL_DB_NAME", "scm_ads")

		u := &url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(user, password),
			Host:   host + ":" + port,
			Path:   dbName,
		}
		q := u.Query()
		q.Set("sslmode", "disable")
		u.RawQuery = q.Encode()
		databaseURL = u.String()
	}

	cfg := &Config{
		Port:                   getEnv("PORT", "9000"),
		Environment:            getEnv("ENVIRONMENT", "development"),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
		DatabaseURL:            databaseURL,
		ReplicatorDatabaseURL:  replicatorDatabaseURL,
		AuthVerboseErrors:      getEnvBool("AUTH_VERBOSE_ERRORS", false),
		AuthReturnResetToken:   getEnvBool("AUTH_RETURN_RESET_TOKEN", false),
		RateLimitWindowSeconds: getEnvInt("RATE_LIMIT_WINDOW_SECONDS", 60),
		RateLimitMax:           getEnvInt("RATE_LIMIT_MAX", 120),

		VenuesCacheTTLSeconds:          getEnvInt("VENUES_CACHE_TTL_SECONDS", 300),
		SuggestVenuesWorkers:           getEnvInt("SUGGEST_VENUES_WORKERS", 4),
		RekognitionTimeoutMs:           getEnvInt("REKOGNITION_TIMEOUT_MS", 7000),
		EnableRekognitionLabelFallback: getEnvBool("ENABLE_REKOGNITION_LABEL_FALLBACK", false),

		JWTSecret:            getEnv("JWT_SECRET", ""),
		JWTExpiresInSeconds:  getEnvInt64("JWT_EXPIRES_IN_SECONDS", 86400),
		AuthResetPasswordURL: getEnv("AUTH_RESET_PASSWORD_URL", "https://scm-ads.citypost.us/reset-password"),
		DashboardBaseURL:     getEnv("DASHBOARD_BASE_URL", "https://scm-ads.citypost.us"),
		GoogleClientID:       getEnv("GOOGLE_CLIENT_ID", ""),

		CityPostConsoleBaseURL:    getEnv("CITYPOST_CONSOLE_BASE_URL", ""),
		CityPostConsoleUsername:   getEnv("CITYPOST_CONSOLE_USERNAME", ""),
		CityPostConsolePassword:   getEnv("CITYPOST_CONSOLE_PASSWORD", ""),
		CityPostConsoleAuthScheme: getEnv("CITYPOST_CONSOLE_AUTH_SCHEME", "Token"),
		PopAPIBaseURL:             getEnv("POP_API_BASE_URL", "https://pop-api.citypost.us"),

		SMTPHost:     getEnv("SMTP_HOST", ""),
		SMTPPort:     getEnv("SMTP_PORT", ""),
		SMTPUser:     getEnv("SMTP_USER", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", ""),
		SMTPUseTLS:   getEnvBool("SMTP_USE_TLS", true),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	missing := make([]string, 0)

	if strings.TrimSpace(c.JWTSecret) == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if strings.TrimSpace(c.SMTPHost) == "" {
		missing = append(missing, "SMTP_HOST")
	}
	if strings.TrimSpace(c.SMTPPort) == "" {
		missing = append(missing, "SMTP_PORT")
	}
	if strings.TrimSpace(c.SMTPUser) == "" {
		missing = append(missing, "SMTP_USER")
	}
	if strings.TrimSpace(c.SMTPPassword) == "" {
		missing = append(missing, "SMTP_PASSWORD")
	}
	if strings.TrimSpace(c.SMTPFrom) == "" {
		missing = append(missing, "SMTP_FROM")
	}
	if strings.TrimSpace(c.CityPostConsoleBaseURL) == "" {
		missing = append(missing, "CITYPOST_CONSOLE_BASE_URL")
	}
	if strings.TrimSpace(c.CityPostConsoleUsername) == "" {
		missing = append(missing, "CITYPOST_CONSOLE_USERNAME")
	}
	if strings.TrimSpace(c.CityPostConsolePassword) == "" {
		missing = append(missing, "CITYPOST_CONSOLE_PASSWORD")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	if c.Environment != "development" && c.JWTSecret == "dev-secret" {
		return fmt.Errorf("JWT_SECRET must not use the insecure default in %s environment", c.Environment)
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	value := getEnv(key, "")
	if value == "" {
		return defaultValue
	}
	i, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return defaultValue
	}
	return i
}

func getEnvInt(key string, defaultValue int) int {
	value := strings.TrimSpace(getEnv(key, ""))
	if value == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return i
}

func getEnvBool(key string, defaultValue bool) bool {
	value := strings.TrimSpace(getEnv(key, ""))
	if value == "" {
		return defaultValue
	}
	b, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return b
}
