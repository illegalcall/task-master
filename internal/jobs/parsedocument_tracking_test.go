package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestParseDocumentWithTracking(t *testing.T) {
	// Save the original functions
	originalExtractPDFText := ExtractPDFText
	originalNewGeminiClient := NewGeminiClient
	originalTracker := globalTracker

	// Restore the original functions after the test
	defer func() {
		ExtractPDFText = originalExtractPDFText
		NewGeminiClient = originalNewGeminiClient
		globalTracker = originalTracker
	}()

	// Create a mock tracker
	mockWebhook := &MockWebhookClient{}
	config := DefaultParsingTrackerConfig()
	config.WebhookEnabled = true
	config.WebhookURL = "http://example.com/webhook"
	config.MaxRetries = 1

	tracker := NewParsingTracker(config)
	tracker.webhookClient = mockWebhook
	globalTracker = tracker

	// Create a status channel to track updates
	statusCh := make(chan ParsingStatusUpdate, 10)
	tracker.Subscribe(statusCh)

	// Mock extraction to fail once then succeed
	extractionAttempts := 0
	ExtractPDFText = func(documentSource string, documentType string, maxPages int) (string, error) {
		extractionAttempts++
		if extractionAttempts == 1 {
			return "", &MockError{message: "simulated extraction failure"}
		}
		return "Mock document text", nil
	}

	// Create a mock Gemini client with sample response
	mockClient := &MockGeminiClient{
		MockResponse: []byte(`{"field1": "value1", "field2": 42}`),
	}

	// Setup the client creation function to return a client that uses our mock
	NewGeminiClient = func(ctx context.Context) (*HTTPGeminiClient, error) {
		// Use the mock client through a custom closure that proxies to it
		return &HTTPGeminiClient{
			apiKey: "test-key",
			// Override the client's GenerateContent method to use our mock
			generateContentFunc: func(ctx context.Context, text string, schema map[string]interface{}, description string) ([]byte, error) {
				return mockClient.GenerateContent(ctx, text, schema, description)
			},
		}, nil
	}

	// Create a test payload with document ID
	payload := map[string]interface{}{
		"documentID":    "test-doc-123",
		"document":      "test-document.pdf",
		"documentType":  "path",
		"outputSchema":  map[string]interface{}{"type": "object"},
		"description":   "Test document",
		"options":       map[string]interface{}{"language": "en"},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	// Process with tracking - should fail first, then retry and succeed
	result, err := ParseDocumentWithTracking(context.Background(), payloadBytes)
	
	// Should eventually succeed
	if err != nil {
		t.Fatalf("Expected success after retry, got error: %v", err)
	}

	// Verify we got a result
	if result.Data == nil {
		t.Fatal("Expected result data, got nil")
	}

	// Check that extractPDFText was called twice (initial failure + retry)
	if extractionAttempts != 2 {
		t.Errorf("Expected 2 extraction attempts, got %d", extractionAttempts)
	}

	// Verify the mock client was used by checking if it was called
	if len(mockClient.Calls) == 0 {
		t.Error("Expected mock client to be called, but it wasn't")
	}

	// Check status progression from the channel
	statuses := []DocumentStatus{}
	timeoutCh := time.After(2 * time.Second) // Increase timeout to 2 seconds for more reliable testing

collectStatuses:
	for {
		select {
		case update := <-statusCh:
			fmt.Printf("Received status update: %s\n", update.Status) // Add debug output
			statuses = append(statuses, update.Status)
		case <-timeoutCh:
			break collectStatuses
		default:
			if len(statuses) >= 7 { // We expect 7 status updates
				break collectStatuses
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Check that we got the expected status transitions
	expectedStatuses := []DocumentStatus{
		StatusUploaded,
		StatusParsing,
		StatusFailed,
		StatusRetrying,
		StatusParsing,
		StatusConverting,
		StatusComplete,
	}

	// Check that each expected status appears in our collected statuses
	// (Not checking exact order since some status changes might be missed in the channel)
	for _, expectedStatus := range expectedStatuses {
		found := false
		for _, actualStatus := range statuses {
			if actualStatus == expectedStatus {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected status %s not found in status updates", expectedStatus)
		}
	}

	// Check webhook calls
	if len(mockWebhook.Calls) == 0 {
		t.Error("Expected webhook calls, got none")
	}

	// Check metrics
	metrics := tracker.GetMetrics()
	if metrics.TotalCount != 1 {
		t.Errorf("Expected total count 1, got %d", metrics.TotalCount)
	}
	if metrics.RetryCount < 1 {
		t.Errorf("Expected retry count at least 1, got %d", metrics.RetryCount)
	}
	if metrics.SuccessCount != 1 {
		t.Errorf("Expected success count 1, got %d", metrics.SuccessCount)
	}
}

// MockError is a simple error implementation for testing
type MockError struct {
	message string
}

func (e *MockError) Error() string {
	return e.message
} 