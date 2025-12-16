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
    SelectedDays []string    `json:"selected_days"`
    TimeSlots    []string    `json:"time_slots"`
    Devices      []string    `json:"devices"`
    UploadedAt   time.Time   `json:"uploaded_at"`
}

type CreateCreativeRequest struct {
    Name     string      `json:"name" validate:"required"`
    Type     CreativeType `json:"type" validate:"required,oneof=image video"`
}

type UpdateCreativeRequest struct {
    Name         *string      `json:"name,omitempty" validate:"omitempty,min=1"`
    Type         *CreativeType `json:"type,omitempty" validate:"omitempty,oneof=image video"`
    URL          *string      `json:"url,omitempty"`
    FilePath     *string      `json:"-"`
    Size         *int64       `json:"size,omitempty"`
    SelectedDays *[]string    `json:"selected_days,omitempty"`
    TimeSlots    *[]string    `json:"time_slots,omitempty"`
    Devices      *[]string    `json:"devices,omitempty"`
}