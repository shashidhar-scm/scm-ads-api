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
	ListAll(ctx context.Context, limit int, offset int, createdByUserID *string) ([]*models.Creative, error)
	CountAll(ctx context.Context, createdByUserID *string) (int, error)
	Search(ctx context.Context, term string, limit int, offset int, createdByUserID *string) ([]*models.Creative, int, error)
	ListByCampaign(ctx context.Context, campaignID string, limit int, offset int) ([]*models.Creative, error)
	CountByCampaign(ctx context.Context, campaignID string) (int, error)
	ListByDevice(ctx context.Context, device string, activeNow bool, now time.Time, limit int, offset int) ([]*models.Creative, error)
	CountByDevice(ctx context.Context, device string, activeNow bool, now time.Time) (int, error)
	Update(ctx context.Context, id string, req *models.UpdateCreativeRequest) error
	Delete(ctx context.Context, id string) error
	EnsureRotationGroup(ctx context.Context, campaignID string, name string, selectedDays []string, timeSlots []string) (string, error)
	PickNextRotationalCreative(ctx context.Context, device string, campaignID string, rotationGroupID string, candidateCreativeIDs []string) (string, error)
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
            id, name, type, url, file_path, size, impression_count, impressions_served, rotation_group_id, campaign_id, selected_days, time_slots, devices, uploaded_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
        RETURNING uploaded_at
    `
    
    var rotationGroupID any
    if creative.RotationGroupID != nil && strings.TrimSpace(*creative.RotationGroupID) != "" {
        rotationGroupID = *creative.RotationGroupID
    }

    err := r.db.QueryRowContext(
        ctx,
        query,
        creative.ID,
        creative.Name,
        creative.Type,
        creative.URL,
        creative.FilePath,
        creative.Size,
        creative.ImpressionCount,
        creative.ImpressionsServed,
        rotationGroupID,
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
        SELECT id, name, type, url, file_path, size, impression_count, impressions_served, rotation_group_id, campaign_id, selected_days, time_slots, devices, uploaded_at
        FROM creatives
        WHERE id = $1
    `
    
    var creative models.Creative
    var impressionCount sql.NullInt64
    var rotationGroupID sql.NullString
    err := r.db.QueryRowContext(ctx, query, id).Scan(
        &creative.ID,
        &creative.Name,
        &creative.Type,
        &creative.URL,
        &creative.FilePath,
        &creative.Size,
        &impressionCount,
        &creative.ImpressionsServed,
        &rotationGroupID,
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
    
    if impressionCount.Valid {
        v := impressionCount.Int64
        creative.ImpressionCount = &v
    }
	if rotationGroupID.Valid {
		v := rotationGroupID.String
		creative.RotationGroupID = &v
	}
    return &creative, nil
}


func (r *creativeRepository) ListAll(ctx context.Context, limit int, offset int, createdByUserID *string) ([]*models.Creative, error) {
	query := `
		SELECT
			cr.id, cr.name, cr.type, cr.url, cr.file_path, cr.size, cr.impression_count, cr.impressions_served,
			cr.rotation_group_id, cr.campaign_id, cr.selected_days, cr.time_slots, cr.devices, cr.uploaded_at
		FROM creatives cr
		JOIN campaigns ca ON ca.id = cr.campaign_id
		JOIN advertisers a ON a.id = ca.advertiser_id
		WHERE 1=1
	`

	args := make([]any, 0, 3)
	argPos := 1
	if createdByUserID != nil && strings.TrimSpace(*createdByUserID) != "" {
		query += fmt.Sprintf(" AND a.created_by = $%d", argPos)
		args = append(args, strings.TrimSpace(*createdByUserID))
		argPos++
	}

	query += " ORDER BY cr.uploaded_at DESC"
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
        var impressionCount sql.NullInt64
        var rotationGroupID sql.NullString
        if err := rows.Scan(
            &creative.ID,
            &creative.Name,
            &creative.Type,
            &creative.URL,
            &creative.FilePath,
            &creative.Size,
            &impressionCount,
            &creative.ImpressionsServed,
            &rotationGroupID,
            &creative.CampaignID,
            pq.Array(&creative.SelectedDays),
            pq.Array(&creative.TimeSlots),
            pq.Array(&creative.Devices),
            &creative.UploadedAt,
        ); err != nil {
            return nil, err
        }
        if impressionCount.Valid {
            v := impressionCount.Int64
            creative.ImpressionCount = &v
        }
        if rotationGroupID.Valid {
            v := rotationGroupID.String
            creative.RotationGroupID = &v
        }
        creatives = append(creatives, &creative)
    }

    	return creatives, rows.Err()
}

