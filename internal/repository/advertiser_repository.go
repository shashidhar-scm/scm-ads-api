package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"scm/internal/interfaces"
	"scm/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type advertiserRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewAdvertiserRepository(pool *pgxpool.Pool) interfaces.AdvertiserRepository {
	return &advertiserRepository{pool: pool, queryTimeout: 5 * time.Second}
}

func (r *advertiserRepository) ensurePool() error {
	if r.pool == nil {
		return errors.New("advertiser repository: pgx pool is nil")
	}
	return nil
}

func (r *advertiserRepository) translateNoRows(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return sql.ErrNoRows
	}
	return err
}

func (r *advertiserRepository) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, r.queryTimeout)
}

func (r *advertiserRepository) Create(ctx context.Context, advertiser *models.Advertiser) error {
	if err := r.ensurePool(); err != nil {
		return err
	}

	query := `
		INSERT INTO advertisers (name, email, created_by)
		VALUES ($1, $2, $3)
		RETURNING id, created_by, created_at, updated_at
	`

	var createdBy sql.NullString

	execCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	err := r.pool.QueryRow(
		execCtx,
		query,
		advertiser.Name,
		advertiser.Email,
		advertiser.CreatedBy,
	).Scan(
		&advertiser.ID,
		&createdBy,
		&advertiser.CreatedAt,
		&advertiser.UpdatedAt,
	)
	if createdBy.Valid {
		advertiser.CreatedBy = createdBy.String
	} else {
		advertiser.CreatedBy = ""
	}

	if err != nil {
		log.Printf("Error creating advertiser: %v", err)
		return fmt.Errorf("failed to create advertiser: %w", err)
	}

	return nil
}

func (r *advertiserRepository) GetByID(ctx context.Context, id string) (*models.Advertiser, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, email, created_by, created_at, updated_at
		FROM advertisers
		WHERE id = $1
	`

	rowCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var advertiser models.Advertiser
	var createdBy sql.NullString
	err := r.pool.QueryRow(rowCtx, query, id).Scan(
		&advertiser.ID,
		&advertiser.Name,
		&advertiser.Email,
		&createdBy,
		&advertiser.CreatedAt,
		&advertiser.UpdatedAt,
	)
	if createdBy.Valid {
		advertiser.CreatedBy = createdBy.String
	} else {
		advertiser.CreatedBy = ""
	}

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		log.Printf("Error getting advertiser: %v", err)
		return nil, fmt.Errorf("failed to get advertiser: %w", err)
	}

	return &advertiser, nil
}

func (r *advertiserRepository) GetByIDInSet(ctx context.Context, id string, allowedIDs []string) (*models.Advertiser, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, email, created_by, created_at, updated_at
		FROM advertisers
		WHERE id = $1
		  AND id = ANY($2::uuid[])
	`

	rowCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var advertiser models.Advertiser
	var createdBy sql.NullString
	if err := r.pool.QueryRow(rowCtx, query, id, allowedIDs).Scan(
		&advertiser.ID,
		&advertiser.Name,
		&advertiser.Email,
		&createdBy,
		&advertiser.CreatedAt,
		&advertiser.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if createdBy.Valid {
		advertiser.CreatedBy = createdBy.String
	} else {
		advertiser.CreatedBy = ""
	}
	return &advertiser, nil
}

func (r *advertiserRepository) List(ctx context.Context, limit int, offset int) ([]models.Advertiser, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, email, created_by, created_at, updated_at
		FROM advertisers
		ORDER BY name
	`

	args := make([]any, 0, 2)
	argPos := 1
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, limit)
		argPos++
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, offset)
	}

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.pool.Query(queryCtx, query, args...)
	if err != nil {
		log.Printf("Error listing advertisers: %v", err)
		return nil, fmt.Errorf("failed to list advertisers: %w", err)
	}
	defer rows.Close()

	var advertisers []models.Advertiser
	for rows.Next() {
		var adv models.Advertiser
		var createdBy sql.NullString
		if err := rows.Scan(
			&adv.ID,
			&adv.Name,
			&adv.Email,
			&createdBy,
			&adv.CreatedAt,
			&adv.UpdatedAt,
		); err != nil {
			log.Printf("Error scanning advertiser: %v", err)
			return nil, fmt.Errorf("failed to scan advertiser: %w", err)
		}
		if createdBy.Valid {
			adv.CreatedBy = createdBy.String
		} else {
			adv.CreatedBy = ""
		}
		advertisers = append(advertisers, adv)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating advertisers: %v", err)
		return nil, fmt.Errorf("error iterating advertisers: %w", err)
	}

	return advertisers, nil
}

func (r *advertiserRepository) ListByIDs(ctx context.Context, allowedIDs []string, limit int, offset int) ([]models.Advertiser, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, email, created_by, created_at, updated_at
		FROM advertisers
		WHERE id = ANY($1::uuid[])
		ORDER BY name
	`
	args := []any{allowedIDs}
	argPos := 2
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, limit)
		argPos++
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, offset)
	}

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.pool.Query(queryCtx, query, args...)
	if err != nil {
		log.Printf("Error listing scoped advertisers: %v", err)
		return nil, fmt.Errorf("failed to list advertisers: %w", err)
	}
	defer rows.Close()

	var advertisers []models.Advertiser
	for rows.Next() {
		var adv models.Advertiser
		var createdBy sql.NullString
		if err := rows.Scan(
			&adv.ID,
			&adv.Name,
			&adv.Email,
			&createdBy,
			&adv.CreatedAt,
			&adv.UpdatedAt,
		); err != nil {
			log.Printf("Error scanning advertiser: %v", err)
			return nil, fmt.Errorf("failed to scan advertiser: %w", err)
		}
		if createdBy.Valid {
			adv.CreatedBy = createdBy.String
		} else {
			adv.CreatedBy = ""
		}
		advertisers = append(advertisers, adv)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Error iterating advertisers: %v", err)
		return nil, fmt.Errorf("error iterating advertisers: %w", err)
	}
	return advertisers, nil
}

