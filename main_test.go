// main_test.go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/IBM/sarama"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

// --- Mock Kafka Producer ---

// Updated MockProducer implements sarama.SyncProducer.
type MockProducer struct{}

func (p *MockProducer) SendMessage(msg *sarama.ProducerMessage) (int32, int64, error) {
	// Simulate a successful Kafka message send.
	return 0, 0, nil
}

func (p *MockProducer) SendMessages(msgs []*sarama.ProducerMessage) error {
	// Simulate a successful send for multiple messages.
	return nil
}

func (p *MockProducer) Close() error {
	// Simulate closing the producer.
	return nil
}

func (p *MockProducer) AbortTxn() error { return nil }
func (p *MockProducer) AddMessageToTxn(msg *sarama.ConsumerMessage, groupId string, topic *string) error {
	return nil
}
func (p *MockProducer) BeginTxn() error                         { return nil }
func (p *MockProducer) CommitTxn() error                        { return nil }
func (p *MockProducer) TxnStatus() sarama.ProducerTxnStatusFlag { return 0 }
func (p *MockProducer) AddOffsetsToTxn(map[string][]*sarama.PartitionOffsetMetadata, string) error {
	return nil
}
func (p *MockProducer) SyncProducer() sarama.SyncProducer { return p }
func (p *MockProducer) IsTransactional() bool             { return false }

// --- Helper to set up sqlmock-backed DB ---

func setupMockDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	dbMock, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock database: %s", err)
	}
	sqlxDB := sqlx.NewDb(dbMock, "sqlmock")
	return sqlxDB, mock
}

// --- Test: Create Job API ---
func TestCreateJob(t *testing.T) {
	// Set up sqlmock as our global db
	mockDB, mock := setupMockDB(t)
	db = mockDB
	defer db.Close()

	// Expect the INSERT query which returns an id.
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO jobs (name) VALUES ($1) RETURNING id")).
		WithArgs("Test Job").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	// Use our MockProducer for Kafka publishing.
	producer = &MockProducer{}

	app := fiber.New()
	app.Post("/jobs", createJob)

	requestBody, err := json.Marshal(map[string]string{"name": "Test Job"})
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}

	req := httptest.NewRequest("POST", "/jobs", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to perform request: %v", err)
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var responseBody map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&responseBody)
	if err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	// Validate the returned job details.
	job, ok := responseBody["job"].(map[string]interface{})
	assert.True(t, ok, "expected job to be a map")
	assert.Equal(t, float64(1), job["id"]) // JSON numbers are decoded as float64.
	assert.Equal(t, "Test Job", job["name"])
	assert.Equal(t, "pending", job["status"])

	// Ensure all SQL expectations were met.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

// --- Test: Get Job by ID API ---
func TestGetJob(t *testing.T) {
	mockDB, mock := setupMockDB(t)
	db = mockDB
	defer db.Close()

	// Expect the SELECT query for getting a job by ID.
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, status FROM jobs WHERE id = $1")).
		WithArgs("1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "status"}).
			AddRow(1, "Test Job", "pending"))

	app := fiber.New()
	app.Get("/jobs/:id", getJob)

	req := httptest.NewRequest("GET", "/jobs/1", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to perform request: %v", err)
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var responseBody map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&responseBody)
	if err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	assert.Equal(t, "Test Job", responseBody["name"])

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

// --- Test: List Jobs API ---
func TestListJobs(t *testing.T) {
	mockDB, mock := setupMockDB(t)
	db = mockDB
	defer db.Close()

	// Expect the SELECT query for listing jobs.
	rows := sqlmock.NewRows([]string{"id", "name", "status"}).
		AddRow(1, "Test Job", "pending").
		AddRow(2, "Another Job", "completed")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, status FROM jobs ORDER BY created_at DESC")).
		WillReturnRows(rows)

	app := fiber.New()
	app.Get("/jobs", listJobs)

	req := httptest.NewRequest("GET", "/jobs", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to perform request: %v", err)
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var responseBody map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&responseBody)
	if err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	jobs, ok := responseBody["jobs"].([]interface{})
	assert.True(t, ok, "expected jobs to be an array")
	assert.Equal(t, 2, len(jobs))

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

// --- Test: JWT Login ---
func TestLogin(t *testing.T) {
	app := fiber.New()
	app.Post("/login", login("testsecret"))

	requestBody, err := json.Marshal(map[string]string{
		"username": "admin",
		"password": "password",
	})
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}

	req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to perform request: %v", err)
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var responseBody map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&responseBody)
	if err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	// Validate that a token is returned.
	token, exists := responseBody["token"]
	assert.True(t, exists, "token should be present")
	tokenStr := token.(string)
	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte("testsecret"), nil
	})
	assert.NoError(t, err)
	expClaim, ok := claims["exp"].(float64)
	assert.True(t, ok, "exp claim should be a float64")
	// Instead of checking for exact equality, allow a small delta.
	expectedExp := time.Now().Add(72 * time.Hour).Unix()
	assert.InDelta(t, expectedExp, int64(expClaim), 5, "expiration time should be within 5 seconds of expected")
}

// --- Test: Process Job Success ---
