package main

import (
	"encoding/json"
	"testing"

	"github.com/IBM/sarama"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

// ✅ Setup Mock Database
func setupMockWorkerDB() {
	db = sqlx.MustConnect("postgres", "host=localhost user=admin password=admin dbname=taskmaster sslmode=disable")
}

// ✅ Test Kafka Job Processing
func TestProcessJob(t *testing.T) {
	setupMockWorkerDB() // Ensure DB is initialized

	job := map[string]interface{}{
		"id":   1,
		"name": "Test Job",
	}
	jobData, _ := json.Marshal(job)

	msg := &sarama.ConsumerMessage{
		Value: jobData,
	}

	processJob(msg)

	// Verify the job is marked as completed in the database
	var status string
	db.QueryRow("SELECT status FROM jobs WHERE id = $1", job["id"]).Scan(&status)

	assert.Equal(t, "completed", status, "Job status should be completed")
}
