// internal/db/migrations/migrations.go
package migrations

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const migrationsDir = "migrations"

type Migration struct {
	Version int
	Name    string
	Up      string
	Down    string
}

func RunMigrations(db *sql.DB) error {
	// Create migrations table if it doesn't exist
	if err := createMigrationsTable(db); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	applied, err := getAppliedMigrations(db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Get migration files
	files, err := getMigrationFiles()
	if err != nil {
		return fmt.Errorf("failed to get migration files: %w", err)
	}

	// Apply pending migrations
	for _, file := range files {
		if _, exists := applied[file.Version]; !exists {
			if err := applyMigration(db, file); err != nil {
				return fmt.Errorf("failed to apply migration %s: %w", file.Name, err)
			}
			log.Printf("Applied migration: %s", file.Name)
		}
	}

	return nil
}

func createMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func getAppliedMigrations(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}
	return applied, rows.Err()
}

func getMigrationFiles() ([]Migration, error) {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return nil, err
	}

	var migrations []Migration
	for _, file := range files {
		version, name, err := parseMigrationFilename(filepath.Base(file))
		if err != nil {
			return nil, err
		}

		content, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, err
		}

		// Find the down migration
		downContent := []byte{}
		downFile := filepath.Join(migrationsDir, fmt.Sprintf("%04d_%s.down.sql", version, name))
		if _, err := os.Stat(downFile); err == nil {
			downContent, err = ioutil.ReadFile(downFile)
			if err != nil {
				return nil, err
			}
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    name,
			Up:      string(content),
			Down:    string(downContent),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func parseMigrationFilename(filename string) (int, string, error) {
	// Expected format: 0001_name.up.sql
	base := filepath.Base(filename)
	parts := strings.SplitN(base, "_", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid migration filename format: %s", filename)
	}

	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", fmt.Errorf("invalid version in filename %s: %w", filename, err)
	}

	name := strings.TrimSuffix(parts[1], ".up.sql")
	name = strings.TrimSuffix(name, ".down.sql")

	return version, name, nil
}

func applyMigration(db *sql.DB, migration Migration) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Execute migration
	if _, err := tx.Exec(migration.Up); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	// Record migration
	if _, err := tx.Exec(
		"INSERT INTO schema_migrations (version, name) VALUES ($1, $2) ON CONFLICT (version) DO NOTHING",
		migration.Version,
		migration.Name,
	); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return tx.Commit()
}