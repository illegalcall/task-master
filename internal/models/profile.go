package models

import (
	"time"
)

// Profile represents a user profile in the system
type Profile struct {
	ID        string    `json:"id" db:"id"`           // UUID that matches auth.users.id
	APIKey    string    `json:"api_key" db:"api_key"` // Unique API key for the profile
	Credit    int       `json:"credit" db:"credit"`   // Default value: 100 credits
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// NewProfileRequest is the request structure for creating a new profile
type NewProfileRequest struct {
	UserID string `json:"user_id"` // The user ID from Supabase auth
}

// NewProfileResponse is the response structure when a profile is created
type NewProfileResponse struct {
	Profile Profile `json:"profile"`
	Success bool    `json:"success"`
}
