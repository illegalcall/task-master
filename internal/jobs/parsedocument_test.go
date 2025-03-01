package jobs

import (
	"encoding/json"
	"testing"
)

func TestParseDocumentPayloadValidation(t *testing.T) {
	// Test valid payload
	validPayload := ParseDocumentPayload{
		Document:     "/path/to/document.pdf",
		DocumentType: "path",
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"field1": map[string]interface{}{"type": "string"},
			},
		},
		Description: "Test description",
	}

	if err := validPayload.Validate(); err != nil {
		t.Errorf("Expected valid payload to pass validation, got error: %v", err)
	}

	// Test missing document
	invalidPayload := validPayload
	invalidPayload.Document = ""
	if err := invalidPayload.Validate(); err == nil {
		t.Error("Expected error for missing document, got nil")
	}

	// Test invalid document type
	invalidPayload = validPayload
	invalidPayload.DocumentType = "invalid"
	if err := invalidPayload.Validate(); err == nil {
		t.Error("Expected error for invalid document type, got nil")
	}

	// Test missing output schema
	invalidPayload = validPayload
	invalidPayload.OutputSchema = nil
	if err := invalidPayload.Validate(); err == nil {
		t.Error("Expected error for missing output schema, got nil")
	}

	// Test invalid confidence threshold
	invalidPayload = validPayload
	invalidPayload.Options.ConfidenceThreshold = 1.5
	if err := invalidPayload.Validate(); err == nil {
		t.Error("Expected error for invalid confidence threshold, got nil")
	}
}

func TestGJSONValidation(t *testing.T) {
	// Test valid payload
	validPayload := ParseDocumentPayload{
		Document:     "/path/to/document.pdf",
		DocumentType: "path",
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"field1": map[string]interface{}{"type": "string"},
			},
		},
		Description: "Test description",
		Options: ParseOptions{
			Language:            "en",
			OCREnabled:          true,
			ConfidenceThreshold: 0.7,
		},
	}

	payloadBytes, err := json.Marshal(validPayload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	if err := ValidateWithGJSON(payloadBytes); err != nil {
		t.Errorf("Expected valid payload to pass validation, got error: %v", err)
	}

	// Test with invalid document type
	invalidPayload := validPayload
	invalidPayload.DocumentType = "invalid"
	payloadBytes, _ = json.Marshal(invalidPayload)
	if err := ValidateWithGJSON(payloadBytes); err == nil {
		t.Error("Expected error for invalid document type in validation, got nil")
	}

	// Test with invalid confidence threshold
	invalidPayload = validPayload
	invalidPayload.Options.ConfidenceThreshold = 2.0
	payloadBytes, _ = json.Marshal(invalidPayload)
	if err := ValidateWithGJSON(payloadBytes); err == nil {
		t.Error("Expected error for invalid confidence threshold in validation, got nil")
	}

	// Test with missing required field
	invalidJSON := `{"documentType": "path", "outputSchema": {"type": "object"}}`
	if err := ValidateWithGJSON([]byte(invalidJSON)); err == nil {
		t.Error("Expected error for missing document field, got nil")
	}

	// Test with invalid JSON
	if err := ValidateWithGJSON([]byte("{")); err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestSamplePayloads(t *testing.T) {
	samples := CreateSamplePayloads()

	// Check if we have the expected samples
	expectedSamples := []string{"invoice", "resume", "contract"}
	for _, name := range expectedSamples {
		if _, exists := samples[name]; !exists {
			t.Errorf("Expected sample '%s' not found", name)
		}
	}

	// Validate each sample
	for name, payload := range samples {
		if err := payload.Validate(); err != nil {
			t.Errorf("Sample '%s' failed validation: %v", name, err)
		}

		// Test JSON serialization
		bytes, err := json.Marshal(payload)
		if err != nil {
			t.Errorf("Failed to marshal sample '%s': %v", name, err)
		}

		// Test GJSON validation
		if err := ValidateWithGJSON(bytes); err != nil {
			t.Errorf("Sample '%s' failed GJSON validation: %v", name, err)
		}

		// Test deserialization
		var deserializedPayload ParseDocumentPayload
		if err := json.Unmarshal(bytes, &deserializedPayload); err != nil {
			t.Errorf("Failed to unmarshal sample '%s': %v", name, err)
		}
	}
} 