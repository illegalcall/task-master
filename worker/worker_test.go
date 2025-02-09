package main

import (
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/IBM/sarama"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

// TestProcessJob_Success verifies that a job with an ID not divisible by 5 is processed successfully.
func TestProcessJob_Success(t *testing.T) {
	// Create a sqlmock database and assign it to the global db variable.
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock database: %v", err)
	}
	defer sqlDB.Close()
	db = sqlx.NewDb(sqlDB, "sqlmock")

	// For a successful job, processJobLogic should return nil and then processJob will update the job as completed.
	// We expect one UPDATE query to mark the job as "completed".
	mock.ExpectExec(regexp.QuoteMeta("UPDATE jobs SET status = 'completed' WHERE id = $1")).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Build a valid job message.
	job := map[string]interface{}{
		"id":   1,
		"name": "Test Job",
	}
	jobData, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("failed to marshal job data: %v", err)
	}

	msg := &sarama.ConsumerMessage{
		Value: jobData,
	}

	start := time.Now()
	processJob(msg)
	elapsed := time.Since(start)
	// Check that the function completes in a reasonable time.
	assert.Less(t, elapsed.Seconds(), 10.0, "processJob took too long")

	// Ensure all expectations were met.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

// TestProcessJob_Failure verifies that a job with an ID divisible by 5 fails processing and is marked as "failed".
func TestProcessJob_Failure(t *testing.T) {
	// Create a sqlmock database and assign it to the global db variable.
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock database: %v", err)
	}
	defer sqlDB.Close()
	db = sqlx.NewDb(sqlDB, "sqlmock")

	// For a failing job, processJobLogic should always return an error.
	// After maxRetries, processJob will update the job as failed.
	mock.ExpectExec(regexp.QuoteMeta("UPDATE jobs SET status = 'failed' WHERE id = $1")).
		WithArgs(5).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Build a job message that will simulate failure (job ID divisible by 5).
	job := map[string]interface{}{
		"id":   5,
		"name": "Test Job Failure",
	}
	jobData, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("failed to marshal job data: %v", err)
	}

	msg := &sarama.ConsumerMessage{
		Value: jobData,
	}

	start := time.Now()
	processJob(msg)
	elapsed := time.Since(start)
	// Relax the allowed time to 16 seconds to account for minor overhead.
	assert.Less(t, elapsed.Seconds(), 16.0, "processJob took too long in failure scenario")

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

// TestProcessJob_InvalidJSON verifies that when an invalid JSON is received, no database calls are made.
func TestProcessJob_InvalidJSON(t *testing.T) {
	// Create a sqlmock database and assign it to the global db variable.
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock database: %v", err)
	}
	defer sqlDB.Close()
	db = sqlx.NewDb(sqlDB, "sqlmock")

	// Build a message with invalid JSON.
	msg := &sarama.ConsumerMessage{
		Value: []byte("invalid json"),
	}

	// Call processJob. Since the JSON is invalid, no DB Exec should be executed.
	processJob(msg)

	// Verify that no unexpected database interactions occurred.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unexpected database calls were made: %v", err)
	}
}
