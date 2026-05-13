package models

import "time"

// PlaceExchangeToken represents a cached Place Exchange access token document.
type PlaceExchangeToken struct {
	DocID     string    `json:"_id"`
	City      string    `json:"city"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
