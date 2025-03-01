package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/IBM/sarama"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/illegalcall/task-master/internal/config"
	"github.com/illegalcall/task-master/internal/models"
)

// MockStorage implements storage.Storage interface for testing
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) StoreFromURL(ctx context.Context, url string) (string, error) {
	args := m.Called(ctx, url)
	return args.String(0), args.Error(1)
}

func (m *MockStorage) StoreFromBytes(ctx context.Context, data []byte) (string, error) {
	args := m.Called(ctx, data)
	return args.String(0), args.Error(1)
}

func (m *MockStorage) Delete(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func TestHandlePDFParseJob(t *testing.T) {
	// Setup test server with mocked storage
	app := fiber.New()
	mockStorage := &MockStorage{}
	
	cfg := &config.Config{
		Storage: config.StorageConfig{
			TempDir: os.TempDir(),
			MaxSize: 10 * 1024 * 1024,
			TTL:     time.Hour,
		},
	}

	app.Post("/api/jobs/parse-document", (&Server{
		app:     app,
		cfg:     cfg,
		storage: mockStorage,
		// Mock other dependencies as needed
	}).handlePDFParseJob)

	tests := []struct {
		name           string
		payload        models.ParseDocumentPayload
		setupMocks     func(*MockStorage)
		expectedStatus int
		expectError    bool
	}{
		{
			name: "Valid URL PDF Source",
			payload: models.ParseDocumentPayload{
				PDFSource:      "https://example.com/test.pdf",
				ExpectedSchema: json.RawMessage(`{"type": "object"}`),
				Description:    "Test PDF parsing",
				WebhookURL:    "https://webhook.example.com",
			},
			setupMocks: func(m *MockStorage) {
				m.On("StoreFromURL", mock.Anything, "https://example.com/test.pdf").
					Return("/tmp/test.pdf", nil)
				m.On("Delete", mock.Anything, "/tmp/test.pdf").
					Return(nil)
			},
			expectedStatus: fiber.StatusOK,
			expectError:    false,
		},
		{
			name: "Valid Base64 PDF Source",
			payload: models.ParseDocumentPayload{
				PDFSource:      base64.StdEncoding.EncodeToString([]byte("%PDF-1.4\n...")),
				ExpectedSchema: json.RawMessage(`{"type": "object"}`),
			},
			setupMocks: func(m *MockStorage) {
				m.On("StoreFromBytes", mock.Anything, mock.Anything).
					Return("/tmp/test.pdf", nil)
				m.On("Delete", mock.Anything, "/tmp/test.pdf").
					Return(nil)
			},
			expectedStatus: fiber.StatusOK,
			expectError:    false,
		},
		{
			name: "Storage Error",
			payload: models.ParseDocumentPayload{
				PDFSource:      "https://example.com/test.pdf",
				ExpectedSchema: json.RawMessage(`{"type": "object"}`),
			},
			setupMocks: func(m *MockStorage) {
				m.On("StoreFromURL", mock.Anything, mock.Anything).
					Return("", assert.AnError)
			},
			expectedStatus: fiber.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock and setup expectations
			mockStorage.ExpectedCalls = nil
			mockStorage.Calls = nil
			if tt.setupMocks != nil {
				tt.setupMocks(mockStorage)
			}

			// Convert payload to JSON
			payloadBytes, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			// Create request
			req := httptest.NewRequest("POST", "/api/jobs/parse-document", bytes.NewReader(payloadBytes))
			req.Header.Set("Content-Type", "application/json")

			// Execute request
			resp, err := app.Test(req)
			require.NoError(t, err)

			// Check status code
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Parse response
			var result map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)

			// Verify response structure
			if !tt.expectError {
				assert.Contains(t, result, "job_id")
				assert.Contains(t, result, "status")
				assert.Equal(t, models.StatusPending, result["status"])
			} else {
				assert.Contains(t, result, "error")
				assert.NotEmpty(t, result["error"])
			}

			// Verify all mock expectations were met
			mockStorage.AssertExpectations(t)
		})
	}
}

