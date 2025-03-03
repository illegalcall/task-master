package models

// LoginRequest represents the login credentials
type LoginRequest struct {
    // User's email address
    Email string `json:"email" example:"user@example.com"`
    // User's password
    Password string `json:"password" example:"password123"`
}

// LoginResponse represents the login response
type LoginResponse struct {
    // JWT token for authentication
    Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}

// APIResponse represents a generic API response
type APIResponse struct {
    // Status of the response (success/error)
    Status string `json:"status" example:"success"`
    // Response message
    Message string `json:"message" example:"Operation completed successfully"`
    // Optional data payload
    Data interface{} `json:"data,omitempty"`
} 