func (r *advertiserRepository) Count(ctx context.Context) (int, error) {
	if err := r.ensurePool(); err != nil {
		return 0, err
	}

	query := `SELECT COUNT(*) FROM advertisers`

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var total int
	if err := r.pool.QueryRow(queryCtx, query).Scan(&total); err != nil {
		log.Printf("Error counting advertisers: %v", err)
		return 0, fmt.Errorf("failed to count advertisers: %w", err)
	}
	return total, nil
}

func (r *advertiserRepository) CountByIDs(ctx context.Context, allowedIDs []string) (int, error) {
	if err := r.ensurePool(); err != nil {
		return 0, err
	}

	query := `SELECT COUNT(*) FROM advertisers WHERE id = ANY($1::uuid[])`

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var total int
	if err := r.pool.QueryRow(queryCtx, query, allowedIDs).Scan(&total); err != nil {
		log.Printf("Error counting advertisers: %v", err)
		return 0, fmt.Errorf("failed to count advertisers: %w", err)
	}
	return total, nil
}

func (r *advertiserRepository) Search(ctx context.Context, term string, limit int, offset int) ([]models.Advertiser, int, error) {
	if err := r.ensurePool(); err != nil {
		return nil, 0, err
	}

	likeTerm := fmt.Sprintf("%%%s%%", strings.ToLower(term))

	countQuery := `
		SELECT COUNT(*)
		FROM advertisers
		WHERE LOWER(name) LIKE $1
			OR LOWER(email) LIKE $1
	`

	countCtx, cancelCount := r.withTimeout(ctx)
	defer cancelCount()

	var total int
	if err := r.pool.QueryRow(countCtx, countQuery, likeTerm).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count advertisers: %w", err)
	}

	query := `
		SELECT id, name, email, created_by, created_at, updated_at
		FROM advertisers
		WHERE LOWER(name) LIKE $1
			OR LOWER(email) LIKE $1
		ORDER BY name
	`

	args := []any{likeTerm}
	argPos := 2
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, limit)
		argPos++
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, offset)
	}

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.pool.Query(queryCtx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search advertisers: %w", err)
	}
	defer rows.Close()

	var advertisers []models.Advertiser
	for rows.Next() {
		var adv models.Advertiser
		var createdBy sql.NullString
		if err := rows.Scan(
			&adv.ID,
			&adv.Name,
			&adv.Email,
			&createdBy,
			&adv.CreatedAt,
			&adv.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan advertiser: %w", err)
		}
		if createdBy.Valid {
			adv.CreatedBy = createdBy.String
		}
		advertisers = append(advertisers, adv)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating advertisers: %w", err)
	}

	return advertisers, total, nil
}

