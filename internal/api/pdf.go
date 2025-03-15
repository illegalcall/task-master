package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
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

	// Parse the request payload
	var payload models.NewParseDocumentPayload
	if err := c.BodyParser(&payload); err != nil {
		fmt.Println("Error parsing request body:", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	fmt.Println("Payload parsed successfully:", payload)

	// Validate required fields
	if err := validatePDFParsePayload(&payload); err != nil {
		fmt.Println("Payload validation failed:", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	fmt.Println("Payload validation successful")

	// Store the PDF file
	var pdfPath string
	var err error
	if strings.HasPrefix(payload.PDFSource, "http://") || strings.HasPrefix(payload.PDFSource, "https://") {
		fmt.Println("Storing PDF from URL:", payload.PDFSource)
		pdfPath, err = s.storage.StoreFromURL(ctx, payload.PDFSource)
	} else {
		fmt.Println("Storing PDF from base64 data")
		pdfData, err := base64.StdEncoding.DecodeString(payload.PDFSource)
		if err != nil {
			fmt.Println("Error decoding base64 PDF data:", err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid base64-encoded PDF data",
			})
		}
		pdfPath, err = s.storage.StoreFromBytes(ctx, pdfData)
	}
	if err != nil {
		fmt.Println("Failed to store PDF:", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to store PDF: %v", err),
		})
	}
	fmt.Println("PDF stored successfully at path:", pdfPath)

	// Create a new job
	basicJob := models.Job{
		Name:   payload.Name,
		Status: models.StatusPending,
		Type:   models.JobTypePDFParse,
	}
	job := models.PDFParsingJob{
		Job:  basicJob,
		Data: payload,
	}
	fmt.Println("Job created:", job)
	fmt.Println("job.Data:", job.Data)
	fmt.Println("job.data.type:", reflect.TypeOf(job.Data))
	// Insert job into the database
	// Marshal the job payload to JSON
	payloadBytes, err := json.Marshal(job.Data)
	if err != nil {
		fmt.Println("Failed to marshal job payload:", err)
		_ = s.storage.Delete(ctx, pdfPath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create job due to payload marshalling error",
		})
	}
	fmt.Println("Job payload marshalled to JSON successfully")

	// Insert job into the database
	err = s.db.DB.QueryRow(
		"INSERT INTO jobs (name, status, created_at, type, payload) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		job.Job.Name, job.Job.Status, time.Now(), job.Job.Type, payloadBytes,
	).Scan(&job.ID)
	if err != nil {
		fmt.Println("Failed to insert job into database:", err)
		_ = s.storage.Delete(ctx, pdfPath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create job because of db error",
		})
	}
	fmt.Println("Job inserted into database with ID:", job.ID)

	// Store job payload in Redis
	jobPayload := struct {
		models.NewParseDocumentPayload
		PDFPath string `json:"pdf_path"`
	}{
		NewParseDocumentPayload: payload,
		PDFPath:                 pdfPath,
	}
	payloadBytes_2, _ := json.Marshal(jobPayload)
	redisKey := fmt.Sprintf("job:%d:payload", job.ID)
	if err := s.db.Redis.Set(ctx, redisKey, payloadBytes_2, s.cfg.Storage.TTL).Err(); err != nil {
		fmt.Println("Failed to store job payload in Redis:", err)
		_ = s.storage.Delete(ctx, pdfPath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to store job payload",
		})
	}
	fmt.Println("Job payload stored in Redis under key:", redisKey)

	// Set initial status in Redis
	statusKey := fmt.Sprintf("job:%d", job.ID)
	if err := s.db.Redis.Set(ctx, statusKey, models.StatusPending, 0).Err(); err != nil {
		fmt.Println("Failed to set job status in Redis:", err)
		_ = s.storage.Delete(ctx, pdfPath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to set job status",
		})
	}
	fmt.Println("Job status set in Redis under key:", statusKey)

	// Send to Kafka
	fmt.Println("Sending job to Kafka:", job)
	jobBytes, _ := json.Marshal(job)
	msg := &sarama.ProducerMessage{
		Topic: s.cfg.Kafka.Topic,
		Value: sarama.StringEncoder(jobBytes),
	}
	if _, _, err := s.producer.SendMessage(msg); err != nil {
		fmt.Println("Failed to queue job to Kafka:", err)
		_ = s.storage.Delete(ctx, pdfPath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to queue job",
		})
	}
	fmt.Println("Job queued successfully to Kafka topic:", s.cfg.Kafka.Topic)

	// Schedule file cleanup after TTL
	fmt.Println("Scheduling file cleanup for PDF path:", pdfPath, "after TTL:", s.cfg.Storage.TTL)
	go func() {
		time.Sleep(s.cfg.Storage.TTL)
		_ = s.storage.Delete(context.Background(), pdfPath)
		fmt.Println("Executed file cleanup for PDF path:", pdfPath)
	}()

	fmt.Println("Job processing completed for job ID:", job.ID)
	return c.JSON(fiber.Map{
		"job_id": job.ID,
		"status": job.Status,
	})
}

// validatePDFParsePayload validates the PDF parse job payload
func validatePDFParsePayload(payload *models.NewParseDocumentPayload) error {
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
	if err := json.Unmarshal([]byte(payload.ExpectedSchema), &js); err != nil {
		return fmt.Errorf("invalid JSON schema: %v", err)
	}

	return nil
}
