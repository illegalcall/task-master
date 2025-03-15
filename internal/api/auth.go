package api

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"github.com/illegalcall/task-master/internal/pkg/supabase"
)

type LoginRequest struct {
	Email    string `json:"email"` // Changed from Username to Email
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	TokenType string `json:"type"`
}

func (s *Server) handleLogin(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email and password are required",
		})
	}

	// Log authentication attempt
	s.logger.Info("Authentication attempt", "email", req.Email)

	// Validate credentials with Supabase
	valid, err := supabase.ValidateCredentials(req.Email, req.Password)
	if err != nil {
		// Log the detailed error for server-side debugging
		s.logger.Error("Authentication error", "error", err)

		// Return user-friendly error message
		errorMessage := "Authentication service error"
		if s.cfg.Server.Environment != "production" {
			// In non-production environments, include error details
			errorMessage = fmt.Sprintf("Authentication error: %v", err)
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": errorMessage,
		})
	}

	if !valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid credentials",
		})
	}

	// Generate JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email": req.Email, // Use email instead of username
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	})

	tokenString, err := token.SignedString([]byte(s.cfg.JWT.Secret))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}

	s.logger.Info("User successfully authenticated", "email", req.Email)

	return c.JSON(LoginResponse{
		Token:     tokenString,
		TokenType: "Bearer",
	})
}

// TODO: Replace with database lookup
func isValidCredentials(username, password string) bool {
	return username == "admin" && password == "password"
}
