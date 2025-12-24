package models

import "time"

type Venue struct {
	ID        int       `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// VenueWithDevices includes the associated devices information
type VenueWithDevices struct {
	ID      int      `json:"id" db:"id"`
	Name    string   `json:"name" db:"name"`
	Devices []Device `json:"devices,omitempty"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// VenueDevice represents the relationship between venues and devices
type VenueDevice struct {
	VenueID  int       `json:"venue_id" db:"venue_id"`
	DeviceID int       `json:"device_id" db:"device_id"`
	AddedAt  time.Time `json:"added_at" db:"added_at"`
}
