package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"scm/internal/interfaces"
	"scm/internal/models"
)

type advertiserRepository struct {
	db *sql.DB
}

func NewAdvertiserRepository(db *sql.DB) interfaces.AdvertiserRepository {
	return &advertiserRepository{db: db}
}

func (r *advertiserRepository) Create(ctx context.Context, advertiser *models.Advertiser) error {
	query := `
		INSERT INTO advertisers (name, email)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		advertiser.Name,
		advertiser.Email,
	).Scan(
		&advertiser.ID,
		&advertiser.CreatedAt,
		&advertiser.UpdatedAt,
	)

	if err != nil {
		log.Printf("Error creating advertiser: %v", err)
		return fmt.Errorf("failed to create advertiser: %w", err)
	}

	return nil
}

func (r *advertiserRepository) GetByID(ctx context.Context, id string) (*models.Advertiser, error) {
	query := `
		SELECT id, name, email, created_at, updated_at
		FROM advertisers
		WHERE id = $1
	`

	var advertiser models.Advertiser
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&advertiser.ID,
		&advertiser.Name,
		&advertiser.Email,
		&advertiser.CreatedAt,
		&advertiser.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		log.Printf("Error getting advertiser: %v", err)
		return nil, fmt.Errorf("failed to get advertiser: %w", err)
	}

	return &advertiser, nil
}

func (r *advertiserRepository) List(ctx context.Context) ([]models.Advertiser, error) {
	query := `
		SELECT id, name, email, created_at, updated_at
		FROM advertisers
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("Error listing advertisers: %v", err)
		return nil, fmt.Errorf("failed to list advertisers: %w", err)
	}
	defer rows.Close()

	var advertisers []models.Advertiser
	for rows.Next() {
		var adv models.Advertiser
		if err := rows.Scan(
			&adv.ID,
			&adv.Name,
			&adv.Email,
			&adv.CreatedAt,
			&adv.UpdatedAt,
		); err != nil {
			log.Printf("Error scanning advertiser: %v", err)
			return nil, fmt.Errorf("failed to scan advertiser: %w", err)
		}
		advertisers = append(advertisers, adv)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating advertisers: %v", err)
		return nil, fmt.Errorf("error iterating advertisers: %w", err)
	}

	return advertisers, nil
}

func (r *advertiserRepository) Update(ctx context.Context, id string, req *models.UpdateAdvertiserRequest) error {
	setValues := []string{}
	args := []interface{}{}
	argId := 1

	if req.Name != nil {
		setValues = append(setValues, fmt.Sprintf("name = $%d", argId))
		args = append(args, *req.Name)
		argId++
	}

	if req.Email != nil {
		setValues = append(setValues, fmt.Sprintf("email = $%d", argId))
		args = append(args, *req.Email)
		argId++
	}

	if len(setValues) == 0 {
		return fmt.Errorf("no fields to update")
	}

	// Add updated_at
	setValues = append(setValues, "updated_at = NOW() AT TIME ZONE 'UTC'")

	// Add ID to args
	args = append(args, id)

	query := fmt.Sprintf(
		"UPDATE advertisers SET %s WHERE id = $%d",
		strings.Join(setValues, ", "),
		argId,
	)

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		log.Printf("Error updating advertiser: %v", err)
		return fmt.Errorf("failed to update advertiser: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected: %v", err)
		return fmt.Errorf("failed to update advertiser: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("advertiser not found")
	}

	return nil
}

func (r *advertiserRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM advertisers WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		log.Printf("Error deleting advertiser: %v", err)
		return fmt.Errorf("failed to delete advertiser: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected: %v", err)
		return fmt.Errorf("failed to delete advertiser: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}
