package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
	PickNextRotationalCreative(ctx context.Context, device string, campaignID string, candidateCreativeIDs []string) (string, error)
}

type creativeRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewCreativeRepository(pool *pgxpool.Pool) CreativeRepository {
	return &creativeRepository{pool: pool, queryTimeout: 5 * time.Second}
}

func (r *creativeRepository) ensurePool() error {
	if r.pool == nil {
		return errors.New("creative repository: pgx pool is nil")
	}
	return nil
}

func (r *creativeRepository) translateNoRows(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return sql.ErrNoRows
	}
	return err
}

func (r *creativeRepository) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, r.queryTimeout)
}

func (r *creativeRepository) Create(ctx context.Context, creative *models.Creative) error {
	if err := r.ensurePool(); err != nil {
		return err
	}

	query := `
		INSERT INTO creatives (
			id, name, type, url, file_path, size, impression_count, impressions_served, play_weight, campaign_id, selected_days, time_slots, devices, uploaded_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING uploaded_at
	`

	execCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	err := r.pool.QueryRow(execCtx,
		query,
		creative.ID,
		creative.Name,
		creative.Type,
		creative.URL,
		creative.FilePath,
		creative.Size,
		creative.ImpressionCount,
		creative.ImpressionsServed,
		creative.PlayWeight,
		creative.CampaignID,
		creative.SelectedDays,
		creative.TimeSlots,
		creative.Devices,
		creative.UploadedAt,
	).Scan(&creative.UploadedAt)

	return err
}

func (r *creativeRepository) GetByID(ctx context.Context, id string) (*models.Creative, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, type, url, file_path, size, impression_count, impressions_served, play_weight, campaign_id, selected_days, time_slots, devices, uploaded_at
		FROM creatives
		WHERE id = $1
	`

	rowCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var creative models.Creative
	var impressionCount sql.NullInt64
	err := r.pool.QueryRow(rowCtx, query, id).Scan(
		&creative.ID,
		&creative.Name,
		&creative.Type,
		&creative.URL,
		&creative.FilePath,
		&creative.Size,
		&impressionCount,
		&creative.ImpressionsServed,
		&creative.PlayWeight,
		&creative.CampaignID,
		&creative.SelectedDays,
		&creative.TimeSlots,
		&creative.Devices,
		&creative.UploadedAt,
	)

	if err != nil {
		return nil, r.translateNoRows(err)
	}

	if impressionCount.Valid {
		v := impressionCount.Int64
		creative.ImpressionCount = &v
	}
	return &creative, nil
}

func (r *creativeRepository) ListAll(ctx context.Context, limit int, offset int, createdByUserID *string) ([]*models.Creative, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT
			cr.id, cr.name, cr.type, cr.url, cr.file_path, cr.size, cr.impression_count, cr.impressions_served, cr.play_weight,
			cr.campaign_id, cr.selected_days, cr.time_slots, cr.devices, cr.uploaded_at
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

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.pool.Query(queryCtx, query, args...)
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
			&creative.PlayWeight,
			&creative.CampaignID,
			&creative.SelectedDays,
			&creative.TimeSlots,
			&creative.Devices,
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

func (r *creativeRepository) CountAll(ctx context.Context, createdByUserID *string) (int, error) {
	if err := r.ensurePool(); err != nil {
		return 0, err
	}

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

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var total int
	if err := r.pool.QueryRow(queryCtx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *creativeRepository) Search(ctx context.Context, term string, limit int, offset int, createdByUserID *string) ([]*models.Creative, int, error) {
	if err := r.ensurePool(); err != nil {
		return nil, 0, err
	}

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

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var total int
	if err := r.pool.QueryRow(queryCtx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("search creatives count: %w", err)
	}

	query := `
		SELECT
			cr.id, cr.name, cr.type, cr.url, cr.file_path, cr.size, cr.impression_count, cr.impressions_served, cr.play_weight,
			cr.campaign_id, cr.selected_days, cr.time_slots, cr.devices, cr.uploaded_at
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

	rows, err := r.pool.Query(queryCtx, query, qArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("search creatives: %w", err)
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
			&creative.PlayWeight,
			&creative.CampaignID,
			&creative.SelectedDays,
			&creative.TimeSlots,
			&creative.Devices,
			&creative.UploadedAt,
		); err != nil {
			return nil, 0, err
		}
		if impressionCount.Valid {
			v := impressionCount.Int64
			creative.ImpressionCount = &v
		}
		creatives = append(creatives, &creative)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate creatives: %w", err)
	}

	return creatives, total, nil
}

func (r *creativeRepository) ListByCampaign(ctx context.Context, campaignID string, limit int, offset int) ([]*models.Creative, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT
			id, name, type, url, file_path, size, impression_count, impressions_served, play_weight, campaign_id, selected_days, time_slots, devices, uploaded_at
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

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.pool.Query(queryCtx, query, args...)
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
			&creative.PlayWeight,
			&creative.CampaignID,
			&creative.SelectedDays,
			&creative.TimeSlots,
			&creative.Devices,
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
	if err := r.ensurePool(); err != nil {
		return 0, err
	}

	query := `SELECT COUNT(*) FROM creatives WHERE campaign_id = $1`

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var total int
	if err := r.pool.QueryRow(queryCtx, query, campaignID).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *creativeRepository) ListByDevice(ctx context.Context, device string, activeNow bool, now time.Time, limit int, offset int) ([]*models.Creative, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT
			cr.id, cr.name, cr.type, cr.url, cr.file_path, cr.size, cr.impression_count, cr.impressions_served, cr.play_weight, cr.campaign_id, cr.selected_days, cr.time_slots, cr.devices, cr.uploaded_at
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

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.pool.Query(queryCtx, query, args...)
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
			&creative.PlayWeight,
			&creative.CampaignID,
			&creative.SelectedDays,
			&creative.TimeSlots,
			&creative.Devices,
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

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return creatives, nil
}

