package models

import "time"

type Role struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	IsSystem    bool      `json:"is_system"`
	CreatedAt   time.Time `json:"created_at"`
}

type Permission struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateRoleRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
}

type UpdateRoleRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type CreatePermissionRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
}

type UpdatePermissionRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type SetRolePermissionsRequest struct {
	PermissionIDs []string `json:"permission_ids" validate:"required"`
}

type UserRoleAssignment struct {
	RoleID       string  `json:"role_id" validate:"required"`
	AdvertiserID *string `json:"advertiser_id"`
}

type SetUserRolesRequest struct {
	Roles []UserRoleAssignment `json:"roles" validate:"required"`
}
