package models

import (
	"time"
)

type Advertiser struct {
	ID        string    `json:"id"`
	Name      string    `json:"name" validate:"required,min=3,max=255"`
	Email     string    `json:"email,omitempty" validate:"omitempty,email"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateAdvertiserRequest struct {
	Name      string `json:"name" validate:"required,min=3,max=255"`
	Email     string `json:"email,omitempty" validate:"omitempty,email"`
	CreatedBy string `json:"created_by" validate:"required,uuid"`
}

type UpdateAdvertiserRequest struct {
	Name  *string `json:"name,omitempty" validate:"omitempty,min=3,max=255"`
	Email *string `json:"email,omitempty" validate:"omitempty,email"`
}
