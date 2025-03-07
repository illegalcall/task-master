package jobs

import (
	"errors"
	"fmt"

	"github.com/tidwall/gjson"
)

// ParseDocumentPayload defines the structure for document parsing jobs
type ParseDocumentPayload struct {
	// Document represents a PDF document source - can be a file path, URL, or base64 encoded content
	Document string `json:"document"`
	// DocumentType indicates the source type ("path", "url", or "base64")
	DocumentType string `json:"documentType"`
	// OutputSchema defines the expected JSON structure for the parsed result
	OutputSchema string`json:"expected_schema"`
	// Description provides additional context to guide the LLM during parsing
	Description string `json:"description"`
	// Options contains optional parsing parameters
	Options ParseOptions `json:"options,omitempty"`
}

// ParseOptions contains optional configuration for document parsing
type ParseOptions struct {
	// Language specifies the expected language of the document
	Language string `json:"language,omitempty"`
	// OCREnabled determines whether to use OCR for image-based PDFs
	OCREnabled bool `json:"ocrEnabled,omitempty"`
	// ConfidenceThreshold sets minimum confidence level for extracted fields (0.0-1.0)
	ConfidenceThreshold float64 `json:"confidenceThreshold,omitempty"`
	// MaxPages limits processing to the first N pages (0 = all pages)
	MaxPages int `json:"maxPages,omitempty"`
}

// Validate checks if the ParseDocumentPayload is valid
func (p *ParseDocumentPayload) Validate() error {
	// Check required fields
	if p.Document == "" {
		return errors.New("document is required")
	}

	// Validate document type
	validTypes := map[string]bool{
		"path":   true,
		"url":    true,
		"base64": true,
	}
	if !validTypes[p.DocumentType] {
		return fmt.Errorf("documentType must be one of: path, url, base64")
	}

	// Validate output schema
	if p.OutputSchema == "" || len(p.OutputSchema) == 0 {
		return errors.New("outputSchema is required")
	}

	// Validate options if provided
	if p.Options.ConfidenceThreshold < 0 || p.Options.ConfidenceThreshold > 1 {
		return errors.New("confidenceThreshold must be between 0.0 and 1.0")
	}

	return nil
}

// ValidateWithGJSON performs validation using gjson
func ValidateWithGJSON(payload []byte) error {
	if !gjson.ValidBytes(payload) {
		return errors.New("invalid JSON payload")
	}

	// Parse the JSON payload
	data := gjson.ParseBytes(payload)

	// Validate required fields
	requiredFields := []string{"document", "documentType", "outputSchema"}
	for _, field := range requiredFields {
		if !data.Get(field).Exists() {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	// Validate document field
	document := data.Get("document").String()
	if document == "" {
		return errors.New("document must not be empty")
	}

	// Validate documentType field
	documentType := data.Get("documentType").String()
	validTypes := map[string]bool{
		"path":   true,
		"url":    true,
		"base64": true,
	}
	if !validTypes[documentType] {
		return fmt.Errorf("documentType must be one of: path, url, base64")
	}

	// Validate outputSchema
	outputSchema := data.Get("outputSchema")
	if !outputSchema.IsObject() || len(outputSchema.Map()) == 0 {
		return errors.New("outputSchema must be a non-empty object")
	}

	// Validate options if present
	if data.Get("options").Exists() {
		// Validate confidenceThreshold if present
		if data.Get("options.confidenceThreshold").Exists() {
			confidenceThreshold := data.Get("options.confidenceThreshold").Float()
			if confidenceThreshold < 0 || confidenceThreshold > 1 {
				return errors.New("confidenceThreshold must be between 0.0 and 1.0")
			}
		}

		// Validate maxPages if present
		if data.Get("options.maxPages").Exists() {
			maxPages := data.Get("options.maxPages").Int()
			if maxPages < 0 {
				return errors.New("maxPages must be a non-negative integer")
			}
		}
	}

	return nil
}