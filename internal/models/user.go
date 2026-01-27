package models

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email" validate:"required,email"`
	Name         string    `json:"name,omitempty"`
	UserName     string    `json:"user_name,omitempty"`
	PhoneNumber  string    `json:"phone_number,omitempty"`
	Status       string    `json:"status,omitempty"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type SignupRequest struct {
	Email       string `json:"email" validate:"required,email"`
	Password    string `json:"password" validate:"required,strongpassword"`
	Name        string `json:"name" validate:"required"`
	UserName    string `json:"user_name" validate:"required,alphanum"`
	PhoneNumber string `json:"phone_number" validate:"required"`
}

type LoginRequest struct {
	Identifier string `json:"identifier" validate:"required"`
	Password   string `json:"password" validate:"required"`
}

type GoogleAuthRequest struct {
	IDToken string `json:"id_token" validate:"required"`
}

type LoginRole struct {
	Name string `json:"name"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	Email       string `json:"email"`
	Name        string `json:"name,omitempty"`
	UserName    string `json:"user_name,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
	ID          string `json:"id"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	Role        string `json:"role"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,strongpassword"`
}

type UpdateUserRequest struct {
	Email       *string `json:"email,omitempty"`
	Name        *string `json:"name,omitempty"`
	UserName    *string `json:"user_name,omitempty"`
	PhoneNumber *string `json:"phone_number,omitempty"`
	Status      *string `json:"status,omitempty"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,strongpassword"`
}
