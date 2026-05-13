package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"scm/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DeviceRepository interface {
	Upsert(ctx context.Context, device *models.Device) error
	GetByHostName(ctx context.Context, hostName string) (*models.Device, error)
	List(ctx context.Context, limit int, offset int) ([]*models.Device, error)
	Count(ctx context.Context) (int, error)
	ListByProject(ctx context.Context, projectID int, limit int, offset int) ([]*models.Device, error)
	CountByProject(ctx context.Context, projectID int) (int, error)
	ListWithFilters(ctx context.Context, filters DeviceFilters, limit int, offset int) ([]*models.Device, error)
	CountWithFilters(ctx context.Context, filters DeviceFilters) (int, error)
	CountByRegion(ctx context.Context, city *string, test *bool) ([]RegionDeviceCount, error)
	Search(ctx context.Context, term string, city *string, region *string, limit int, offset int) ([]*models.Device, int, error)
	Recommend(ctx context.Context, city string, region string, limit int) ([]DeviceRecommendation, error)
}

type RegionDeviceCount struct {
	Region string  `json:"region"`
	Count  int     `json:"count"`
	City   *string `json:"city"`
}

type DeviceRecommendation struct {
	HostName string  `json:"host_name"`
	Name     string  `json:"name"`
	City     *string `json:"city,omitempty"`
	Region   *string `json:"region,omitempty"`
	Score    float64 `json:"score"`

	Features map[string]POIFeature `json:"features,omitempty"`

	CollegeCount1km          int      `json:"college_count_1km"`
	CollegeCount2km          int      `json:"college_count_2km"`
	DormCount1km             int      `json:"dorm_count_1km"`
	HostelCount1km           int      `json:"hostel_count_1km"`
	NearestCollegeDistanceKm *float64 `json:"nearest_college_distance_km,omitempty"`
}

type POIFeature struct {
	Count1km          int      `json:"count_1km"`
	Count2km          int      `json:"count_2km"`
	NearestDistanceKm *float64 `json:"nearest_distance_km,omitempty"`
}

type DeviceFilters struct {
	ProjectID  *int
	City       *string
	Region     *string
	DeviceType *string
	Test       *bool
}

type deviceRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewDeviceRepository(pool *pgxpool.Pool) DeviceRepository {
	return &deviceRepository{pool: pool, queryTimeout: 5 * time.Second}
}

func (r *deviceRepository) ensurePool() error {
	if r.pool == nil {
		return errors.New("device repository: pgx pool is nil")
	}
	return nil
}

func (r *deviceRepository) translateNoRows(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return sql.ErrNoRows
	}
	return err
}

func (r *deviceRepository) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, r.queryTimeout)
}

func (r *deviceRepository) Upsert(ctx context.Context, device *models.Device) error {
	if err := r.ensurePool(); err != nil {
		return err
	}

	deviceTypeJSON, err := json.Marshal(device.DeviceType)
	if err != nil {
		return fmt.Errorf("marshal device_type: %w", err)
	}
	regionJSON, err := json.Marshal(device.Region)
	if err != nil {
		return fmt.Errorf("marshal region: %w", err)
	}

	query := `
		INSERT INTO devices (
			id, device_type, region, name, host_name, description, change,
			last_synced_at, sync_status, project, device_config, rtty_data,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (host_name) DO UPDATE SET
			id = EXCLUDED.id,
			device_type = EXCLUDED.device_type,
			region = EXCLUDED.region,
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			change = EXCLUDED.change,
			last_synced_at = EXCLUDED.last_synced_at,
			sync_status = EXCLUDED.sync_status,
			project = EXCLUDED.project,
			device_config = EXCLUDED.device_config,
			rtty_data = EXCLUDED.rtty_data,
			updated_at = NOW()
	`

	now := time.Now().UTC()
	execCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	_, err = r.pool.Exec(execCtx, query,
		device.ID, deviceTypeJSON, regionJSON, device.Name, device.HostName,
		device.Description, device.Change, device.LastSyncedAt, device.SyncStatus,
		device.Project, device.DeviceConfig, device.RttyData,
		now, now,
	)
	if err != nil {
		return fmt.Errorf("upsert device: %w", err)
	}
	return nil
}

