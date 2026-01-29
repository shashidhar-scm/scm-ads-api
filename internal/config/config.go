// internal/config/config.go
package config

import (
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port        string
	Environment string
	DatabaseURL string
	AuthVerboseErrors bool
	AuthReturnResetToken bool
	AuthResetPasswordURL string
	DashboardBaseURL string
	RateLimitWindowSeconds int
	RateLimitMax           int

	VenuesCacheTTLSeconds            int
	SuggestVenuesWorkers             int
	RekognitionTimeoutMs             int
	EnableRekognitionLabelFallback   bool

	GoogleClientID string

	CityPostConsoleBaseURL   string
	CityPostConsoleUsername  string
	CityPostConsolePassword  string
	CityPostConsoleAuthScheme string
	PopAPIBaseURL            string

	JWTSecret           string
	JWTExpiresInSeconds int64

	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string
	SMTPUseTLS   bool
}

func Load() *Config {
	databaseURL := os.Getenv("DATABASE_URL")
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

	return &Config{
		Port:        getEnv("PORT", "9000"),
		Environment: getEnv("ENVIRONMENT", "development"),
		DatabaseURL: databaseURL,
		AuthVerboseErrors: getEnvBool("AUTH_VERBOSE_ERRORS", false),
		AuthReturnResetToken: getEnvBool("AUTH_RETURN_RESET_TOKEN", false),
		RateLimitWindowSeconds: getEnvInt("RATE_LIMIT_WINDOW_SECONDS", 60),
		RateLimitMax:           getEnvInt("RATE_LIMIT_MAX", 120),

		VenuesCacheTTLSeconds:          getEnvInt("VENUES_CACHE_TTL_SECONDS", 300),
		SuggestVenuesWorkers:           getEnvInt("SUGGEST_VENUES_WORKERS", 4),
		RekognitionTimeoutMs:           getEnvInt("REKOGNITION_TIMEOUT_MS", 7000),
		EnableRekognitionLabelFallback: getEnvBool("ENABLE_REKOGNITION_LABEL_FALLBACK", false),

		JWTSecret:           getEnv("JWT_SECRET", "dev-secret"),
		JWTExpiresInSeconds: getEnvInt64("JWT_EXPIRES_IN_SECONDS", 86400),
		AuthResetPasswordURL: getEnv("AUTH_RESET_PASSWORD_URL", "https://scm-ads.citypost.us/reset-password"),
		DashboardBaseURL:    getEnv("DASHBOARD_BASE_URL", "https://scm-ads.citypost.us"),
		GoogleClientID:      getEnv("GOOGLE_CLIENT_ID", ""),

		CityPostConsoleBaseURL:    getEnv("CITYPOST_CONSOLE_BASE_URL", "https://consoleapi.citypost.us/scm-cloud"),
		CityPostConsoleUsername:   getEnv("CITYPOST_CONSOLE_USERNAME", "girish@smartcitymedia.us"),
		CityPostConsolePassword:   getEnv("CITYPOST_CONSOLE_PASSWORD", "liv3wire"),
		CityPostConsoleAuthScheme: getEnv("CITYPOST_CONSOLE_AUTH_SCHEME", "Token"),
		PopAPIBaseURL:             getEnv("POP_API_BASE_URL", "https://pop-api.citypost.us"),

		SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPUser:     getEnv("SMTP_USER", "citypost@smartcitymedia.us"),
		SMTPPassword: getEnv("SMTP_PASSWORD", "iwud bnba gmpi dbct"),
		SMTPFrom:     getEnv("SMTP_FROM", "citypost@smartcitymedia.us"),
		SMTPUseTLS:   getEnvBool("SMTP_USE_TLS", true),
	}
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