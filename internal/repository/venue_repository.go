package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"scm/internal/models"
)

type VenueRepository interface {
	Create(ctx context.Context, venue *models.Venue) error
	GetByID(ctx context.Context, id int) (*models.Venue, error)
	GetByIDWithDevices(ctx context.Context, id int) (*models.VenueWithDevices, error)
	GetByName(ctx context.Context, name string) (*models.Venue, error)
	List(ctx context.Context, limit int, offset int) ([]*models.Venue, error)
	Count(ctx context.Context) (int, error)
	Update(ctx context.Context, venue *models.Venue) error
	Delete(ctx context.Context, id int) error
	
	// Many-to-many operations
	AddDeviceToVenue(ctx context.Context, venueID, deviceID int) error
	RemoveDeviceFromVenue(ctx context.Context, venueID, deviceID int) error
	GetVenuesByDeviceID(ctx context.Context, deviceID int, limit int, offset int) ([]*models.Venue, error)
	CountVenuesByDeviceID(ctx context.Context, deviceID int) (int, error)
	GetDevicesByVenueID(ctx context.Context, venueID int, limit int, offset int) ([]*models.Device, error)
	CountDevicesByVenueID(ctx context.Context, venueID int) (int, error)
}

type venueRepository struct {
	db *sql.DB
}

func NewVenueRepository(db *sql.DB) VenueRepository {
	return &venueRepository{db: db}
}

func (r *venueRepository) Create(ctx context.Context, venue *models.Venue) error {
	query := `INSERT INTO venues (name, created_at, updated_at) 
			  VALUES ($1, $2, $3) RETURNING id`
	
	now := time.Now()
	err := r.db.QueryRowContext(ctx, query, venue.Name, now, now).Scan(&venue.ID)
	if err != nil {
		return fmt.Errorf("create venue: %w", err)
	}
	
	venue.CreatedAt = now
	venue.UpdatedAt = now
	return nil
}

func (r *venueRepository) GetByID(ctx context.Context, id int) (*models.Venue, error) {
	query := `SELECT id, name, created_at, updated_at 
			  FROM venues WHERE id = $1`
	
	var venue models.Venue
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&venue.ID, &venue.Name, &venue.CreatedAt, &venue.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("venue not found")
		}
		return nil, fmt.Errorf("get venue by id: %w", err)
	}
	
	return &venue, nil
}

func (r *venueRepository) GetByIDWithDevices(ctx context.Context, id int) (*models.VenueWithDevices, error) {
	// Get venue details
	venueQuery := `SELECT id, name, created_at, updated_at FROM venues WHERE id = $1`
	var venue models.VenueWithDevices
	err := r.db.QueryRowContext(ctx, venueQuery, id).Scan(
		&venue.ID, &venue.Name, &venue.CreatedAt, &venue.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("venue not found")
		}
		return nil, fmt.Errorf("get venue by id: %w", err)
	}
	
	// Get associated devices
	devicesQuery := `SELECT d.id, d.device_type, d.region, d.name, d.host_name, d.description, d.change, d.last_synced_at, d.sync_status, d.project, d.device_config, d.rtty_data, d.created_at, d.updated_at
					 FROM devices d 
					 JOIN venue_devices vd ON d.id = vd.device_id 
					 WHERE vd.venue_id = $1`
	
	rows, err := r.db.QueryContext(ctx, devicesQuery, id)
	if err != nil {
		return nil, fmt.Errorf("get devices for venue: %w", err)
	}
	defer rows.Close()
	
	var devices []models.Device
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
		
		// Note: We need to import encoding/json for this
		if err := json.Unmarshal(deviceTypeJSON, &device.DeviceType); err != nil {
			return nil, fmt.Errorf("unmarshal device_type: %w", err)
		}
		if err := json.Unmarshal(regionJSON, &device.Region); err != nil {
			return nil, fmt.Errorf("unmarshal region: %w", err)
		}
		
		devices = append(devices, device)
	}
	
	venue.Devices = devices
	return &venue, nil
}

func (r *venueRepository) GetByName(ctx context.Context, name string) (*models.Venue, error) {
	query := `SELECT id, name, created_at, updated_at 
			  FROM venues WHERE name = $1`
	
	var venue models.Venue
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&venue.ID, &venue.Name, &venue.CreatedAt, &venue.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("venue not found")
		}
		return nil, fmt.Errorf("get venue by name: %w", err)
	}
	
	return &venue, nil
}

