package jobs

import (
	"fmt"
	"sync"
	"time"
)

// DocumentStatus represents the current status of a document parsing job
type DocumentStatus string

const (
	// StatusUploaded indicates the document was successfully uploaded and is waiting to be processed
	StatusUploaded DocumentStatus = "uploaded"
	// StatusParsing indicates the document text extraction is in progress
	StatusParsing DocumentStatus = "parsing"
	// StatusConverting indicates the extracted text is being converted to structured data by the LLM
	StatusConverting DocumentStatus = "converting"
	// StatusComplete indicates the document was successfully parsed and converted
	StatusComplete DocumentStatus = "complete"
	// StatusFailed indicates the document parsing failed
	StatusFailed DocumentStatus = "failed"
	// StatusRetrying indicates a failed step is being retried
	StatusRetrying DocumentStatus = "retrying"
)

// DocumentParsingMetrics tracks various metrics about document parsing
type DocumentParsingMetrics struct {
	// TotalCount is the total number of documents processed
	TotalCount int `json:"totalCount"`
	// SuccessCount is the number of documents successfully processed
	SuccessCount int `json:"successCount"`
	// FailureCount is the number of documents that failed processing
	FailureCount int `json:"failureCount"`
	// RetryCount is the number of times processing was retried
	RetryCount int `json:"retryCount"`
	// AverageProcessingTimeMs is the average time to process a document in milliseconds
	AverageProcessingTimeMs int64 `json:"averageProcessingTimeMs"`
	// TotalProcessingTimeMs is the total time spent processing documents in milliseconds
	TotalProcessingTimeMs int64 `json:"totalProcessingTimeMs"`
}

// ParsingStatusUpdate represents a change in the document parsing status
type ParsingStatusUpdate struct {
	// DocumentID is the unique identifier for the document
	DocumentID string `json:"documentID"`
	// Status is the new status of the document
	Status DocumentStatus `json:"status"`
	// Error is any error message associated with the status update
	Error string `json:"error,omitempty"`
	// Timestamp is when the status update occurred
	Timestamp time.Time `json:"timestamp"`
	// Progress is an optional field for progress updates (0-100)
	Progress int `json:"progress,omitempty"`
	// RetryCount indicates how many times processing has been retried
	RetryCount int `json:"retryCount,omitempty"`
}

// ParsingTrackerConfig holds configuration for the parsing tracker
type ParsingTrackerConfig struct {
	// MaxRetries is the maximum number of retries for a failed document
	MaxRetries int
	// WebhookURL is the endpoint to notify about status changes
	WebhookURL string
	// WebhookEnabled determines whether to send webhook notifications
	WebhookEnabled bool
}

// DefaultParsingTrackerConfig returns a default configuration
func DefaultParsingTrackerConfig() ParsingTrackerConfig {
	return ParsingTrackerConfig{
		MaxRetries:     3,
		WebhookEnabled: false,
	}
}

// ParsingTracker is responsible for tracking and reporting document parsing status
type ParsingTracker struct {
	// statuses stores the current status of all documents
	statuses map[string]ParsingStatusUpdate
	// webhookClient is responsible for sending webhook notifications
	webhookClient WebhookClient
	// metrics tracks overall parsing metrics
	metrics DocumentParsingMetrics
	// config holds the configuration for the tracker
	config ParsingTrackerConfig
	// statusSubscribers are channels that receive status updates
	statusSubscribers []chan<- ParsingStatusUpdate
	// mutex protects concurrent access to the tracker's state
	mutex sync.RWMutex
}

// NewParsingTracker creates a new instance of ParsingTracker
func NewParsingTracker(config ParsingTrackerConfig) *ParsingTracker {
	var webhookClient WebhookClient
	if config.WebhookEnabled {
		webhookClient = &HTTPWebhookClient{}
	} else {
		// Use a no-op client when webhooks are disabled
		webhookClient = &noopWebhookClient{}
	}

	return &ParsingTracker{
		statuses:     make(map[string]ParsingStatusUpdate),
		webhookClient: webhookClient,
		metrics:     DocumentParsingMetrics{},
		config:      config,
	}
}

// noopWebhookClient is a webhook client that does nothing (for when webhooks are disabled)
type noopWebhookClient struct{}

func (c *noopWebhookClient) Send(url string, data interface{}) error {
	return nil
}

