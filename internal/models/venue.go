package models

import "time"

type Venue struct {
	ID          int       `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	SubCategory []string  `json:"sub_category" db:"sub_category"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type VenueRequest struct {
	Name        string   `json:"name"`
	SubCategory []string `json:"sub_category"`
}

type VenueUpdateRequest struct {
	Name        string   `json:"name"`
	SubCategory []string `json:"sub_category"`
}

// VenueWithDevices includes the associated devices information
type VenueWithDevices struct {
	ID          int       `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	SubCategory []string  `json:"sub_category" db:"sub_category"`
	Devices     []Device  `json:"devices,omitempty"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// VenueDevice represents the relationship between venues and devices
type VenueDevice struct {
	VenueID  int       `json:"venue_id" db:"venue_id"`
	DeviceID int       `json:"device_id" db:"device_id"`
	AddedAt  time.Time `json:"added_at" db:"added_at"`
}
