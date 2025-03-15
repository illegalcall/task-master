package api

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/illegalcall/task-master/internal/models"
)

// handleCreateProfile handles the creation of a new user profile when a user registers
func (s *Server) handleCreateProfile(c *fiber.Ctx) error {
	var req models.NewProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.UserID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	s.logger.Info("Creating profile for user", "user_id", req.UserID)

	// Check if profile already exists
	var count int
	err := s.db.DB.Get(&count, "SELECT COUNT(*) FROM profiles WHERE id = $1", req.UserID)
	if err != nil {
		s.logger.Error("Failed to check for existing profile", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to check for existing profile",
		})
	}

	if count > 0 {
		s.logger.Info("Profile already exists for user", "user_id", req.UserID)
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Profile already exists for this user",
		})
	}

	// Create a new profile with default values
	profile := models.Profile{
		ID:        req.UserID,
		CreatedAt: time.Now(),
		Credit:    100, // Default credit value (changed from 500 to 100)
	}

	// PostgreSQL will set default values for api_key and created_at
	// using the DEFAULT constraints defined in the table
	_, err = s.db.DB.Exec(
		`INSERT INTO profiles (id) VALUES ($1)`,
		profile.ID,
	)

	if err != nil {
		s.logger.Error("Failed to create profile", "error", err, "user_id", req.UserID)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create profile",
		})
	}

	// Fetch the complete profile with the generated API key
	err = s.db.DB.Get(&profile, "SELECT * FROM profiles WHERE id = $1", profile.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			s.logger.Error("Profile not found after creation", "user_id", req.UserID)
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Profile not found after creation",
			})
		}
		s.logger.Error("Failed to fetch created profile", "error", err, "user_id", req.UserID)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch created profile",
		})
	}

	s.logger.Info("Profile created successfully", "user_id", req.UserID, "api_key", profile.APIKey)

	return c.Status(fiber.StatusCreated).JSON(models.NewProfileResponse{
		Profile: profile,
		Success: true,
	})
}