func (r *deviceRepository) GetByHostName(ctx context.Context, hostName string) (*models.Device, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, device_type, region, name, host_name, description, change,
			last_synced_at, sync_status, project, device_config, rtty_data,
			created_at, updated_at
		FROM devices
		WHERE host_name = $1
	`

	rowCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var device models.Device
	var deviceTypeJSON, regionJSON []byte
	err := r.pool.QueryRow(rowCtx, query, hostName).Scan(
		&device.ID, &deviceTypeJSON, &regionJSON, &device.Name, &device.HostName,
		&device.Description, &device.Change, &device.LastSyncedAt, &device.SyncStatus,
		&device.Project, &device.DeviceConfig, &device.RttyData,
		&device.CreatedAt, &device.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("device not found")
		}
		return nil, fmt.Errorf("get device: %w", err)
	}

	if err := json.Unmarshal(deviceTypeJSON, &device.DeviceType); err != nil {
		return nil, fmt.Errorf("unmarshal device_type: %w", err)
	}
	if err := json.Unmarshal(regionJSON, &device.Region); err != nil {
		return nil, fmt.Errorf("unmarshal region: %w", err)
	}

	return &device, nil
}

func (r *deviceRepository) List(ctx context.Context, limit int, offset int) ([]*models.Device, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, device_type, region, name, host_name, description, change,
			last_synced_at, sync_status, project, device_config, rtty_data,
			created_at, updated_at
		FROM devices
		ORDER BY host_name ASC
	`

	args := []interface{}{}
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
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()

	var devices []*models.Device
	for rows.Next() {
		var device models.Device
		var deviceTypeJSON, regionJSON []byte
		if err := rows.Scan(
			&device.ID, &deviceTypeJSON, &regionJSON, &device.Name, &device.HostName,
			&device.Description, &device.Change, &device.LastSyncedAt, &device.SyncStatus,
			&device.Project, &device.DeviceConfig, &device.RttyData,
			&device.CreatedAt, &device.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}

		if err := json.Unmarshal(deviceTypeJSON, &device.DeviceType); err != nil {
			return nil, fmt.Errorf("unmarshal device_type: %w", err)
		}
		if err := json.Unmarshal(regionJSON, &device.Region); err != nil {
			return nil, fmt.Errorf("unmarshal region: %w", err)
		}

		devices = append(devices, &device)
	}

	return devices, rows.Err()
}

func (r *deviceRepository) Count(ctx context.Context) (int, error) {
	if err := r.ensurePool(); err != nil {
		return 0, err
	}

	var count int
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	err := r.pool.QueryRow(queryCtx, "SELECT COUNT(*) FROM devices").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count devices: %w", err)
	}
	return count, nil
}

