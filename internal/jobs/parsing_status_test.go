package jobs

import (
	"errors"
	"testing"
	"time"
)

func TestParsingTracker_UpdateStatus(t *testing.T) {
	// Create a parsing tracker with a mock webhook client
	mockWebhook := &MockWebhookClient{
		Calls: []WebhookCall{}, // Initialize with empty slice to avoid nil reference
	}
	config := DefaultParsingTrackerConfig()
	config.WebhookEnabled = true
	config.WebhookURL = "http://example.com/webhook"
	
	tracker := NewParsingTracker(config)
	
	// Important: Replace the default webhook client with our mock
	tracker.webhookClient = mockWebhook
	
	// Create a subscription channel
	statusCh := make(chan ParsingStatusUpdate, 10)
	tracker.Subscribe(statusCh)
	
	// Test initial status update
	docID := "doc123"
	tracker.UpdateStatus(docID, StatusUploaded, nil)
	
	// Wait a moment for the async webhook call to happen
	time.Sleep(100 * time.Millisecond)
	
	// Check that the status was stored
	status, err := tracker.GetStatus(docID)
	if err != nil {
		t.Errorf("Failed to get status: %v", err)
	}
	if status.Status != StatusUploaded {
		t.Errorf("Expected status %s, got %s", StatusUploaded, status.Status)
	}
	
	// Check webhook was called
	if len(mockWebhook.Calls) != 1 {
		t.Errorf("Expected 1 webhook call, got %d", len(mockWebhook.Calls))
	}
	
	// Check subscriber was notified
	select {
	case update := <-statusCh:
		if update.DocumentID != docID || update.Status != StatusUploaded {
			t.Errorf("Unexpected update: %+v", update)
		}
	case <-time.After(time.Second):
		t.Error("Timed out waiting for status update")
	}
	
	// Update status with an error
	testError := errors.New("test error")
	tracker.UpdateStatus(docID, StatusFailed, testError)
	
	// Check error was stored
	status, _ = tracker.GetStatus(docID)
	if status.Error != testError.Error() {
		t.Errorf("Expected error %s, got %s", testError.Error(), status.Error)
	}
	
	// Test metrics calculation
	tracker.UpdateStatus("doc456", StatusUploaded, nil)
	tracker.UpdateStatus("doc456", StatusComplete, nil)
	
	metrics := tracker.GetMetrics()
	if metrics.TotalCount != 2 {
		t.Errorf("Expected total count 2, got %d", metrics.TotalCount)
	}
	if metrics.SuccessCount != 1 {
		t.Errorf("Expected success count 1, got %d", metrics.SuccessCount)
	}
	if metrics.FailureCount != 1 {
		t.Errorf("Expected failure count 1, got %d", metrics.FailureCount)
	}
}

func TestParsingTracker_ShouldRetry(t *testing.T) {
	config := DefaultParsingTrackerConfig()
	config.MaxRetries = 2
	tracker := NewParsingTracker(config)
	
	docID := "doc123"
	
	// Initial failure
	tracker.UpdateStatus(docID, StatusFailed, errors.New("test error"))
	
	// Should retry the first time
	if !tracker.ShouldRetry(docID) {
		t.Error("Expected ShouldRetry to return true for first retry")
	}
	
	// Mark as retrying
	tracker.UpdateStatus(docID, StatusRetrying, nil)
	// Fail again
	tracker.UpdateStatus(docID, StatusFailed, errors.New("test error"))
	
	// Should retry the second time
	if !tracker.ShouldRetry(docID) {
		t.Error("Expected ShouldRetry to return true for second retry")
	}
	
	// Mark as retrying
	tracker.UpdateStatus(docID, StatusRetrying, nil)
	// Fail again
	tracker.UpdateStatus(docID, StatusFailed, errors.New("test error"))
	
	// Should not retry after max retries
	if tracker.ShouldRetry(docID) {
		t.Error("Expected ShouldRetry to return false after max retries")
	}
}

func TestParsingTracker_SubscribeAndUnsubscribe(t *testing.T) {
	tracker := NewParsingTracker(DefaultParsingTrackerConfig())
	
	// Create two subscription channels
	ch1 := make(chan ParsingStatusUpdate, 1)
	ch2 := make(chan ParsingStatusUpdate, 1)
	
	// Subscribe both
	tracker.Subscribe(ch1)
	tracker.Subscribe(ch2)
	
	// Update status
	tracker.UpdateStatus("doc123", StatusUploaded, nil)
	
	// Both channels should receive the update
	select {
	case <-ch1:
		// Update received
	case <-time.After(time.Second):
		t.Error("Channel 1 didn't receive update")
	}
	
	select {
	case <-ch2:
		// Update received
	case <-time.After(time.Second):
		t.Error("Channel 2 didn't receive update")
	}
	
	// Unsubscribe one channel
	tracker.Unsubscribe(ch1)
	
	// Update status again
	tracker.UpdateStatus("doc123", StatusParsing, nil)
	
	// Channel 2 should receive the update, but not channel 1
	select {
	case <-ch1:
		t.Error("Channel 1 received update after unsubscribe")
	case <-time.After(100 * time.Millisecond):
		// Expected timeout
	}
	
	select {
	case <-ch2:
		// Update received
	case <-time.After(time.Second):
		t.Error("Channel 2 didn't receive update")
	}
} 