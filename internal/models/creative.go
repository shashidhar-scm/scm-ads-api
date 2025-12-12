// internal/models/creative.go
package models

import "time"

type CreativeType string

const (
    CreativeTypeImage CreativeType = "image"
    CreativeTypeVideo CreativeType = "video"
)

type Creative struct {
    ID           string      `json:"id"`
    Name         string      `json:"name" validate:"required"`
    Type         CreativeType `json:"type" validate:"required,oneof=image video"`
    URL          string      `json:"url"`
    FilePath     string      `json:"-"`
    Size         int64       `json:"size"`
    CampaignID   string      `json:"campaign_id" validate:"required,uuid4"`
    AdvertiserID string      `json:"advertiser_id" validate:"required,uuid4"`
    CreatedAt    time.Time   `json:"created_at"`
    UpdatedAt    time.Time   `json:"updated_at"`
}

type CreateCreativeRequest struct {
    Name     string      `json:"name" validate:"required"`
    Type     CreativeType `json:"type" validate:"required,oneof=image video"`
    CampaignID string    `json:"campaign_id" validate:"required,uuid4"`
}

type UpdateCreativeRequest struct {
    Name *string `json:"name,omitempty" validate:"omitempty,min=3"`
}