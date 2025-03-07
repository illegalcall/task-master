package models

import (
	"encoding/json"
	"time"
)

type Job struct {
	ID        int       `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Status    string    `json:"status" db:"status"`
	Type      string    `json:"type" db:"type"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	Response  string    `json:"response" db:"response"`
}

// pdf parsing job
type PDFParsingJob struct {
	Job
	Data NewParseDocumentPayload `json:"data"`
}

const (
	StatusPending   = "pending"
	StatusFailed    = "failed"
	StatusCompleted = "completed"
	JobTypePDFParse = "pdf_parse"
)

type Result struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type JobHandlerFunc func(payload []byte) (Result, error)

// ParseDocumentPayload represents the payload for PDF parsing jobs
type ParseDocumentPayload struct {
	PDFSource      string          `json:"pdf_source" validate:"required"`      // URL or base64-encoded PDF data
	ExpectedSchema json.RawMessage `json:"expected_schema" validate:"required"` // JSON schema for desired output
	Description    string          `json:"description"`                         // Additional context for parsing
	Options        struct {
		Language      string `json:"language,omitempty"`       // Optional language specification
		ParsingMethod string `json:"parsing_method,omitempty"` // Optional parsing method preference
	} `json:"options,omitempty"`
	WebhookURL string `json:"webhook_url,omitempty" validate:"omitempty,url"` // Optional webhook for notifications
}

type NewParseDocumentPayload struct {
	PDFSource      string `json:"pdf_source" validate:"required"`      // URL or base64-encoded PDF data
	ExpectedSchema string `json:"expected_schema" validate:"required"` // JSON schema for desired output
	Name           string `json:"name" validate:"required"`
	Description    string `json:"description" validate:"required"`
}

func (n NewParseDocumentPayload) JSON() any {
	panic("unimplemented")
}

// PDFSourceType indicates the type of PDF source provided
const (
	PDFSourceTypeURL    = "url"
	PDFSourceTypeBase64 = "base64"
)
