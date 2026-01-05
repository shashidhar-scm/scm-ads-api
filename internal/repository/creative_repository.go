package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"scm/internal/models"
)

type CreativeRepository interface {
	Create(ctx context.Context, creative *models.Creative) error
	GetByID(ctx context.Context, id string) (*models.Creative, error)
	ListAll(ctx context.Context, limit int, offset int) ([]*models.Creative, error)
	CountAll(ctx context.Context) (int, error)
	Search(ctx context.Context, term string, limit int, offset int) ([]*models.Creative, int, error)
	ListByCampaign(ctx context.Context, campaignID string, limit int, offset int) ([]*models.Creative, error)
	CountByCampaign(ctx context.Context, campaignID string) (int, error)
	ListByDevice(ctx context.Context, device string, activeNow bool, now time.Time, limit int, offset int) ([]*models.Creative, error)
	CountByDevice(ctx context.Context, device string, activeNow bool, now time.Time) (int, error)
	Update(ctx context.Context, id string, req *models.UpdateCreativeRequest) error
	Delete(ctx context.Context, id string) error
}

type creativeRepository struct {
    db *sql.DB
}

func NewCreativeRepository(db *sql.DB) CreativeRepository {
    return &creativeRepository{db: db}
}

func (r *creativeRepository) Create(ctx context.Context, creative *models.Creative) error {
    query := `
        INSERT INTO creatives (
            id, name, type, url, file_path, size, campaign_id, selected_days, time_slots, devices, uploaded_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        RETURNING uploaded_at
    `
    
    err := r.db.QueryRowContext(
        ctx,
        query,
        creative.ID,
        creative.Name,
        creative.Type,
        creative.URL,
        creative.FilePath,
        creative.Size,
        creative.CampaignID,
        pq.Array(creative.SelectedDays),
        pq.Array(creative.TimeSlots),
        pq.Array(creative.Devices),
        creative.UploadedAt,
    ).Scan(&creative.UploadedAt)
    
    return err
}

func (r *creativeRepository) GetByID(ctx context.Context, id string) (*models.Creative, error) {
    query := `
        SELECT id, name, type, url, file_path, size, campaign_id, selected_days, time_slots, devices, uploaded_at
        FROM creatives
        WHERE id = $1
    `
    
    var creative models.Creative
    err := r.db.QueryRowContext(ctx, query, id).Scan(
        &creative.ID,
        &creative.Name,
        &creative.Type,
        &creative.URL,
        &creative.FilePath,
        &creative.Size,
        &creative.CampaignID,
        pq.Array(&creative.SelectedDays),
        pq.Array(&creative.TimeSlots),
        pq.Array(&creative.Devices),
        &creative.UploadedAt,
    )
    
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, sql.ErrNoRows
        }
        return nil, err
    }
    
    return &creative, nil
}

