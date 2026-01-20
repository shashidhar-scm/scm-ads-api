// internal/models/campaign.go
package models

import "time"

type CampaignStatus string

const (
    CampaignStatusDraft     CampaignStatus = "draft"
    CampaignStatusActive    CampaignStatus = "active"
    CampaignStatusPaused    CampaignStatus = "paused"
    CampaignStatusScheduled CampaignStatus = "scheduled"
    CampaignStatusCompleted CampaignStatus = "completed"
)

type Campaign struct {
    ID           string        `json:"id"`
    Name         string        `json:"name" validate:"required"`
    Status       CampaignStatus `json:"status"`
    Cities       []string      `json:"cities"`
    StartDate    time.Time     `json:"start_date" validate:"required"`
    EndDate      time.Time     `json:"end_date" validate:"required,gtfield=StartDate"`
    Budget       float64       `json:"budget" validate:"required,gt=0"`
    Spent        float64       `json:"spent"`
    ImpressionsBased bool      `json:"impressions_based"`
    LifetimeImpressions *int64  `json:"lifetime_impressions,omitempty"`
    TotalImpressions int64      `json:"total_impressions"`
    ServedImpressions int64     `json:"served_impressions"`
    Clicks       int           `json:"clicks"`
    CTR          float64       `json:"ctr"`
    AdvertiserID string        `json:"advertiser_id" validate:"required,uuid4"`
    CreatedAt    time.Time     `json:"created_at"`
    UpdatedAt    time.Time     `json:"updated_at"`
}

type CreateCampaignRequest struct {
    Name         string    `json:"name" validate:"required"`
    Cities       []string  `json:"cities"`
    StartDate    time.Time `json:"start_date" validate:"required"`
    EndDate      time.Time `json:"end_date" validate:"required,gtfield=StartDate"`
    Budget       float64   `json:"budget" validate:"required,gt=0"`
    AdvertiserID string    `json:"advertiser_id" validate:"required,uuid4"`
    ImpressionsBased bool  `json:"impressions_based"`
}

type UpdateCampaignRequest struct {
    Name      *string    `json:"name,omitempty"`
    Status    *string    `json:"status,omitempty" validate:"omitempty,oneof=draft active paused scheduled completed"`
    Cities    *[]string  `json:"cities,omitempty"`
    StartDate *time.Time `json:"start_date,omitempty"`
    EndDate   *time.Time `json:"end_date,omitempty" validate:"omitempty,gtfield=StartDate"`
    Budget    *float64   `json:"budget,omitempty" validate:"omitempty,gt=0"`
    ImpressionsBased *bool `json:"impressions_based,omitempty"`
}

type CampaignSummary struct {
    ActiveCampaignCount int     `json:"active_campaign_count"`
    TotalBudget         float64 `json:"total_budget"`
    TotalImpression     int64   `json:"total_impression"`
    ServedImpression    int64   `json:"served_impression"`
}