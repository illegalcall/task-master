package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/illegalcall/task-master/internal/config"
	"github.com/illegalcall/task-master/internal/models"
	"github.com/illegalcall/task-master/pkg/database"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockConsumerGroup mocks sarama.ConsumerGroup
type MockConsumerGroup struct {
	mock.Mock
}

func (m *MockConsumerGroup) Consume(ctx context.Context, topics []string, handler sarama.ConsumerGroupHandler) error {
	args := m.Called(ctx, topics, handler)
	return args.Error(0)
}

func (m *MockConsumerGroup) Errors() <-chan error {
	args := m.Called()
	return args.Get(0).(chan error)
}

func (m *MockConsumerGroup) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockConsumerGroup) Pause(partitions map[string][]int32) {
	m.Called(partitions)
}

func (m *MockConsumerGroup) Resume(partitions map[string][]int32) {
	m.Called(partitions)
}

func (m *MockConsumerGroup) PauseAll() {
	m.Called()
}

func (m *MockConsumerGroup) ResumeAll() {
	m.Called()
}

// setupTestWorker creates a test worker with mocked dependencies
func setupTestWorker(t *testing.T) (*Worker, sqlmock.Sqlmock, *miniredis.Miniredis, *MockConsumerGroup) {
	// Setup SQL mock
	sqlDB, mock, err := sqlmock.New()
	assert.NoError(t, err)

	// Convert to sqlx.DB
	db := sqlx.NewDb(sqlDB, "sqlmock")

	// Setup Redis mock
	miniRedis := miniredis.NewMiniRedis()
	err = miniRedis.Start()
	assert.NoError(t, err)

	redisClient := redis.NewClient(&redis.Options{
		Addr: miniRedis.Addr(),
	})

	// Create database clients
	dbClients := &database.Clients{
		DB:    db,
		Redis: redisClient,
	}

	// Setup config
	cfg := &config.Config{
		Kafka: config.KafkaConfig{
			Topic:          "test-topic",
			RetryMax:       3,
			RetryBackoff:   time.Millisecond,
			ProcessingTime: time.Millisecond,
		},
		Storage: config.StorageConfig{
			TTL: time.Hour,
		},
	}

	// Setup consumer group mock
	mockConsumerGroup := new(MockConsumerGroup)

	// Create worker
	worker := NewWorker(cfg, dbClients, mockConsumerGroup)

	return worker, mock, miniRedis, mockConsumerGroup
}

func TestProcessJob(t *testing.T) {
	worker, mock, miniRedis, _ := setupTestWorker(t)
	defer miniRedis.Close()

	// Test cases
	testCases := []struct {
		name        string
		jobType     string
		setupMocks  func()
		expectError bool
	}{
		{
			name:    "PDF Parse Job Success",
			jobType: models.JobTypePDFParse,
			setupMocks: func() {
				// Setup Redis payload
				payload := models.ParseDocumentPayload{
					PDFSource:      "test.pdf",
					ExpectedSchema: json.RawMessage(`{"field": "value"}`),
				}
				payloadBytes, _ := json.Marshal(payload)
				worker.db.Redis.Set(context.Background(), "job:1:payload", payloadBytes, 0)

				// Expect status update in DB
				mock.ExpectExec("UPDATE jobs SET status = \\$1 WHERE id = \\$2").
					WithArgs(models.StatusCompleted, 1).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectError: false,
		},
		{
			name:    "Unknown Job Type",
			jobType: "unknown",
			setupMocks: func() {
				// Expect status update in DB for failed job
				mock.ExpectExec("UPDATE jobs SET status = \\$1 WHERE id = \\$2").
					WithArgs(models.StatusCompleted, 1).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup test data
			job := struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
				Type string `json:"type"`
			}{
				ID:   1,
				Name: "Test Job",
				Type: tc.jobType,
			}

			// Setup mocks
			tc.setupMocks()

			// Create message
			msgValue, _ := json.Marshal(job)
			msg := &sarama.ConsumerMessage{
				Value: msgValue,
			}

			// Process job
			err := worker.processJob(msg)

			// Verify results
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestWorkerStart(t *testing.T) {
	worker, _, miniRedis, mockConsumerGroup := setupTestWorker(t)
	defer miniRedis.Close()

	// Setup expectations
	errChan := make(chan error)
	mockConsumerGroup.On("Errors").Return(errChan)
	mockConsumerGroup.On("Consume", mock.Anything, []string{worker.cfg.Kafka.Topic}, mock.Anything).
		Return(nil)

	// Start worker in background
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Run worker
	err := worker.Start(ctx)
	assert.NoError(t, err)

	// Verify expectations
	mockConsumerGroup.AssertExpectations(t)
}