func (r *creativeRepository) CountAll(ctx context.Context, createdByUserID *string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM creatives cr
		JOIN campaigns ca ON ca.id = cr.campaign_id
		JOIN advertisers a ON a.id = ca.advertiser_id
		WHERE 1=1
	`
	args := make([]any, 0, 1)
	if createdByUserID != nil && strings.TrimSpace(*createdByUserID) != "" {
		query += " AND a.created_by = $1"
		args = append(args, strings.TrimSpace(*createdByUserID))
	}
	var total int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *creativeRepository) Search(ctx context.Context, term string, limit int, offset int, createdByUserID *string) ([]*models.Creative, int, error) {
	likeTerm := fmt.Sprintf("%%%s%%", strings.ToLower(term))

	countQuery := `
		SELECT COUNT(*)
		FROM creatives cr
		JOIN campaigns ca ON ca.id = cr.campaign_id
		JOIN advertisers a ON a.id = ca.advertiser_id
		WHERE (
			LOWER(cr.name) LIKE $1
			OR LOWER(cr.type::text) LIKE $1
			OR LOWER(COALESCE(array_to_string(cr.selected_days, ' '), '')) LIKE $1
			OR LOWER(COALESCE(array_to_string(cr.time_slots, ' '), '')) LIKE $1
			OR LOWER(COALESCE(array_to_string(cr.devices, ' '), '')) LIKE $1
		)
	`
	args := []any{likeTerm}
	if createdByUserID != nil && strings.TrimSpace(*createdByUserID) != "" {
		countQuery += " AND a.created_by = $2"
		args = append(args, strings.TrimSpace(*createdByUserID))
	}
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("search creatives count: %w", err)
	}

	query := `
		SELECT
			cr.id, cr.name, cr.type, cr.url, cr.file_path, cr.size, cr.impression_count, cr.impressions_served,
			cr.rotation_group_id, cr.campaign_id, cr.selected_days, cr.time_slots, cr.devices, cr.uploaded_at
		FROM creatives cr
		JOIN campaigns ca ON ca.id = cr.campaign_id
		JOIN advertisers a ON a.id = ca.advertiser_id
		WHERE (
			LOWER(cr.name) LIKE $1
			OR LOWER(cr.type::text) LIKE $1
			OR EXISTS (SELECT 1 FROM unnest(cr.selected_days) d WHERE LOWER(d) LIKE $1)
			OR EXISTS (SELECT 1 FROM unnest(cr.time_slots) t WHERE LOWER(t) LIKE $1)
			OR EXISTS (SELECT 1 FROM unnest(cr.devices) dv WHERE LOWER(dv) LIKE $1)
		)
	`
	qArgs := []any{likeTerm}
	if createdByUserID != nil && strings.TrimSpace(*createdByUserID) != "" {
		query += " AND a.created_by = $2"
		qArgs = append(qArgs, strings.TrimSpace(*createdByUserID))
	}
	query += " ORDER BY cr.uploaded_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(qArgs)+1, len(qArgs)+2)
	qArgs = append(qArgs, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, qArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("search creatives: %w", err)
	}
	defer rows.Close()

	var creatives []*models.Creative
	for rows.Next() {
		var creative models.Creative
		var impressionCount sql.NullInt64
		var rotationGroupID sql.NullString
		if err := rows.Scan(
			&creative.ID,
			&creative.Name,
			&creative.Type,
			&creative.URL,
			&creative.FilePath,
			&creative.Size,
			&impressionCount,
			&creative.ImpressionsServed,
			&rotationGroupID,
			&creative.CampaignID,
			pq.Array(&creative.SelectedDays),
			pq.Array(&creative.TimeSlots),
			pq.Array(&creative.Devices),
			&creative.UploadedAt,
		); err != nil {
			return nil, 0, err
		}
		if impressionCount.Valid {
			v := impressionCount.Int64
			creative.ImpressionCount = &v
		}
		if rotationGroupID.Valid {
			v := rotationGroupID.String
			creative.RotationGroupID = &v
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
			id, name, type, url, file_path, size, impression_count, impressions_served, rotation_group_id, campaign_id, selected_days, time_slots, devices, uploaded_at
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
        var impressionCount sql.NullInt64
        if err := rows.Scan(
            &creative.ID,
            &creative.Name,
            &creative.Type,
            &creative.URL,
            &creative.FilePath,
            &creative.Size,
            &impressionCount,
            &creative.ImpressionsServed,
            &creative.RotationGroupID,
            &creative.CampaignID,
            pq.Array(&creative.SelectedDays),
            pq.Array(&creative.TimeSlots),
            pq.Array(&creative.Devices),
            &creative.UploadedAt,
        ); err != nil {
            return nil, err
        }
        if impressionCount.Valid {
            v := impressionCount.Int64
            creative.ImpressionCount = &v
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
			cr.id, cr.name, cr.type, cr.url, cr.file_path, cr.size, cr.impression_count, cr.impressions_served, cr.rotation_group_id, cr.campaign_id, cr.selected_days, cr.time_slots, cr.devices, cr.uploaded_at
		FROM creatives cr
		JOIN campaigns ca ON ca.id = cr.campaign_id
		WHERE EXISTS (
			SELECT 1
			FROM unnest(cr.devices) dv
			WHERE lower(trim(dv)) = lower($1)
		)
		AND ca.status = 'active'
		AND (
			ca.impressions_based = false
			OR (
				ca.impressions_based = true
				AND cr.impression_count IS NOT NULL
				AND cr.impression_count > COALESCE(cr.impressions_served, 0)
			)
		)
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
		var impressionCount sql.NullInt64
		var rotationGroupID sql.NullString
		if err := rows.Scan(
			&creative.ID,
			&creative.Name,
			&creative.Type,
			&creative.URL,
			&creative.FilePath,
			&creative.Size,
			&impressionCount,
			&creative.ImpressionsServed,
			&rotationGroupID,
			&creative.CampaignID,
			pq.Array(&creative.SelectedDays),
			pq.Array(&creative.TimeSlots),
			pq.Array(&creative.Devices),
			&creative.UploadedAt,
		); err != nil {
			return nil, err
		}
		if impressionCount.Valid {
			v := impressionCount.Int64
			creative.ImpressionCount = &v
		}
		if rotationGroupID.Valid {
			v := rotationGroupID.String
			creative.RotationGroupID = &v
		}
		creatives = append(creatives, &creative)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return creatives, nil
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
		AND (
			ca.impressions_based = false
			OR (
				ca.impressions_based = true
				AND cr.impression_count IS NOT NULL
				AND cr.impression_count > COALESCE(cr.impressions_served, 0)
			)
		)
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
			impression_count = COALESCE($6, impression_count),
			rotation_group_id = CASE
				WHEN $7::boolean = true THEN NULL
				ELSE COALESCE($8::uuid, rotation_group_id)
			END,
			selected_days = COALESCE($9::text[], selected_days),
			time_slots = COALESCE($10::text[], time_slots),
			devices = COALESCE($11::text[], devices)
		WHERE id = $12
		RETURNING id
	`

	clearRotation := req.ClearRotationGroup
	var rotationGroupID any
	if req.RotationGroupID != nil && strings.TrimSpace(*req.RotationGroupID) != "" {
		rotationGroupID = *req.RotationGroupID
	}

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

	if err := r.db.QueryRowContext(
		ctx,
		query,
		req.Name,
		req.Type,
		req.URL,
		req.FilePath,
		req.Size,
		req.ImpressionCount,
		clearRotation,
		rotationGroupID,
		selectedDays,
		timeSlots,
		devices,
		id,
	).Scan(&id); err != nil {
		return err
	}
	return nil
}