func (r *creativeRepository) CountByDevice(ctx context.Context, device string, activeNow bool, now time.Time) (int, error) {
	if err := r.ensurePool(); err != nil {
		return 0, err
	}

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

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var total int
	if err := r.pool.QueryRow(queryCtx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *creativeRepository) Update(ctx context.Context, id string, req *models.UpdateCreativeRequest) error {
	if err := r.ensurePool(); err != nil {
		return err
	}

	query := `
		UPDATE creatives
		SET name = COALESCE($1, name),
			type = COALESCE($2, type),
			url = COALESCE($3, url),
			file_path = COALESCE($4, file_path),
			size = COALESCE($5, size),
			impression_count = COALESCE($6, impression_count),
			play_weight = COALESCE($7, play_weight),
			selected_days = COALESCE($8::text[], selected_days),
			time_slots = COALESCE($9::text[], time_slots),
			devices = COALESCE($10::text[], devices)
		WHERE id = $11
		RETURNING id
	`

	var selectedDays any
	if req.SelectedDays != nil {
		selectedDays = *req.SelectedDays
	}
	var timeSlots any
	if req.TimeSlots != nil {
		timeSlots = *req.TimeSlots
	}
	var devices any
	if req.Devices != nil {
		devices = *req.Devices
	}

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	if err := r.pool.QueryRow(
		queryCtx,
		query,
		req.Name,
		req.Type,
		req.URL,
		req.FilePath,
		req.Size,
		req.ImpressionCount,
		req.PlayWeight,
		selectedDays,
		timeSlots,
		devices,
		id,
	).Scan(&id); err != nil {
		return err
	}
	return nil
}

type rotationDailyRow struct {
	creativeID  string
	servedCount int
}

func (r *creativeRepository) PickNextRotationalCreative(ctx context.Context, device string, campaignID string, candidateCreativeIDs []string) (string, error) {
	if err := r.ensurePool(); err != nil {
		return "", err
	}

	device = strings.TrimSpace(device)
	campaignID = strings.TrimSpace(campaignID)
	if device == "" || campaignID == "" {
		return "", fmt.Errorf("device and campaignID are required")
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

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	day := time.Now().UTC().Truncate(24 * time.Hour)
	dayStr := day.Format("2006-01-02")

	// Load weights for candidates.
	weights := make(map[string]int, len(unique))
	rowsW, err := tx.Query(ctx, `
		SELECT id::text, COALESCE(play_weight, 0)
		FROM creatives
		WHERE id = ANY($1::uuid[])
	`, unique)
	if err != nil {
		return "", err
	}
	for rowsW.Next() {
		var id string
		var w int
		if err := rowsW.Scan(&id, &w); err != nil {
			rowsW.Close()
			return "", err
		}
		weights[id] = w
	}
	rowsW.Close()
	if err := rowsW.Err(); err != nil {
		return "", err
	}
	for _, id := range unique {
		if _, ok := weights[id]; !ok {
			weights[id] = 0
		}
	}

	sumW := 0
	for _, id := range unique {
		w := weights[id]
		if w < 0 {
			w = 0
		}
		sumW += w
	}
	if sumW <= 0 {
		// Fallback: treat all weights as equal.
		sumW = len(unique)
		for _, id := range unique {
			weights[id] = 1
		}
	}

	served := make(map[string]int, len(unique))
	rowsS, err := tx.Query(ctx, `
		SELECT creative_id::text, served_count
		FROM creative_rotation_state_daily
		WHERE device = $1 AND campaign_id = $2 AND day = $3::date
		FOR UPDATE
	`, device, campaignID, dayStr)
	if err != nil {
		return "", err
	}
	for rowsS.Next() {
		var cid string
		var cnt int
		if err := rowsS.Scan(&cid, &cnt); err != nil {
			rowsS.Close()
			return "", err
		}
		served[cid] = cnt
	}
	rowsS.Close()
	if err := rowsS.Err(); err != nil {
		return "", err
	}

	// Determine next pick using deficit vs expected share after (total+1) deliveries.
	totalServed := 0
	for _, id := range unique {
		totalServed += served[id]
	}
	projectedTotal := totalServed + 1

	chosen := unique[0]
	bestScore := -math.MaxFloat64
	for _, id := range unique {
		w := weights[id]
		if w < 0 {
			w = 0
		}
		expected := (float64(projectedTotal) * float64(w)) / float64(sumW)
		deficit := expected - float64(served[id])
		if deficit > bestScore {
			bestScore = deficit
			chosen = id
		}
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO creative_rotation_state_daily (device, campaign_id, day, creative_id, served_count, updated_at)
		VALUES ($1, $2, $3::date, $4, 1, NOW() AT TIME ZONE 'UTC')
		ON CONFLICT (device, campaign_id, day, creative_id)
		DO UPDATE SET served_count = creative_rotation_state_daily.served_count + 1, updated_at = EXCLUDED.updated_at
	`, device, campaignID, dayStr, chosen)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
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
	if err := r.ensurePool(); err != nil {
		return err
	}

	query := `DELETE FROM creatives WHERE id = $1`

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	result, err := r.pool.Exec(queryCtx, query, id)
	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}