func (r *creativeRepository) ListAll(ctx context.Context, limit int, offset int) ([]*models.Creative, error) {
	query := `
		SELECT
			id, name, type, url, file_path, size, campaign_id, selected_days, time_slots, devices, uploaded_at
		FROM creatives
		ORDER BY uploaded_at DESC
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

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

    var creatives []*models.Creative
    for rows.Next() {
        var creative models.Creative
        if err := rows.Scan(
            &creative.ID,
            &creative.Name,
            &creative.Type,
            &creative.URL,
            &creative.FilePath,
            &creative.Size,
            &creative.CampaignID,
            pq.Array(&creative.SelectedDays),
            pq.Array(&creative.TimeSlots),
            pq.Array(&creative.Devices),
            &creative.UploadedAt,
        ); err != nil {
            return nil, err
        }
        creatives = append(creatives, &creative)
    }

    	return creatives, rows.Err()
}

func (r *creativeRepository) CountAll(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM creatives`
	var total int
	if err := r.db.QueryRowContext(ctx, query).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *creativeRepository) Search(ctx context.Context, term string, limit int, offset int) ([]*models.Creative, int, error) {
	likeTerm := fmt.Sprintf("%%%s%%", strings.ToLower(term))

	countQuery := `
		SELECT COUNT(*)
		FROM creatives
		WHERE LOWER(name) LIKE $1
			OR LOWER(type::text) LIKE $1
			OR LOWER(COALESCE(array_to_string(selected_days, ' '), '')) LIKE $1
			OR LOWER(COALESCE(array_to_string(time_slots, ' '), '')) LIKE $1
			OR LOWER(COALESCE(array_to_string(devices, ' '), '')) LIKE $1
	`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, likeTerm).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("search creatives count: %w", err)
	}

	query := `
		SELECT
			id, name, type, url, file_path, size, campaign_id, selected_days, time_slots, devices, uploaded_at
		FROM creatives
		WHERE LOWER(name) LIKE $1
			OR LOWER(type::text) LIKE $1
			OR LOWER(COALESCE(array_to_string(selected_days, ' '), '')) LIKE $1
			OR LOWER(COALESCE(array_to_string(time_slots, ' '), '')) LIKE $1
			OR LOWER(COALESCE(array_to_string(devices, ' '), '')) LIKE $1
		ORDER BY uploaded_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, likeTerm, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("search creatives: %w", err)
	}
	defer rows.Close()

	var creatives []*models.Creative
	for rows.Next() {
		var creative models.Creative
		if err := rows.Scan(
			&creative.ID,
			&creative.Name,
			&creative.Type,
			&creative.URL,
			&creative.FilePath,
			&creative.Size,
			&creative.CampaignID,
			pq.Array(&creative.SelectedDays),
			pq.Array(&creative.TimeSlots),
			pq.Array(&creative.Devices),
			&creative.UploadedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan creative: %w", err)
		}
		creatives = append(creatives, &creative)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate creatives: %w", err)
	}

	return creatives, total, nil
}

func (r *creativeRepository) ListByCampaign(ctx context.Context, campaignID string, limit int, offset int) ([]*models.Creative, error) {
	query := `
		SELECT
			id, name, type, url, file_path, size, campaign_id, selected_days, time_slots, devices, uploaded_at
		FROM creatives
		WHERE campaign_id = $1
		ORDER BY uploaded_at DESC
	`

	args := make([]any, 0, 3)
	args = append(args, campaignID)
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

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
    
    var creatives []*models.Creative
    for rows.Next() {
        var creative models.Creative
        if err := rows.Scan(
            &creative.ID,
            &creative.Name,
            &creative.Type,
            &creative.URL,
            &creative.FilePath,
            &creative.Size,
            &creative.CampaignID,
            pq.Array(&creative.SelectedDays),
            pq.Array(&creative.TimeSlots),
            pq.Array(&creative.Devices),
            &creative.UploadedAt,
        ); err != nil {
            return nil, err
        }
        creatives = append(creatives, &creative)
    }
    
    	return creatives, rows.Err()
}

func (r *creativeRepository) CountByCampaign(ctx context.Context, campaignID string) (int, error) {
	query := `SELECT COUNT(*) FROM creatives WHERE campaign_id = $1`
	var total int
	if err := r.db.QueryRowContext(ctx, query, campaignID).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *creativeRepository) ListByDevice(ctx context.Context, device string, activeNow bool, now time.Time, limit int, offset int) ([]*models.Creative, error) {
	query := `
		SELECT
			cr.id, cr.name, cr.type, cr.url, cr.file_path, cr.size, cr.campaign_id, cr.selected_days, cr.time_slots, cr.devices, cr.uploaded_at
		FROM creatives cr
		JOIN campaigns ca ON ca.id = cr.campaign_id
		WHERE EXISTS (
			SELECT 1
			FROM unnest(cr.devices) dv
			WHERE lower(trim(dv)) = lower($1)
		)
		AND ca.status = 'active'
	`

	args := []any{device}
	argPos := 2

	if activeNow {
		day := now.Weekday().String()
		tm := now.Format("15:04")

		query += `
			AND EXISTS (
				SELECT 1
				FROM unnest(cr.selected_days) d
				WHERE lower(trim(d)) = lower($2)
			)
			AND EXISTS (
				SELECT 1
				FROM unnest(cr.time_slots) ts
				WHERE (
					position('-' in ts) > 0
					AND $3::time >= split_part(ts, '-', 1)::time
					AND $3::time <= split_part(ts, '-', 2)::time
				)
				OR (
					position('-' in ts) = 0
					AND lower(trim(ts)) = lower($4)
				)
			)
		`

		args = append(args, day, tm, tm)
		argPos = 5
	}

	query += `
		ORDER BY uploaded_at DESC
	`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, limit)
		argPos++
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var creatives []*models.Creative
	for rows.Next() {
		var creative models.Creative
		if err := rows.Scan(
			&creative.ID,
			&creative.Name,
			&creative.Type,
			&creative.URL,
			&creative.FilePath,
			&creative.Size,
			&creative.CampaignID,
			pq.Array(&creative.SelectedDays),
			pq.Array(&creative.TimeSlots),
			pq.Array(&creative.Devices),
			&creative.UploadedAt,
		); err != nil {
			return nil, err
		}
		creatives = append(creatives, &creative)
	}

	return creatives, rows.Err()
}

func (r *creativeRepository) CountByDevice(ctx context.Context, device string, activeNow bool, now time.Time) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM creatives cr
		JOIN campaigns ca ON ca.id = cr.campaign_id
		WHERE EXISTS (
			SELECT 1
			FROM unnest(cr.devices) dv
			WHERE lower(trim(dv)) = lower($1)
		)
		AND ca.status = 'active'
	`

	args := []any{device}

	if activeNow {
		day := now.Weekday().String()
		tm := now.Format("15:04")
		query += `
			AND EXISTS (
				SELECT 1
				FROM unnest(cr.selected_days) d
				WHERE lower(trim(d)) = lower($2)
			)
			AND EXISTS (
				SELECT 1
				FROM unnest(cr.time_slots) ts
				WHERE (
					position('-' in ts) > 0
					AND $3::time >= split_part(ts, '-', 1)::time
					AND $3::time <= split_part(ts, '-', 2)::time
				)
				OR (
					position('-' in ts) = 0
					AND lower(trim(ts)) = lower($4)
				)
			)
		`
		args = append(args, day, tm, tm)
	}

	var total int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *creativeRepository) Update(ctx context.Context, id string, req *models.UpdateCreativeRequest) error {
    query := `
        UPDATE creatives
        SET name = COALESCE($1, name),
            type = COALESCE($2, type),
            url = COALESCE($3, url),
            file_path = COALESCE($4, file_path),
            size = COALESCE($5, size),
            selected_days = COALESCE($6::text[], selected_days),
            time_slots = COALESCE($7::text[], time_slots),
            devices = COALESCE($8::text[], devices)
        WHERE id = $9
        RETURNING id
    `

    var selectedDays any
    if req.SelectedDays != nil {
        selectedDays = pq.Array(*req.SelectedDays)
    }
    var timeSlots any
    if req.TimeSlots != nil {
        timeSlots = pq.Array(*req.TimeSlots)
    }
    var devices any
    if req.Devices != nil {
        devices = pq.Array(*req.Devices)
    }

    err := r.db.QueryRowContext(
        ctx,
        query,
        req.Name,
        req.Type,
        req.URL,
        req.FilePath,
        req.Size,
        selectedDays,
        timeSlots,
        devices,
        id,
    ).Scan(&id)
    
    return err
}

func (r *creativeRepository) Delete(ctx context.Context, id string) error {
    query := `DELETE FROM creatives WHERE id = $1`
    result, err := r.db.ExecContext(ctx, query, id)
    if err != nil {
        return err
    }
    
    rowsAffected, err := result.RowsAffected()
    if err != nil {
        return err
    }
    
    if rowsAffected == 0 {
        return sql.ErrNoRows
    }
    
    return nil
}