// UpdateStatus updates the status of a document
func (t *ParsingTracker) UpdateStatus(documentID string, status DocumentStatus, err error) {
	t.mutex.Lock()
	
	// Create the status update
	update := ParsingStatusUpdate{
		DocumentID: documentID,
		Status:     status,
		Timestamp:  time.Now(),
	}
	
	// Add error message if present
	if err != nil {
		update.Error = err.Error()
	}
	
	// Update retry count if we're retrying
	prevStatus, exists := t.statuses[documentID]
	if status == StatusRetrying && exists {
		update.RetryCount = prevStatus.RetryCount + 1
	} else if exists {
		update.RetryCount = prevStatus.RetryCount
	}
	
	// Store the status
	t.statuses[documentID] = update
	
	// Update metrics based on the new status
	t.updateMetrics(update)
	
	// Get a local copy of subscribers to avoid holding the lock during notifications
	subscribers := make([]chan<- ParsingStatusUpdate, len(t.statusSubscribers))
	copy(subscribers, t.statusSubscribers)
	
	t.mutex.Unlock()
	
	// Send webhook notification if enabled
	if t.config.WebhookEnabled && t.webhookClient != nil && t.config.WebhookURL != "" {
		go func() {
			t.webhookClient.Send(t.config.WebhookURL, update)
		}()
	}
	
	// Notify subscribers
	for _, ch := range subscribers {
		select {
		case ch <- update:
			// Status update sent successfully
		default:
			// Channel is not ready to receive, we'll skip it
		}
	}
}

// GetStatus returns the current status of a document
func (t *ParsingTracker) GetStatus(documentID string) (ParsingStatusUpdate, error) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	status, exists := t.statuses[documentID]
	if !exists {
		return ParsingStatusUpdate{}, fmt.Errorf("no status found for document %s", documentID)
	}

	return status, nil
}

// ShouldRetry determines if a failed document should be retried
func (t *ParsingTracker) ShouldRetry(documentID string) bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	status, exists := t.statuses[documentID]
	if !exists {
		return false
	}

	// Only retry if the document is in failed status and hasn't exceeded max retries
	return status.Status == StatusFailed && status.RetryCount < t.config.MaxRetries
}

// GetMetrics returns the current metrics
func (t *ParsingTracker) GetMetrics() DocumentParsingMetrics {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return t.metrics
}

// Subscribe adds a channel to receive status updates
func (t *ParsingTracker) Subscribe(ch chan<- ParsingStatusUpdate) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.statusSubscribers = append(t.statusSubscribers, ch)
}

// Unsubscribe removes a channel from receiving status updates
func (t *ParsingTracker) Unsubscribe(ch chan<- ParsingStatusUpdate) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	for i, subscriber := range t.statusSubscribers {
		if subscriber == ch {
			t.statusSubscribers = append(t.statusSubscribers[:i], t.statusSubscribers[i+1:]...)
			return
		}
	}
}

// updateMetrics updates the parsing metrics based on a status update
func (t *ParsingTracker) updateMetrics(update ParsingStatusUpdate) {
	// Update total count for new documents
	prevStatus, exists := t.statuses[update.DocumentID]
	if !exists || prevStatus.Status == StatusUploaded {
		t.metrics.TotalCount++
	}

	// Update success/failure counts
	if update.Status == StatusComplete {
		t.metrics.SuccessCount++
	} else if update.Status == StatusFailed {
		t.metrics.FailureCount++
	}

	// Update retry count
	if update.Status == StatusRetrying {
		t.metrics.RetryCount++
	}

	// Update processing time for completed documents
	if update.Status == StatusComplete && exists {
		processingTime := update.Timestamp.Sub(prevStatus.Timestamp).Milliseconds()
		t.metrics.TotalProcessingTimeMs += processingTime
		
		// Recalculate average
		if t.metrics.SuccessCount > 0 {
			t.metrics.AverageProcessingTimeMs = t.metrics.TotalProcessingTimeMs / int64(t.metrics.SuccessCount)
		}
	}
}

// notifySubscribers sends the status update to all subscribers
func (t *ParsingTracker) notifySubscribers(update ParsingStatusUpdate) {
	for _, ch := range t.statusSubscribers {
		select {
		case ch <- update:
			// Status update sent successfully
		default:
			// Channel is not ready to receive, we'll skip it
		}
	}
} 