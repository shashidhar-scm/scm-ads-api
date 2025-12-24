package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type Device struct {
	ID           int                    `json:"id" db:"id"`
	DeviceType   DeviceType             `json:"device_type" db:"device_type"`
	Region       Region                 `json:"region" db:"region"`
	Name         string                 `json:"name" db:"name"`
	HostName     string                 `json:"host_name" db:"host_name"` // used as business key
	Description  string                 `json:"description" db:"description"`
	Change       bool                   `json:"change" db:"change"`
	LastSyncedAt *time.Time             `json:"last_synced_at" db:"last_synced_at"`
	SyncStatus   *string                `json:"sync_status" db:"sync_status"`
	Project      int                    `json:"project" db:"project"` // foreign key to Project.id (console ID)
	DeviceConfig json.RawMessage        `json:"device_config" db:"device_config"` // store as JSONB
	RttyData     int64                  `json:"rtty_data" db:"rtty_data"`

	// Local fields
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type DeviceType struct {
	ID          int     `json:"id" db:"id"`
	Name        string  `json:"name" db:"name"`
	Code        string  `json:"code" db:"code"`
	Description *string `json:"description" db:"description"`
}

type Region struct {
	ID                  int      `json:"id" db:"id"`
	Code                string   `json:"code" db:"code"`
	Name                string   `json:"name" db:"name"`
	Company             *string  `json:"company" db:"company"`
	Description         *string  `json:"description" db:"description"`
	MaxDevices          int      `json:"max_devices" db:"max_devices"`
	ProfileImg          string   `json:"profile_img" db:"profile_img"`
	Header              bool     `json:"header" db:"header"`
	SubType             *string  `json:"sub_type" db:"sub_type"`
	Production          bool     `json:"production" db:"production"`
	CityPosterFrequency int      `json:"city_poster_frequency" db:"city_poster_frequency"`
	AdPosterFrequency   int      `json:"ad_poster_frequency" db:"ad_poster_frequency"`
	CityPosterPlayTime  int      `json:"city_poster_play_time" db:"city_poster_play_time"`
	LoopLength          int      `json:"loop_length" db:"loop_length"`
	SmallbizSupport     bool     `json:"smallbiz_support" db:"smallbiz_support"`
	Proxy               *string  `json:"proxy" db:"proxy"`
	IsTransit           bool     `json:"is_transit" db:"is_transit"`
	TimeZone            string   `json:"time_zone" db:"time_zone"`
	IsoURL              string   `json:"iso_url" db:"iso_url"`
	Address             string   `json:"address" db:"address"`
	Latitude            string   `json:"latitude" db:"latitude"`
	Longitude           string   `json:"longitude" db:"longitude"`
	ScmHealth           bool     `json:"scm_health" db:"scm_health"`
	Replicas            int      `json:"replicas" db:"replicas"`
	Languages           []string `json:"languages" db:"languages"`
}

// Value implements driver.Valuer for JSONB serialization
func (r Region) Value() (driver.Value, error) {
	return json.Marshal(r)
}

// Scan implements sql.Scanner for JSONB deserialization
func (r *Region) Scan(value interface{}) error {
	if value == nil {
		*r = Region{}
		return nil
	}
	if bytes, ok := value.([]byte); ok {
		return json.Unmarshal(bytes, r)
	}
	return nil
}

// Value implements driver.Valuer for JSONB serialization
func (dt DeviceType) Value() (driver.Value, error) {
	return json.Marshal(dt)
}

// Scan implements sql.Scanner for JSONB deserialization
func (dt *DeviceType) Scan(value interface{}) error {
	if value == nil {
		*dt = DeviceType{}
		return nil
	}
	if bytes, ok := value.([]byte); ok {
		return json.Unmarshal(bytes, dt)
	}
	return nil
}

// Request/response DTOs for API layer (if needed later)
type CreateProjectRequest struct {
	// Add fields for creating a project manually, if you want to support that later
}

type UpdateProjectRequest struct {
	Name        *string  `json:"name,omitempty"`
	Company     *string  `json:"company,omitempty"`
	Description *string  `json:"description,omitempty"`
	// Add other updatable fields as needed
}

type CreateDeviceRequest struct {
	// Add fields for creating a device manually, if you want to support that later
}

type UpdateDeviceRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	// Add other updatable fields as needed
}
