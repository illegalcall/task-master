package jobs

import (
	"context"
)

// Result represents the outcome of a job execution
type Result struct {
	// Data contains the job result data
	Data interface{} `json:"data"`
	// Metadata contains additional information about the result
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// JobHandlerFunc defines the signature for job handler functions
type JobHandlerFunc func(ctx context.Context, payload []byte) (Result, error)

// ParsedDocument represents the structured data extracted from a document
type ParsedDocument struct {
	// Content contains the structured JSON data according to the provided schema
	Content interface{} `json:"content"`
	// MetaInfo contains additional information about the parsing process
	MetaInfo map[string]interface{} `json:"metaInfo,omitempty"`
}