func TestValidatePDFParsePayload(t *testing.T) {
	tests := []struct {
		name        string
		payload     models.ParseDocumentPayload
		expectError bool
	}{
		{
			name: "Valid URL payload",
			payload: models.ParseDocumentPayload{
				PDFSource:      "https://example.com/test.pdf",
				ExpectedSchema: json.RawMessage(`{"type": "object"}`),
			},
			expectError: false,
		},
		{
			name: "Valid base64 payload",
			payload: models.ParseDocumentPayload{
				PDFSource:      base64.StdEncoding.EncodeToString([]byte("%PDF-1.4\ntest")),
				ExpectedSchema: json.RawMessage(`{"type": "object"}`),
			},
			expectError: false,
		},
		{
			name: "Empty PDF source",
			payload: models.ParseDocumentPayload{
				ExpectedSchema: json.RawMessage(`{"type": "object"}`),
			},
			expectError: true,
		},
		{
			name: "Empty schema",
			payload: models.ParseDocumentPayload{
				PDFSource: "https://example.com/test.pdf",
			},
			expectError: true,
		},
		{
			name: "Invalid schema JSON",
			payload: models.ParseDocumentPayload{
				PDFSource:      "https://example.com/test.pdf",
				ExpectedSchema: json.RawMessage(`{invalid`),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePDFParsePayload(&tt.payload)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseDocumentHandler(t *testing.T) {
	server, mock, miniRedis := setupTestServer(t)
	defer miniRedis.Close()

	// Test payload
	payload := models.ParseDocumentPayload{
		PDFSource:      "data:application/pdf;base64,JVBER...",
		ExpectedSchema: json.RawMessage(`{"field": "value"}`),
	}
	body, _ := json.Marshal(payload)

	// Expect job creation with PDF parse type
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO jobs (name, status, type) VALUES ($1, $2, $3) RETURNING id")).
		WithArgs("PDF Parse Job", models.StatusPending, models.JobTypePDFParse).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	// Create test request
	req := httptest.NewRequest("POST", "/api/jobs/parse-document", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Run the test
	resp, err := server.app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Verify response
	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), result["job_id"])
	assert.Equal(t, models.StatusPending, result["status"])

	// Verify Redis status
	redisVal, err := miniRedis.Get("job:1")
	assert.NoError(t, err)
	assert.Equal(t, models.StatusPending, redisVal)

	// Verify Redis payload
	payloadVal, err := miniRedis.Get("job:1:payload")
	assert.NoError(t, err)
	assert.NotEmpty(t, payloadVal)

	// Verify Kafka message
	mockProducer := server.producer.(*MockProducer)
	assert.Len(t, mockProducer.messages, 1)
	kafkaMsg := mockProducer.messages[0]
	assert.Equal(t, server.cfg.Kafka.Topic, kafkaMsg.Topic)

	var jobMsg models.Job
	err = json.Unmarshal([]byte(kafkaMsg.Value.(sarama.StringEncoder)), &jobMsg)
	assert.NoError(t, err)
	assert.Equal(t, 1, jobMsg.ID)
	assert.Equal(t, "PDF Parse Job", jobMsg.Name)
	assert.Equal(t, models.StatusPending, jobMsg.Status)
	assert.Equal(t, models.JobTypePDFParse, jobMsg.Type)

	// Verify mock expectations
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestParseDocumentHandlerIntegration(t *testing.T) {
	if os.Getenv("GEMINI_API_KEY") == "" {
		t.Skip("Skipping integration test: GEMINI_API_KEY not set")
	}

	server, mock, miniRedis := setupTestServer(t)
	defer miniRedis.Close()

	// Test payload with real PDF data
	payload := models.ParseDocumentPayload{
		PDFSource:      "https://example.com/test.pdf",
		ExpectedSchema: json.RawMessage(`{"field": "value"}`),
	}
	body, _ := json.Marshal(payload)

	// Expect job creation with PDF parse type
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO jobs (name, status, type) VALUES ($1, $2, $3) RETURNING id")).
		WithArgs("PDF Parse Job", models.StatusPending, models.JobTypePDFParse).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	// Create test request
	req := httptest.NewRequest("POST", "/api/jobs/parse-document", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Run the test
	resp, err := server.app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Verify response
	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, float64(1), result["job_id"])
	assert.Equal(t, models.StatusPending, result["status"])

	// Verify mock expectations
	assert.NoError(t, mock.ExpectationsWereMet())
} 