package jobs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// WebhookClient is an interface for sending webhook notifications
type WebhookClient interface {
	Send(url string, data interface{}) error
}

// HTTPWebhookClient implements WebhookClient using standard HTTP requests
type HTTPWebhookClient struct {
	client *http.Client
}

// Send sends a webhook notification to the specified URL
func (c *HTTPWebhookClient) Send(url string, data interface{}) error {
	// If the client hasn't been initialized, create it with default settings
	if c.client == nil {
		c.client = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	// Marshal the data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook data: %w", err)
	}

	// Create the request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook request failed with status %d", resp.StatusCode)
	}

	return nil
}

// MockWebhookClient is a mock implementation for testing
type MockWebhookClient struct {
	Calls []WebhookCall
	mu    sync.Mutex
}

// WebhookCall represents a call to the webhook client
type WebhookCall struct {
	URL  string
	Data interface{}
}

// Send records the webhook call for testing
func (m *MockWebhookClient) Send(url string, data interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.Calls = append(m.Calls, WebhookCall{
		URL:  url,
		Data: data,
	})
	return nil
} 