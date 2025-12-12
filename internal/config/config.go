// internal/config/config.go
package config

import (
	"net/url"
	"os"
)

type Config struct {
	Port        string
	Environment string
	DatabaseURL string
}

func Load() *Config {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		host := getEnv("PSQL_HOST", "localhost")
		port := getEnv("PSQL_PORT", "5432")
		user := getEnv("PSQL_USER", "postgres")
		password := getEnv("PSQL_PASSWORD", "Asterisk@123")
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
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),
		DatabaseURL: databaseURL,
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}