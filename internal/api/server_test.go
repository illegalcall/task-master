package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/IBM/sarama"
	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/mock"

	"github.com/illegalcall/task-master/internal/config"
	"github.com/illegalcall/task-master/internal/models"
	"github.com/illegalcall/task-master/pkg/database"
)

// MockProducer simulates Kafka producer for testing
type MockProducer struct {
	sarama.SyncProducer
	messages []*sarama.ProducerMessage
}

func (m *MockProducer) SendMessage(msg *sarama.ProducerMessage) (partition int32, offset int64, err error) {
	m.messages = append(m.messages, msg)
	return 0, 0, nil
}

func (m *MockProducer) Close() error {
	return nil
}

// mockProducer implements sarama.SyncProducer for testing
type mockProducer struct {
	mock.Mock
}

func (m *mockProducer) SendMessage(msg *sarama.ProducerMessage) (partition int32, offset int64, err error) {
	args := m.Called(msg)
	return args.Get(0).(int32), args.Get(1).(int64), args.Error(2)
}

func (m *mockProducer) SendMessages(msgs []*sarama.ProducerMessage) error {
	args := m.Called(msgs)
	return args.Error(0)
}

func (m *mockProducer) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockProducer) TxnStatus() sarama.ProducerTxnStatusFlag {
	args := m.Called()
	return args.Get(0).(sarama.ProducerTxnStatusFlag)
}

func (m *mockProducer) IsTransactional() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *mockProducer) BeginTxn() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockProducer) CommitTxn() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockProducer) AbortTxn() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockProducer) AddPartitionToTxn(topic string, partition int32) error {
	args := m.Called(topic, partition)
	return args.Error(0)
}

func (m *mockProducer) AddOffsetsToTxn(offsets map[string][]*sarama.PartitionOffsetMetadata, groupId string) error {
	args := m.Called(offsets, groupId)
	return args.Error(0)
}

func (m *mockProducer) AddMessageToTxn(msg *sarama.ConsumerMessage, groupId string, metadata *string) error {
	args := m.Called(msg, groupId, metadata)
	return args.Error(0)
}

// setupTestServer initializes a test instance of the API server.
func setupTestServer(t *testing.T) (*Server, sqlmock.Sqlmock, *miniredis.Miniredis) {
	// Setup mock PostgreSQL
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)

	db := sqlx.NewDb(mockDB, "sqlmock")

	// Setup mock Redis
	miniRedis, err := miniredis.Run()
	assert.NoError(t, err)

	redisClient := redis.NewClient(&redis.Options{
		Addr: miniRedis.Addr(),
	})

	// Create mock Kafka producer
	producer := &MockProducer{}

	// Create test config
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:        ":8080",
			Environment: "development",
		},
		JWT: config.JWTConfig{
			Secret:     "test-secret",
			Expiration: 24 * time.Hour,
		},
		Kafka: config.KafkaConfig{
			Topic: "test-topic",
		},
	}

	// Create test clients
	clients := &database.Clients{
		DB:    db,
		Redis: redisClient,
	}

	server, err := NewServer(cfg, clients, producer)
	require.NoError(t, err)
	require.NotNil(t, server)

	// Disable JWT middleware for tests
	app := fiber.New()
	server.app = app

	// Register only the routes we want to test
	app.Post("/api/login", server.handleLogin)
	app.Post("/jobs", server.handleCreateJob)
	app.Get("/jobs/:id", server.handleGetJob)
	app.Get("/jobs", server.handleListJobs)

	return server, mock, miniRedis
}

// 🔹 Test Job Creation
func TestHandleCreateJob(t *testing.T) {
	server, mock, miniRedis := setupTestServer(t)
	defer miniRedis.Close()

	// Expect the INSERT query with Type field
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO jobs (name, status, type) VALUES ($1, $2, $3) RETURNING id")).
		WithArgs("Test Job", models.StatusPending, "test_job").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	// Create test request with Type field
	reqBody := map[string]string{
		"name": "Test Job",
		"type": "test_job",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Run the test
	resp, err := server.app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode, "Expected HTTP 200 for successful job creation")

	// Verify response JSON
	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	assert.NoError(t, err, "Response JSON should be valid")

	job, ok := result["job"].(map[string]interface{})
	assert.True(t, ok, "Job response should be a map")

	// Validate job response fields
	assert.Equal(t, float64(1), job["id"], "Job ID should be 1")
	assert.Equal(t, "Test Job", job["name"], "Job name should match input")
	assert.Equal(t, "test_job", job["type"], "Job type should match input")
	assert.Equal(t, models.StatusPending, job["status"], "Job should be in pending state")

	// Verify Redis contains the job status
	redisVal, err := miniRedis.Get("job:1")
	assert.NoError(t, err, "Redis should contain job key")
	assert.Equal(t, models.StatusPending, redisVal, "Redis status should be 'pending'")

	// Verify Kafka message was published
	mockProducer := server.producer.(*MockProducer)
	assert.Len(t, mockProducer.messages, 1, "Kafka should have 1 message")
	assert.Contains(t, string(mockProducer.messages[0].Value.(sarama.StringEncoder)), `"id":1`, "Kafka message should contain job ID")

	// Verify mock expectations
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 🔹 Test Fetching Job
func TestHandleGetJob(t *testing.T) {
	server, mock, miniRedis := setupTestServer(t)
	defer miniRedis.Close()

	// Setup test job data
	jobID := 1
	jobName := "Test Job"
	jobType := "test_job"
	jobStatus := models.StatusCompleted

	// Expect SELECT query with Type field
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, status, type FROM jobs WHERE id = $1")).
		WithArgs(jobID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "status", "type"}).
			AddRow(jobID, jobName, jobStatus, jobType))

	// Set Redis status
	miniRedis.Set("job:1", models.StatusCompleted)

	// Create test request
	req := httptest.NewRequest("GET", "/jobs/1", nil)

	// Run the test
	resp, err := server.app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode, "Expected HTTP 200 for successful job retrieval")

	// Verify response
	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	assert.NoError(t, err, "Response JSON should be valid")

	job, ok := result["job"].(map[string]interface{})
	assert.True(t, ok, "Response should contain job object")

	// Validate job fields
	assert.Equal(t, float64(jobID), job["id"], "Job ID should match")
	assert.Equal(t, jobName, job["name"], "Job name should match")
	assert.Equal(t, jobType, job["type"], "Job type should match")
	assert.Equal(t, models.StatusCompleted, job["status"], "Job status should match Redis override")

	// Verify mock expectations
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestServer(t *testing.T) {
	cfg := &config.Config{
		// ... config setup ...
	}

	db := &database.Clients{
		// ... db setup ...
	}

	producer := &mockProducer{}

	server, err := NewServer(cfg, db, producer)
	require.NoError(t, err)
	require.NotNil(t, server)

	// ... rest of the test ...
}