func (r *creativeRepository) EnsureRotationGroup(ctx context.Context, campaignID string, name string, selectedDays []string, timeSlots []string) (string, error) {
	if strings.TrimSpace(campaignID) == "" {
		return "", fmt.Errorf("campaignID is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("rotation_group_name is required")
	}
	if len(selectedDays) == 0 {
		return "", fmt.Errorf("selected_days is required for rotation group")
	}
	if len(timeSlots) == 0 {
		return "", fmt.Errorf("time_slots is required for rotation group")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO rotation_groups (campaign_id, name, selected_days, time_slots)
		VALUES ($1, $2, $3::text[], $4::text[])
		ON CONFLICT (campaign_id, name) DO NOTHING
	`, campaignID, name, pq.Array(selectedDays), pq.Array(timeSlots))
	if err != nil {
		return "", err
	}

	var id string
	var existingDays []string
	var existingSlots []string
	if err := tx.QueryRowContext(ctx, `
		SELECT id, selected_days, time_slots
		FROM rotation_groups
		WHERE campaign_id = $1 AND name = $2
	`, campaignID, name).Scan(&id, pq.Array(&existingDays), pq.Array(&existingSlots)); err != nil {
		return "", err
	}

	if !equalStringSlices(existingDays, selectedDays) || !equalStringSlices(existingSlots, timeSlots) {
		return "", fmt.Errorf("rotation_group_name already exists with different selected_days/time_slots")
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

func (r *creativeRepository) PickNextRotationalCreative(ctx context.Context, device string, campaignID string, rotationGroupID string, candidateCreativeIDs []string) (string, error) {
	device = strings.TrimSpace(device)
	campaignID = strings.TrimSpace(campaignID)
	rotationGroupID = strings.TrimSpace(rotationGroupID)
	if device == "" || campaignID == "" || rotationGroupID == "" {
		return "", fmt.Errorf("device, campaignID and rotationGroupID are required")
	}
	if len(candidateCreativeIDs) == 0 {
		return "", fmt.Errorf("candidateCreativeIDs is required")
	}

	// Defensive: ensure deterministic order if caller passes same set in different order.
	// We keep caller order by default but still need stable behavior if duplicates.
	seen := make(map[string]struct{}, len(candidateCreativeIDs))
	unique := make([]string, 0, len(candidateCreativeIDs))
	for _, id := range candidateCreativeIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return "", fmt.Errorf("candidateCreativeIDs is required")
	}
	// Preserve the given order (order matters), but keep stable behavior when DB returns last_id not in list.
	// (No sorting here.)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var last sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT last_creative_id
		FROM creative_rotation_state
		WHERE device = $1 AND campaign_id = $2 AND rotation_group_id = $3
		FOR UPDATE
	`, device, campaignID, rotationGroupID).Scan(&last)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}

	chosen := unique[0]
	if last.Valid {
		idx := -1
		for i, id := range unique {
			if id == last.String {
				idx = i
				break
			}
		}
		if idx >= 0 {
			chosen = unique[(idx+1)%len(unique)]
		}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO creative_rotation_state (device, campaign_id, rotation_group_id, last_creative_id, updated_at)
		VALUES ($1, $2, $3, $4, NOW() AT TIME ZONE 'UTC')
		ON CONFLICT (device, campaign_id, rotation_group_id)
		DO UPDATE SET last_creative_id = EXCLUDED.last_creative_id, updated_at = EXCLUDED.updated_at
	`, device, campaignID, rotationGroupID, chosen)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return chosen, nil
}

func equalStringSlices(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if strings.TrimSpace(a[i]) != strings.TrimSpace(b[i]) {
			return false
		}
	}
	return true
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