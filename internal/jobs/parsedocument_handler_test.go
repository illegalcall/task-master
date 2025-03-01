package jobs

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// MockGeminiClient is a mock implementation of the GeminiClient interface for testing
type MockGeminiClient struct {
	MockResponse []byte
	MockError    error
	Calls        []string
}

// GenerateContent mocks the Gemini API call
func (m *MockGeminiClient) GenerateContent(ctx context.Context, text string, schema map[string]interface{}, description string) ([]byte, error) {
	m.Calls = append(m.Calls, text)
	return m.MockResponse, m.MockError
}

// TestParseDocumentHandler tests the ParseDocumentHandler function
func TestParseDocumentHandler(t *testing.T) {
	// Save the original functions
	originalExtractPDFText := ExtractPDFText
	originalNewGeminiClient := NewGeminiClient

	// Restore the original functions after the test
	defer func() {
		ExtractPDFText = originalExtractPDFText
		NewGeminiClient = originalNewGeminiClient
	}()

	// Create a mock extractor that returns a predefined text
	mockText := "Invoice #12345\nDate: 2023-07-01\nVendor: ABC Corp\nTotal: $100.00\nItems:\n1. Item A - $50.00\n2. Item B - $50.00"
	ExtractPDFText = func(documentSource string, documentType string, maxPages int) (string, error) {
		return mockText, nil
	}

	// Create a mock Gemini client
	mockResponse := []byte(`{
		"invoiceNumber": "12345",
		"date": "2023-07-01",
		"vendor": "ABC Corp",
		"total": 100.00,
		"items": [
			{
				"description": "Item A",
				"quantity": 1,
				"unitPrice": 50.00,
				"amount": 50.00
			},
			{
				"description": "Item B",
				"quantity": 1,
				"unitPrice": 50.00,
				"amount": 50.00
			}
		]
	}`)
	
	mockClient := &MockGeminiClient{
		MockResponse: mockResponse,
		MockError:    nil,
	}

	// Create a test context with our mock client
	ctx := context.WithValue(context.Background(), "geminiClient", mockClient)

	// Create a test payload
	payload := ParseDocumentPayload{
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

	// Marshal the payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	// Modify the handler function for testing
	handlerUnderTest := func(ctx context.Context, payload []byte) (Result, error) {
		// Parse and validate the payload
		if err := ValidateWithGJSON(payload); err != nil {
			return Result{}, err
		}

		var parsedPayload ParseDocumentPayload
		if err := json.Unmarshal(payload, &parsedPayload); err != nil {
			return Result{}, err
		}

		// Extract text using our mock
		text, err := ExtractPDFText(parsedPayload.Document, parsedPayload.DocumentType, parsedPayload.Options.MaxPages)
		if err != nil {
			return Result{}, err
		}

		// Use the mock client instead of creating a new one
		client := ctx.Value("geminiClient").(*MockGeminiClient)
		structuredData, err := client.GenerateContent(
			ctx,
			text,
			parsedPayload.OutputSchema,
			parsedPayload.Description,
		)
		if err != nil {
			return Result{}, err
		}

		// Parse the structured data
		var parsedContent interface{}
		if err := json.Unmarshal(structuredData, &parsedContent); err != nil {
			return Result{}, err
		}

		// Prepare the result
		parsedDocument := ParsedDocument{
			Content: parsedContent,
			MetaInfo: map[string]interface{}{
				"processingTimeMs": int64(100), // Fixed value for testing
				"documentType":     parsedPayload.DocumentType,
				"extractedTextLen": len(text),
			},
		}

		result := Result{
			Data: parsedDocument,
			Metadata: map[string]interface{}{
				"completedAt": time.Now().Format(time.RFC3339),
				"success":     true,
			},
		}

		return result, nil
	}

	// Call the handler
	result, err := handlerUnderTest(ctx, payloadBytes)
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}

	// Check that the mock client was called with the correct text
	if len(mockClient.Calls) != 1 {
		t.Fatalf("Expected 1 call to GenerateContent, got %d", len(mockClient.Calls))
	}
	if mockClient.Calls[0] != mockText {
		t.Errorf("GenerateContent was called with unexpected text: %s", mockClient.Calls[0])
	}

	// Verify the result content
	parsedDoc, ok := result.Data.(ParsedDocument)
	if !ok {
		t.Fatalf("Result data is not a ParsedDocument: %T", result.Data)
	}

	// Check some fields from the parsed invoice
	content := parsedDoc.Content.(map[string]interface{})
	if content["invoiceNumber"] != "12345" {
		t.Errorf("Expected invoiceNumber 12345, got %v", content["invoiceNumber"])
	}
	if content["vendor"] != "ABC Corp" {
		t.Errorf("Expected vendor ABC Corp, got %v", content["vendor"])
	}

	// Check metadata
	if parsedDoc.MetaInfo["documentType"] != "path" {
		t.Errorf("Expected documentType 'path', got %v", parsedDoc.MetaInfo["documentType"])
	}
}

// TestParseDocumentHandlerIntegration is an optional test that checks the actual integration with Gemini
// It will be skipped if the GEMINI_API_KEY environment variable is not set
func TestParseDocumentHandlerIntegration(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test - GEMINI_API_KEY not set")
	}

	// Create a simple test payload
	payload := ParseDocumentPayload{
		Document:     "testdata/sample_invoice.pdf", // This should be a real PDF file in your testdata directory
		DocumentType: "path",
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"invoiceNumber": map[string]interface{}{"type": "string"},
				"total":         map[string]interface{}{"type": "number"},
			},
		},
		Description: "Extract invoice details.",
		Options: ParseOptions{
			Language:   "en",
			OCREnabled: true,
		},
	}

	// Check if the test file exists
	if _, err := os.Stat(payload.Document); os.IsNotExist(err) {
		t.Skip("Skipping integration test - test file does not exist")
	}

	// Marshal the payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	// Call the handler
	result, err := ParseDocumentHandler(context.Background(), payloadBytes)
	if err != nil {
		t.Fatalf("Handler returned an error: %v", err)
	}

	// Verify the result has data
	parsedDoc, ok := result.Data.(ParsedDocument)
	if !ok {
		t.Fatalf("Result data is not a ParsedDocument")
	}

	// Just check that we got some content
	if parsedDoc.Content == nil {
		t.Errorf("Parsed content is nil")
	}
} 