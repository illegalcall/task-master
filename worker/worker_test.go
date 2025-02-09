// process_job_test.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/IBM/sarama"
	"github.com/alicebob/miniredis/v2"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// TestProcessJob_Success verifies that a job with an ID not divisible by 5 is processed successfully.
func TestProcessJob_Success(t *testing.T) {
	// Override timing variables for tests.
	origProcessingTime := processingTime
	origRetryBackoff := retryBackoff
	processingTime = 10 * time.Millisecond
	retryBackoff = 10 * time.Millisecond
	defer func() {
		processingTime = origProcessingTime
		retryBackoff = origRetryBackoff
	}()

	// Create a sqlmock database and assign it to the global db variable.
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock database: %v", err)
	}
	defer sqlDB.Close()
	db = sqlx.NewDb(sqlDB, "sqlmock")

	// Set up test Redis.
	miniRedis, rClient := setupTestRedis(t)
	defer miniRedis.Close()
	redisClient = rClient

	// For a successful job, processJobLogic should return nil and then processJob will update the job as completed.
	// We expect one UPDATE query to mark the job as "completed".
	mock.ExpectExec(regexp.QuoteMeta("UPDATE jobs SET status = $1 WHERE id = $2")).
		WithArgs(statusCompleted, 1).
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
	// Now the function should complete in much less time.
	assert.Less(t, elapsed.Seconds(), 0.5, "processJob took too long")

	// Verify that Redis has the job marked as "completed".
	redisKey := fmt.Sprintf(redisKeyTemplate, 1)
	status, err := redisClient.Get(context.Background(), redisKey).Result()
	if err != nil {
		t.Fatalf("failed to get job status from Redis: %v", err)
	}
	assert.Equal(t, statusCompleted, status, "expected job status to be completed")

	// Ensure all expectations were met.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %v", err)
	}
}

// TestProcessJob_Failure verifies that a job with an ID divisible by 5 fails processing and is marked as "failed".
func TestProcessJob_Failure(t *testing.T) {
	// Override timing variables for tests.
	origProcessingTime := processingTime
	origRetryBackoff := retryBackoff
	processingTime = 10 * time.Millisecond
	retryBackoff = 10 * time.Millisecond
	defer func() {
		processingTime = origProcessingTime
		retryBackoff = origRetryBackoff
	}()

	// Create a sqlmock database and assign it to the global db variable.
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock database: %v", err)
	}
	defer sqlDB.Close()
	db = sqlx.NewDb(sqlDB, "sqlmock")

	// Set up test Redis.
	miniRedis, rClient := setupTestRedis(t)
	defer miniRedis.Close()
	redisClient = rClient

	// For a failing job, processJobLogic (the real function) will return an error for IDs divisible by 5.
	// After maxRetries, processJob will update the job as failed.
	mock.ExpectExec(regexp.QuoteMeta("UPDATE jobs SET status = $1 WHERE id = $2")).
		WithArgs(statusFailed, 5).
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
	// With the overrides, the failure scenario should complete very quickly.
	assert.Less(t, elapsed.Seconds(), 1.0, "processJob took too long in failure scenario")

	// Verify that Redis has the job marked as "failed".
	redisKey := fmt.Sprintf(redisKeyTemplate, 5)
	status, err := redisClient.Get(context.Background(), redisKey).Result()
	if err != nil {
		t.Fatalf("failed to get job status from Redis: %v", err)
	}
	assert.Equal(t, statusFailed, status, "expected job status to be failed")

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

	// Set up test Redis.
	miniRedis, rClient := setupTestRedis(t)
	defer miniRedis.Close()
	redisClient = rClient

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

func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	miniRedis, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	client := redis.NewClient(&redis.Options{
		Addr: miniRedis.Addr(),
	})
	return miniRedis, client
}
