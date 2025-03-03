package models

import (
	"encoding/json"
	"time"
)

// Job represents a processing job in the system
// @Description Job contains information about a processing task
type Job struct {
	// Unique identifier for the job
	ID        int       `json:"id" db:"id" example:"1"`
	// Name of the job
	Name      string    `json:"name" db:"name" example:"PDF Parse Job"`
	// Current status of the job
	Status    string    `json:"status" db:"status" example:"pending" enums:"pending,failed,completed"`
	// Type of job
	Type      string    `json:"type" db:"type" example:"pdf_parse"`
	// Time when the job was created
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

const (
	StatusPending   = "pending"
	StatusFailed    = "failed"
	StatusCompleted = "completed"
	JobTypePDFParse = "pdf_parse"
)

type Result struct {
	Message string `json:"message"`
	Data    interface{} `json:"data"`
}

type JobHandlerFunc func(payload []byte) (Result, error)

// ParseDocumentPayload represents the configuration for PDF parsing jobs
// @Description Configuration for PDF document parsing
type ParseDocumentPayload struct {
	// URL or base64-encoded PDF data
	PDFSource      string          `json:"pdf_source" validate:"required" example:"https://example.com/document.pdf"`
	// JSON schema for desired output
	ExpectedSchema json.RawMessage `json:"expected_schema" validate:"required"`
	// Additional context for parsing
	Description    string          `json:"description" example:"Extract invoice details"`
	// Optional configuration
	Options        struct {
		// Optional language specification
		Language       string `json:"language,omitempty" example:"en"`
		// Optional parsing method preference
		ParsingMethod  string `json:"parsing_method,omitempty" example:"ocr"`
	} `json:"options,omitempty"`
	// Optional webhook for notifications
	WebhookURL     string `json:"webhook_url,omitempty" validate:"omitempty,url" example:"https://example.com/webhook"`
}

// PDFSourceType indicates the type of PDF source provided
const (
	PDFSourceTypeURL    = "url"
	PDFSourceTypeBase64 = "base64"
)