func (r *venueRepository) List(ctx context.Context, limit int, offset int) ([]*models.Venue, error) {
	query := `SELECT id, name, created_at, updated_at 
			  FROM venues ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	
	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list venues: %w", err)
	}
	defer rows.Close()
	
	var venues []*models.Venue
	for rows.Next() {
		var venue models.Venue
		if err := rows.Scan(&venue.ID, &venue.Name, &venue.CreatedAt, &venue.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan venue: %w", err)
		}
		venues = append(venues, &venue)
	}
	
	return venues, nil
}

func (r *venueRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM venues").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count venues: %w", err)
	}
	return count, nil
}

func (r *venueRepository) Update(ctx context.Context, venue *models.Venue) error {
	query := `UPDATE venues SET name = $1, updated_at = $2 
			  WHERE id = $3`
	
	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, venue.Name, now, venue.ID)
	if err != nil {
		return fmt.Errorf("update venue: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("venue not found")
	}
	
	venue.UpdatedAt = now
	return nil
}

func (r *venueRepository) Delete(ctx context.Context, id int) error {
	query := "DELETE FROM venues WHERE id = $1"
	
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete venue: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("venue not found")
	}
	
	return nil
}

// Many-to-many operations
func (r *venueRepository) AddDeviceToVenue(ctx context.Context, venueID, deviceID int) error {
	// Check if venue exists
	var venueExists bool
	err := r.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM venues WHERE id = $1)", venueID).Scan(&venueExists)
	if err != nil {
		return fmt.Errorf("check venue exists: %w", err)
	}
	if !venueExists {
		return fmt.Errorf("venue not found")
	}
	
	// Check if device exists
	var deviceExists bool
	err = r.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM devices WHERE id = $1)", deviceID).Scan(&deviceExists)
	if err != nil {
		return fmt.Errorf("check device exists: %w", err)
	}
	if !deviceExists {
		return fmt.Errorf("device not found")
	}
	
	// Add device to venue
	query := `INSERT INTO venue_devices (venue_id, device_id, added_at) 
			  VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	
	now := time.Now()
	_, err = r.db.ExecContext(ctx, query, venueID, deviceID, now)
	if err != nil {
		return fmt.Errorf("add device to venue: %w", err)
	}
	
	return nil
}

func (r *venueRepository) RemoveDeviceFromVenue(ctx context.Context, venueID, deviceID int) error {
	// Check if venue exists
	var venueExists bool
	err := r.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM venues WHERE id = $1)", venueID).Scan(&venueExists)
	if err != nil {
		return fmt.Errorf("check venue exists: %w", err)
	}
	if !venueExists {
		return fmt.Errorf("venue not found")
	}
	
	// Check if device exists
	var deviceExists bool
	err = r.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM devices WHERE id = $1)", deviceID).Scan(&deviceExists)
	if err != nil {
		return fmt.Errorf("check device exists: %w", err)
	}
	if !deviceExists {
		return fmt.Errorf("device not found")
	}
	
	// Remove device from venue
	query := `DELETE FROM venue_devices WHERE venue_id = $1 AND device_id = $2`
	
	result, err := r.db.ExecContext(ctx, query, venueID, deviceID)
	if err != nil {
		return fmt.Errorf("remove device from venue: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("device not found in venue")
	}
	
	return nil
}

func (r *venueRepository) GetVenuesByDeviceID(ctx context.Context, deviceID int, limit int, offset int) ([]*models.Venue, error) {
	query := `SELECT v.id, v.name, v.created_at, v.updated_at 
			  FROM venues v 
			  JOIN venue_devices vd ON v.id = vd.venue_id 
			  WHERE vd.device_id = $1 ORDER BY v.created_at DESC LIMIT $2 OFFSET $3`
	
	rows, err := r.db.QueryContext(ctx, query, deviceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get venues by device: %w", err)
	}
	defer rows.Close()
	
	var venues []*models.Venue
	for rows.Next() {
		var venue models.Venue
		if err := rows.Scan(&venue.ID, &venue.Name, &venue.CreatedAt, &venue.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan venue: %w", err)
		}
		venues = append(venues, &venue)
	}
	
	return venues, nil
}

func (r *venueRepository) CountVenuesByDeviceID(ctx context.Context, deviceID int) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM venue_devices WHERE device_id = $1", deviceID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count venues by device: %w", err)
	}
	return count, nil
}

func (r *venueRepository) GetDevicesByVenueID(ctx context.Context, venueID int, limit int, offset int) ([]*models.Device, error) {
	query := `SELECT d.id, d.device_type, d.region, d.name, d.host_name, d.description, d.change, d.last_synced_at, d.sync_status, d.project, d.device_config, d.rtty_data, d.created_at, d.updated_at
			  FROM devices d 
			  JOIN venue_devices vd ON d.id = vd.device_id 
			  WHERE vd.venue_id = $1 ORDER BY d.created_at DESC LIMIT $2 OFFSET $3`
	
	rows, err := r.db.QueryContext(ctx, query, venueID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get devices by venue: %w", err)
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

func (r *venueRepository) CountDevicesByVenueID(ctx context.Context, venueID int) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM venue_devices WHERE venue_id = $1", venueID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count devices by venue: %w", err)
	}
	return count, nil
}
