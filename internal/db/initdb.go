// internal/db/initdb.go
package db

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"log"
	"net/url"
	"strings"
)

// CreateDatabaseIfNotExists creates the database if it doesn't exist
func CreateDatabaseIfNotExists(connString string) error {
	// Extract database name from connection string
	dbName, err := extractDBName(connString)
	if err != nil {
		return fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Connect to the default 'postgres' database to create our target database
	rootConnStr, err := replaceDBName(connString, "postgres")
	if err != nil {
		return fmt.Errorf("failed to create root connection string: %w", err)
	}

	db, err := sql.Open("postgres", rootConnStr)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}
	defer db.Close()

	// Check if database exists
	var exists bool
	err = db.QueryRow("SELECT 1 FROM pg_database WHERE datname = $1", dbName).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}

	// Create database if it doesn't exist
	if !exists {
		log.Printf("Creating database: %s", dbName)
		// Disable prepared statements for database creation
		_, err = db.Exec("CREATE DATABASE " + dbName)
		if err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
		log.Printf("Database %s created successfully", dbName)
	}

	return nil
}

// extractDBName extracts the database name from a PostgreSQL connection string
func extractDBName(connString string) (string, error) {
	// Try to parse as URL first
	if strings.HasPrefix(connString, "postgres://") || strings.HasPrefix(connString, "postgresql://") {
		u, err := url.Parse(connString)
		if err != nil {
			return "", fmt.Errorf("failed to parse connection URL: %w", err)
		}
		return strings.TrimPrefix(u.Path, "/"), nil
	}

	// Try to parse as key-value pairs
	pairs := strings.Fields(connString)
	for _, pair := range pairs {
		if strings.HasPrefix(pair, "dbname=") {
			return strings.TrimPrefix(pair, "dbname="), nil
		}
	}

	return "", fmt.Errorf("could not find database name in connection string")
}

// replaceDBName replaces the database name in a connection string
func replaceDBName(connString, newName string) (string, error) {
	// Handle URL format
	if strings.HasPrefix(connString, "postgres://") || strings.HasPrefix(connString, "postgresql://") {
		u, err := url.Parse(connString)
		if err != nil {
			return "", err
		}
		u.Path = "/" + newName
		return u.String(), nil
	}

	// Handle key-value pair format
	var result []string
	pairs := strings.Fields(connString)
	for _, pair := range pairs {
		if strings.HasPrefix(pair, "dbname=") {
			result = append(result, "dbname="+newName)
		} else {
			result = append(result, pair)
		}
	}
	return strings.Join(result, " "), nil
}