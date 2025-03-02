package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/gofiber/fiber/v2"
	"github.com/illegalcall/task-master/internal/models"
)

const (
	maxPDFSize = 10 * 1024 * 1024 // 10MB
)

// handlePDFParseJob handles the POST /api/jobs/parse-document endpoint
func (s *Server) handlePDFParseJob(c *fiber.Ctx) error {
	ctx := c.Context()

	// Parse and validate the request payload
	var payload models.ParseDocumentPayload
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if err := validatePDFParsePayload(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Store the PDF file
	var pdfPath string
	var err error

	if strings.HasPrefix(payload.PDFSource, "http://") || strings.HasPrefix(payload.PDFSource, "https://") {
		// Handle URL source
		pdfPath, err = s.storage.StoreFromURL(ctx, payload.PDFSource)
	} else {
		// Handle base64 source
		pdfData, err := base64.StdEncoding.DecodeString(payload.PDFSource)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid base64-encoded PDF data",
			})
		}
		pdfPath, err = s.storage.StoreFromBytes(ctx, pdfData)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to store PDF: %v", err),
		})
	}

	// Create a new job
	job := models.Job{
		Name:   "PDF Parse Job",
		Status: models.StatusPending,
		Type:   models.JobTypePDFParse,
	}

	// Insert job into database
	err = s.db.DB.QueryRow(
		"INSERT INTO jobs (name, status, type) VALUES ($1, $2, $3) RETURNING id",
		job.Name, job.Status, job.Type,
	).Scan(&job.ID)
	if err != nil {
		// Clean up the stored file on error
		_ = s.storage.Delete(ctx, pdfPath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create job",
		})
	}

	// Store job payload in Redis
	jobPayload := struct {
		models.ParseDocumentPayload
		PDFPath string `json:"pdf_path"`
	}{
		ParseDocumentPayload: payload,
		PDFPath:             pdfPath,
	}

	payloadBytes, _ := json.Marshal(jobPayload)
	redisKey := fmt.Sprintf("job:%d:payload", job.ID)
	if err := s.db.Redis.Set(ctx, redisKey, payloadBytes, s.cfg.Storage.TTL).Err(); err != nil {
		// Clean up the stored file on error
		_ = s.storage.Delete(ctx, pdfPath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to store job payload",
		})
	}

	// Set initial status in Redis
	statusKey := fmt.Sprintf("job:%d", job.ID)
	if err := s.db.Redis.Set(ctx, statusKey, models.StatusPending, 0).Err(); err != nil {
		// Clean up the stored file on error
		_ = s.storage.Delete(ctx, pdfPath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to set job status",
		})
	}

	// Send to Kafka
	jobBytes, _ := json.Marshal(job)
	msg := &sarama.ProducerMessage{
		Topic: s.cfg.Kafka.Topic,
		Value: sarama.StringEncoder(jobBytes),
	}
	if _, _, err := s.producer.SendMessage(msg); err != nil {
		// Clean up the stored file on error
		_ = s.storage.Delete(ctx, pdfPath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to queue job",
		})
	}

	// Schedule file cleanup after TTL
	go func() {
		time.Sleep(s.cfg.Storage.TTL)
		_ = s.storage.Delete(context.Background(), pdfPath)
	}()

	return c.JSON(fiber.Map{
		"job_id": job.ID,
		"status": job.Status,
	})
}

// validatePDFParsePayload validates the PDF parse job payload
func validatePDFParsePayload(payload *models.ParseDocumentPayload) error {
	// Validate PDF source
	if payload.PDFSource == "" {
		return fmt.Errorf("pdf_source is required")
	}

	// Determine and validate source type
	if strings.HasPrefix(payload.PDFSource, "http://") || strings.HasPrefix(payload.PDFSource, "https://") {
		// Validate URL
		if _, err := url.ParseRequestURI(payload.PDFSource); err != nil {
			return fmt.Errorf("invalid PDF URL")
		}
	} else {
		// Validate base64
		decoded, err := base64.StdEncoding.DecodeString(payload.PDFSource)
		if err != nil {
			return fmt.Errorf("invalid base64-encoded PDF data")
		}
		
		// Check file size
		if len(decoded) > maxPDFSize {
			return fmt.Errorf("PDF size exceeds maximum allowed size of 10MB")
		}

		// Validate PDF magic number
		if len(decoded) < 4 || string(decoded[:4]) != "%PDF" {
			return fmt.Errorf("invalid PDF format")
		}
	}

	// Validate expected schema
	if len(payload.ExpectedSchema) == 0 {
		return fmt.Errorf("expected_schema is required")
	}

	// Validate that expected_schema is valid JSON
	var js json.RawMessage
	if err := json.Unmarshal(payload.ExpectedSchema, &js); err != nil {
		return fmt.Errorf("invalid JSON schema")
	}

	// Validate webhook URL if provided
	if payload.WebhookURL != "" {
		if _, err := url.ParseRequestURI(payload.WebhookURL); err != nil {
			return fmt.Errorf("invalid webhook URL")
		}
	}

	return nil
} 