func (r *advertiserRepository) SearchByIDs(ctx context.Context, allowedIDs []string, term string, limit int, offset int) ([]models.Advertiser, int, error) {
	if err := r.ensurePool(); err != nil {
		return nil, 0, err
	}

	likeTerm := fmt.Sprintf("%%%s%%", strings.ToLower(term))

	countQuery := `
		SELECT COUNT(*)
		FROM advertisers
		WHERE id = ANY($1::uuid[])
		  AND (LOWER(name) LIKE $2 OR LOWER(email) LIKE $2)
	`

	countCtx, cancelCount := r.withTimeout(ctx)
	defer cancelCount()

	var total int
	if err := r.pool.QueryRow(countCtx, countQuery, allowedIDs, likeTerm).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count advertisers: %w", err)
	}

	query := `
		SELECT id, name, email, created_by, created_at, updated_at
		FROM advertisers
		WHERE id = ANY($1::uuid[])
		  AND (LOWER(name) LIKE $2 OR LOWER(email) LIKE $2)
		ORDER BY name
	`
	args := []any{allowedIDs, likeTerm}
	argPos := 3
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, limit)
		argPos++
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, offset)
	}

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.pool.Query(queryCtx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search advertisers: %w", err)
	}
	defer rows.Close()

	var advertisers []models.Advertiser
	for rows.Next() {
		var adv models.Advertiser
		var createdBy sql.NullString
		if err := rows.Scan(
			&adv.ID,
			&adv.Name,
			&adv.Email,
			&createdBy,
			&adv.CreatedAt,
			&adv.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan advertiser: %w", err)
		}
		if createdBy.Valid {
			adv.CreatedBy = createdBy.String
		}
		advertisers = append(advertisers, adv)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating advertisers: %w", err)
	}
	return advertisers, total, nil
}

func (r *advertiserRepository) Update(ctx context.Context, id string, req *models.UpdateAdvertiserRequest) error {
	if err := r.ensurePool(); err != nil {
		return err
	}

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

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	result, err := r.pool.Exec(queryCtx, query, args...)
	if err != nil {
		log.Printf("Error updating advertiser: %v", err)
		return fmt.Errorf("failed to update advertiser: %w", err)
	}

	rowsAffected := result.RowsAffected()

	if rowsAffected == 0 {
		return fmt.Errorf("advertiser not found")
	}

	return nil
}

func (r *advertiserRepository) Delete(ctx context.Context, id string) error {
	if err := r.ensurePool(); err != nil {
		return err
	}

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var campaignCount int64
	if err := r.pool.QueryRow(queryCtx, `SELECT COUNT(*) FROM campaigns WHERE advertiser_id = $1`, id).Scan(&campaignCount); err != nil {
		log.Printf("Error checking advertiser references: %v", err)
		return fmt.Errorf("failed to delete advertiser: %w", err)
	}
	if campaignCount > 0 {
		return &interfaces.DeletionBlockedError{
			Resource: "advertiser",
			References: map[string]int64{
				"campaigns": campaignCount,
			},
		}
	}

	query := `DELETE FROM advertisers WHERE id = $1`

	result, err := r.pool.Exec(queryCtx, query, id)
	if err != nil {
		log.Printf("Error deleting advertiser: %v", err)
		return fmt.Errorf("failed to delete advertiser: %w", err)
	}

	rowsAffected := result.RowsAffected()

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}