func (r *deviceRepository) ListByProject(ctx context.Context, projectID int, limit int, offset int) ([]*models.Device, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, device_type, region, name, host_name, description, change,
			last_synced_at, sync_status, project, device_config, rtty_data,
			created_at, updated_at
		FROM devices
		WHERE project = $1
		ORDER BY host_name ASC
	`

	args := []interface{}{projectID}
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
		return nil, fmt.Errorf("list devices by project: %w", err)
	}
	defer rows.Close()

	var devices []*models.Device
	for rows.Next() {
		var device models.Device
		var deviceTypeJSON, regionJSON []byte
		if err := rows.Scan(
			&device.ID, &deviceTypeJSON, &regionJSON, &device.Name, &device.HostName,
			&device.Description, &device.Change, &device.LastSyncedAt, &device.SyncStatus,
			&device.Project, &device.DeviceConfig, &device.RttyData,
			&device.CreatedAt, &device.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}

		if err := json.Unmarshal(deviceTypeJSON, &device.DeviceType); err != nil {
			return nil, fmt.Errorf("unmarshal device_type: %w", err)
		}
		if err := json.Unmarshal(regionJSON, &device.Region); err != nil {
			return nil, fmt.Errorf("unmarshal region: %w", err)
		}

		devices = append(devices, &device)
	}

	return devices, rows.Err()
}

func (r *deviceRepository) CountByProject(ctx context.Context, projectID int) (int, error) {
	if err := r.ensurePool(); err != nil {
		return 0, err
	}

	var count int
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	err := r.pool.QueryRow(queryCtx, "SELECT COUNT(*) FROM devices WHERE project = $1", projectID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count devices by project: %w", err)
	}
	return count, nil
}

func (r *deviceRepository) ListWithFilters(ctx context.Context, filters DeviceFilters, limit int, offset int) ([]*models.Device, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := "SELECT id, device_type, region, name, host_name, description, change, last_synced_at, sync_status, project, device_config, rtty_data, created_at, updated_at FROM devices WHERE 1=1"
	var args []any
	argIndex := 1

	if filters.ProjectID != nil {
		query += fmt.Sprintf(" AND project = $%d", argIndex)
		args = append(args, *filters.ProjectID)
		argIndex++
	}

	if filters.City != nil {
		query += fmt.Sprintf(" AND device_config->>'city' = $%d", argIndex)
		args = append(args, *filters.City)
		argIndex++
	}

	if filters.Region != nil {
		query += fmt.Sprintf(" AND region->>'code' = $%d", argIndex)
		args = append(args, *filters.Region)
		argIndex++
	}

	if filters.DeviceType != nil {
		query += fmt.Sprintf(" AND device_type::text LIKE $%d", argIndex)
		args = append(args, "%"+*filters.DeviceType+"%")
		argIndex++
	}

	if filters.Test != nil {
		query += fmt.Sprintf(" AND device_config->>'test' = $%d", argIndex)
		args = append(args, strconv.FormatBool(*filters.Test))
		argIndex++
	}

	query += fmt.Sprintf(" ORDER BY host_name ASC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, limit, offset)

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.pool.Query(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list devices with filters: %w", err)
	}
	defer rows.Close()

	var devices []*models.Device
	for rows.Next() {
		var device models.Device
		var deviceTypeJSON, regionJSON []byte
		if err := rows.Scan(
			&device.ID, &deviceTypeJSON, &regionJSON, &device.Name, &device.HostName,
			&device.Description, &device.Change, &device.LastSyncedAt, &device.SyncStatus,
			&device.Project, &device.DeviceConfig, &device.RttyData,
			&device.CreatedAt, &device.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}

		if err := json.Unmarshal(deviceTypeJSON, &device.DeviceType); err != nil {
			return nil, fmt.Errorf("unmarshal device_type: %w", err)
		}
		if err := json.Unmarshal(regionJSON, &device.Region); err != nil {
			return nil, fmt.Errorf("unmarshal region: %w", err)
		}

		devices = append(devices, &device)
	}

	return devices, nil
}

func (r *deviceRepository) CountWithFilters(ctx context.Context, filters DeviceFilters) (int, error) {
	if err := r.ensurePool(); err != nil {
		return 0, err
	}

	query := "SELECT COUNT(*) FROM devices WHERE 1=1"
	var args []any
	argIndex := 1

	if filters.ProjectID != nil {
		query += fmt.Sprintf(" AND project = $%d", argIndex)
		args = append(args, *filters.ProjectID)
		argIndex++
	}

	if filters.City != nil {
		query += fmt.Sprintf(" AND device_config->>'city' = $%d", argIndex)
		args = append(args, *filters.City)
		argIndex++
	}

	if filters.Region != nil {
		query += fmt.Sprintf(" AND region->>'code' = $%d", argIndex)
		args = append(args, *filters.Region)
		argIndex++
	}

	if filters.DeviceType != nil {
		query += fmt.Sprintf(" AND device_type::text LIKE $%d", argIndex)
		args = append(args, "%"+*filters.DeviceType+"%")
		argIndex++
	}

	if filters.Test != nil {
		query += fmt.Sprintf(" AND device_config->>'test' = $%d", argIndex)
		args = append(args, strconv.FormatBool(*filters.Test))
		argIndex++
	}

	var count int
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	err := r.pool.QueryRow(queryCtx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count devices with filters: %w", err)
	}
	return count, nil
}

func (r *deviceRepository) CountByRegion(ctx context.Context, city *string, test *bool) ([]RegionDeviceCount, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT
			NULLIF(device_config->>'city', '') AS city,
			COALESCE(region->>'code', '') AS region_code,
			COUNT(*)::int AS device_count
		FROM devices
		WHERE 1=1
	`

	var args []any
	argIndex := 1
	if city != nil {
		query += fmt.Sprintf(" AND device_config->>'city' = $%d", argIndex)
		args = append(args, *city)
		argIndex++
	}
	if test != nil {
		query += fmt.Sprintf(" AND device_config->>'test' = $%d", argIndex)
		args = append(args, strconv.FormatBool(*test))
		argIndex++
	}

	query += " GROUP BY 1, 2 ORDER BY device_count DESC"

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.pool.Query(queryCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("count devices by region: %w", err)
	}
	defer rows.Close()

	var out []RegionDeviceCount
	for rows.Next() {
		var row RegionDeviceCount
		var cityNS sql.NullString
		if err := rows.Scan(&cityNS, &row.Region, &row.Count); err != nil {
			return nil, fmt.Errorf("scan region device count: %w", err)
		}
		if cityNS.Valid {
			v := cityNS.String
			row.City = &v
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows region device count: %w", err)
	}

	return out, nil
}

func (r *deviceRepository) Search(ctx context.Context, term string, city *string, region *string, limit int, offset int) ([]*models.Device, int, error) {
	if err := r.ensurePool(); err != nil {
		return nil, 0, err
	}

	likeTerm := fmt.Sprintf("%%%s%%", strings.ToLower(term))

	whereClause := `
		(
			LOWER(host_name) LIKE $1
			OR LOWER(name) LIKE $1
			OR LOWER(description) LIKE $1
			OR LOWER(device_config->>'address') LIKE $1
		)
	`

	args := []any{likeTerm}
	argIndex := 2

	if city != nil {
		whereClause += fmt.Sprintf(" AND device_config->>'city' = $%d", argIndex)
		args = append(args, *city)
		argIndex++
	}
	if region != nil {
		whereClause += fmt.Sprintf(" AND region->>'code' = $%d", argIndex)
		args = append(args, *region)
		argIndex++
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM devices WHERE %s", whereClause)
	var total int
	countCtx, cancelCount := r.withTimeout(ctx)
	defer cancelCount()

	if err := r.pool.QueryRow(countCtx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count search devices: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, device_type, region, name, host_name, description, change,
			last_synced_at, sync_status, project, device_config, rtty_data,
			created_at, updated_at
		FROM devices
		WHERE %s
		ORDER BY host_name ASC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.pool.Query(queryCtx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search devices: %w", err)
	}
	defer rows.Close()

	var devices []*models.Device
	for rows.Next() {
		var device models.Device
		var deviceTypeJSON, regionJSON []byte
		if err := rows.Scan(
			&device.ID, &deviceTypeJSON, &regionJSON, &device.Name, &device.HostName,
			&device.Description, &device.Change, &device.LastSyncedAt, &device.SyncStatus,
			&device.Project, &device.DeviceConfig, &device.RttyData,
			&device.CreatedAt, &device.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan device: %w", err)
		}

		if err := json.Unmarshal(deviceTypeJSON, &device.DeviceType); err != nil {
			return nil, 0, fmt.Errorf("unmarshal device_type: %w", err)
		}
		if err := json.Unmarshal(regionJSON, &device.Region); err != nil {
			return nil, 0, fmt.Errorf("unmarshal region: %w", err)
		}

		devices = append(devices, &device)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows search devices: %w", err)
	}

	return devices, total, nil
}

func (r *deviceRepository) Recommend(ctx context.Context, city string, region string, limit int) ([]DeviceRecommendation, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	city = strings.TrimSpace(city)
	region = strings.TrimSpace(region)
	if city == "" {
		return nil, fmt.Errorf("city is required")
	}
	if region == "" {
		return nil, fmt.Errorf("region is required")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	// Score is an MVP heuristic. It assumes device_poi_feature_values is periodically populated.
	// If a device has no POI feature rows, it will be excluded from recommendations.
	query := `
		WITH by_cat AS (
			SELECT
				host_name,
				category,
				MAX(poi_count) FILTER (WHERE radius_m = 1000) AS count_1km,
				MAX(poi_count) FILTER (WHERE radius_m = 2000) AS count_2km,
				MIN(nearest_distance_km) FILTER (WHERE radius_m = 1000) AS nearest_distance_km
			FROM device_poi_feature_values
			WHERE city = $1
			  AND region_code = $2
			GROUP BY host_name, category
		),
		agg AS (
			SELECT
				host_name,
				MAX(count_1km) FILTER (WHERE category = 'college') AS college_count_1km,
				MAX(count_2km) FILTER (WHERE category = 'college') AS college_count_2km,
				MAX(count_1km) FILTER (WHERE category = 'dorm') AS dorm_count_1km,
				MAX(count_1km) FILTER (WHERE category = 'hostel') AS hostel_count_1km,
				MIN(nearest_distance_km) FILTER (WHERE category = 'college') AS nearest_college_distance_km,
				MAX(count_1km) FILTER (WHERE category = 'restaurant') AS restaurant_count_1km,
				MIN(nearest_distance_km) FILTER (WHERE category = 'restaurant') AS nearest_restaurant_distance_km,
				MAX(count_1km) FILTER (WHERE category = 'pub_bar') AS pub_bar_count_1km,
				MIN(nearest_distance_km) FILTER (WHERE category = 'pub_bar') AS nearest_pub_bar_distance_km,
				MAX(count_1km) FILTER (WHERE category = 'hotel') AS hotel_count_1km,
				MAX(count_1km) FILTER (WHERE category = 'mobile_shop') AS mobile_shop_count_1km,
				jsonb_object_agg(
					category,
					jsonb_build_object(
						'count_1km', COALESCE(count_1km, 0),
						'count_2km', COALESCE(count_2km, 0),
						'nearest_distance_km', nearest_distance_km
					)
				) AS features
			FROM by_cat
			GROUP BY host_name
		)
		SELECT
			d.host_name,
			COALESCE(d.name, '') AS name,
			NULLIF(d.device_config->>'city', '') AS city,
			NULLIF(d.region->>'code', '') AS region_code,
			COALESCE(a.college_count_1km, 0) AS college_count_1km,
			COALESCE(a.college_count_2km, 0) AS college_count_2km,
			COALESCE(a.dorm_count_1km, 0) AS dorm_count_1km,
			COALESCE(a.hostel_count_1km, 0) AS hostel_count_1km,
			a.nearest_college_distance_km,
			a.features,
			(
				5.0 * LN(1 + COALESCE(a.restaurant_count_1km, 0))
				+ 3.0 * LN(1 + COALESCE(a.pub_bar_count_1km, 0))
				+ 2.0 * LN(1 + COALESCE(a.college_count_1km, 0))
				+ 1.0 * LN(1 + COALESCE(a.college_count_2km, 0))
				+ 2.5 * LN(1 + COALESCE(a.dorm_count_1km, 0))
				+ 1.0 * LN(1 + COALESCE(a.hotel_count_1km, 0))
				+ 0.5 * LN(1 + COALESCE(a.mobile_shop_count_1km, 0))
				- 1.0 * COALESCE(a.nearest_restaurant_distance_km, 5.0)
				- 0.6 * COALESCE(a.nearest_pub_bar_distance_km, 5.0)
				- 0.4 * COALESCE(a.nearest_college_distance_km, 5.0)
			) AS score
		FROM devices d
		JOIN agg a ON a.host_name = d.host_name
		WHERE d.device_config->>'city' = $1
		  AND d.region->>'code' = $2
		  AND (d.device_config->>'test' IS NULL OR d.device_config->>'test' <> 'true')
		ORDER BY score DESC, d.host_name ASC
		LIMIT $3
	`

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.pool.Query(queryCtx, query, city, region, limit)
	if err != nil {
		return nil, fmt.Errorf("recommend devices: %w", err)
	}
	defer rows.Close()

	out := make([]DeviceRecommendation, 0, limit)
	for rows.Next() {
		var rec DeviceRecommendation
		var cityNS sql.NullString
		var regionNS sql.NullString
		var nearest sql.NullFloat64
		var featuresJSON []byte
		if err := rows.Scan(
			&rec.HostName,
			&rec.Name,
			&cityNS,
			&regionNS,
			&rec.CollegeCount1km,
			&rec.CollegeCount2km,
			&rec.DormCount1km,
			&rec.HostelCount1km,
			&nearest,
			&featuresJSON,
			&rec.Score,
		); err != nil {
			return nil, fmt.Errorf("scan recommendation: %w", err)
		}
		if cityNS.Valid {
			v := cityNS.String
			rec.City = &v
		}
		if regionNS.Valid {
			v := regionNS.String
			rec.Region = &v
		}
		if nearest.Valid {
			v := nearest.Float64
			rec.NearestCollegeDistanceKm = &v
		}
		if len(featuresJSON) > 0 {
			var m map[string]POIFeature
			if err := json.Unmarshal(featuresJSON, &m); err != nil {
				return nil, fmt.Errorf("unmarshal recommendation features: %w", err)
			}
			rec.Features = m
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recommend rows: %w", err)
	}

	return out, nil
}
