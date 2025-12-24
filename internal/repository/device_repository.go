package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"scm/internal/models"
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
}

type DeviceFilters struct {
	ProjectID   *int
	City        *string
	Region      *string
	DeviceType  *string
}

type deviceRepository struct {
	db *sql.DB
}

func NewDeviceRepository(db *sql.DB) DeviceRepository {
	return &deviceRepository{db: db}
}

func (r *deviceRepository) Upsert(ctx context.Context, device *models.Device) error {
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
	_, err = r.db.ExecContext(ctx, query,
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
	query := `
		SELECT id, device_type, region, name, host_name, description, change,
			last_synced_at, sync_status, project, device_config, rtty_data,
			created_at, updated_at
		FROM devices
		WHERE host_name = $1
	`

	var device models.Device
	var deviceTypeJSON, regionJSON []byte
	err := r.db.QueryRowContext(ctx, query, hostName).Scan(
		&device.ID, &deviceTypeJSON, &regionJSON, &device.Name, &device.HostName,
		&device.Description, &device.Change, &device.LastSyncedAt, &device.SyncStatus,
		&device.Project, &device.DeviceConfig, &device.RttyData,
		&device.CreatedAt, &device.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
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
	query := `
		SELECT id, device_type, region, name, host_name, description, change,
			last_synced_at, sync_status, project, device_config, rtty_data,
			created_at, updated_at
		FROM devices
		ORDER BY created_at DESC
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

	rows, err := r.db.QueryContext(ctx, query, args...)
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
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM devices").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count devices: %w", err)
	}
	return count, nil
}

func (r *deviceRepository) ListByProject(ctx context.Context, projectID int, limit int, offset int) ([]*models.Device, error) {
	query := `
		SELECT id, device_type, region, name, host_name, description, change,
			last_synced_at, sync_status, project, device_config, rtty_data,
			created_at, updated_at
		FROM devices
		WHERE project = $1
		ORDER BY created_at DESC
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

	rows, err := r.db.QueryContext(ctx, query, args...)
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
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM devices WHERE project = $1", projectID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count devices by project: %w", err)
	}
	return count, nil
}

func (r *deviceRepository) ListWithFilters(ctx context.Context, filters DeviceFilters, limit int, offset int) ([]*models.Device, error) {
	query := "SELECT id, device_type, region, name, host_name, description, change, last_synced_at, sync_status, project, device_config, rtty_data, created_at, updated_at FROM devices WHERE 1=1"
	var args []any
	argIndex := 1

	if filters.ProjectID != nil {
		query += fmt.Sprintf(" AND project = $%d", argIndex)
		args = append(args, *filters.ProjectID)
		argIndex++
	}

	if filters.City != nil {
		// Search for city in the device_config JSONB field
		query += fmt.Sprintf(" AND device_config->>'city' = $%d", argIndex)
		args = append(args, *filters.City)
		argIndex++
	}

	if filters.Region != nil {
		// Search for region in the region JSONB field
		query += fmt.Sprintf(" AND region::text LIKE $%d", argIndex)
		args = append(args, "%"+*filters.Region+"%")
		argIndex++
	}

	if filters.DeviceType != nil {
		// Search for device_type in the device_type JSONB field
		query += fmt.Sprintf(" AND device_type::text LIKE $%d", argIndex)
		args = append(args, "%"+*filters.DeviceType+"%")
		argIndex++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
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
		query += fmt.Sprintf(" AND region::text LIKE $%d", argIndex)
		args = append(args, "%"+*filters.Region+"%")
		argIndex++
	}

	if filters.DeviceType != nil {
		query += fmt.Sprintf(" AND device_type::text LIKE $%d", argIndex)
		args = append(args, "%"+*filters.DeviceType+"%")
		argIndex++
	}

	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count devices with filters: %w", err)
	}
	return count, nil
}
