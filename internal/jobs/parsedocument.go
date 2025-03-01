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
	OutputSchema map[string]interface{} `json:"outputSchema"`
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
	if p.OutputSchema == nil || len(p.OutputSchema) == 0 {
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

// CreateSamplePayloads creates example payloads for testing
func CreateSamplePayloads() map[string]ParseDocumentPayload {
	samples := make(map[string]ParseDocumentPayload)

	// Sample 1: Invoice PDF
	samples["invoice"] = ParseDocumentPayload{
		Document:     "/path/to/invoice.pdf",
		DocumentType: "path",
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"invoiceNumber": map[string]interface{}{"type": "string"},
				"date":          map[string]interface{}{"type": "string"},
				"vendor":        map[string]interface{}{"type": "string"},
				"total":         map[string]interface{}{"type": "number"},
				"items": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"description": map[string]interface{}{"type": "string"},
							"quantity":    map[string]interface{}{"type": "number"},
							"unitPrice":   map[string]interface{}{"type": "number"},
							"amount":      map[string]interface{}{"type": "number"},
						},
					},
				},
			},
		},
		Description: "Extract invoice details including invoice number, date, vendor name, total amount, and line items.",
		Options: ParseOptions{
			Language:            "en",
			OCREnabled:          true,
			ConfidenceThreshold: 0.7,
		},
	}

	// Sample 2: Resume PDF
	samples["resume"] = ParseDocumentPayload{
		Document:     "https://example.com/resume.pdf",
		DocumentType: "url",
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":    map[string]interface{}{"type": "string"},
				"email":   map[string]interface{}{"type": "string"},
				"phone":   map[string]interface{}{"type": "string"},
				"summary": map[string]interface{}{"type": "string"},
				"experience": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"company":     map[string]interface{}{"type": "string"},
							"position":    map[string]interface{}{"type": "string"},
							"startDate":   map[string]interface{}{"type": "string"},
							"endDate":     map[string]interface{}{"type": "string"},
							"description": map[string]interface{}{"type": "string"},
						},
					},
				},
				"education": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"institution": map[string]interface{}{"type": "string"},
							"degree":      map[string]interface{}{"type": "string"},
							"year":        map[string]interface{}{"type": "string"},
						},
					},
				},
				"skills": map[string]interface{}{
					"type":  "array",
					"items": map[string]interface{}{"type": "string"},
				},
			},
		},
		Description: "Extract candidate information from resume including personal details, work experience, education, and skills.",
		Options: ParseOptions{
			Language:   "en",
			OCREnabled: false,
		},
	}

	// Sample 3: Contract PDF
	samples["contract"] = ParseDocumentPayload{
		Document:     "base64encodedpdfcontent...",
		DocumentType: "base64",
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"contractTitle":    map[string]interface{}{"type": "string"},
				"parties":          map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"effectiveDate":    map[string]interface{}{"type": "string"},
				"terminationDate":  map[string]interface{}{"type": "string"},
				"paymentTerms":     map[string]interface{}{"type": "string"},
				"deliverables":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"specialClauses":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"confidentiality":  map[string]interface{}{"type": "string"},
				"disputeHandling":  map[string]interface{}{"type": "string"},
				"governingLaw":     map[string]interface{}{"type": "string"},
				"signatoryDetails": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			},
		},
		Description: "Extract key contract terms and conditions including parties, effective dates, payment terms, deliverables, and special clauses.",
		Options: ParseOptions{
			Language:            "en",
			OCREnabled:          true,
			ConfidenceThreshold: 0.8,
			MaxPages:            10,
		},
	}

	return samples
} 