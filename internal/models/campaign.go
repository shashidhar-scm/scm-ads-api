// internal/models/campaign.go
package models

import "time"

type CampaignStatus string

const (
    CampaignStatusDraft     CampaignStatus = "draft"
    CampaignStatusActive    CampaignStatus = "active"
    CampaignStatusPaused    CampaignStatus = "paused"
    CampaignStatusCompleted CampaignStatus = "completed"
)

type Campaign struct {
    ID           string        `json:"id"`
    Name         string        `json:"name" validate:"required"`
    Status       CampaignStatus `json:"status"`
    StartDate    time.Time     `json:"start_date" validate:"required"`
    EndDate      time.Time     `json:"end_date" validate:"required,gtfield=StartDate"`
    Budget       float64       `json:"budget" validate:"required,gt=0"`
    Spent        float64       `json:"spent"`
    Impressions  int           `json:"impressions"`
    Clicks       int           `json:"clicks"`
    CTR          float64       `json:"ctr"`
    AdvertiserID string        `json:"advertiser_id" validate:"required,uuid4"`
    CreatedAt    time.Time     `json:"created_at"`
    UpdatedAt    time.Time     `json:"updated_at"`
}

type CreateCampaignRequest struct {
    Name         string    `json:"name" validate:"required"`
    StartDate    time.Time `json:"start_date" validate:"required"`
    EndDate      time.Time `json:"end_date" validate:"required,gtfield=StartDate"`
    Budget       float64   `json:"budget" validate:"required,gt=0"`
    AdvertiserID string    `json:"advertiser_id" validate:"required,uuid4"`
}

type UpdateCampaignRequest struct {
    Name      *string    `json:"name,omitempty"`
    Status    *string    `json:"status,omitempty" validate:"omitempty,oneof=draft active paused completed"`
    StartDate *time.Time `json:"start_date,omitempty"`
    EndDate   *time.Time `json:"end_date,omitempty" validate:"omitempty,gtfield=StartDate"`
    Budget    *float64   `json:"budget,omitempty" validate:"omitempty,gt=0"`
}