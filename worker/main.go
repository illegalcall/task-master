package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Database & Kafka Consumer
var db *sqlx.DB

type Consumer struct {
	ready chan bool
}

func (consumer *Consumer) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (consumer *Consumer) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (consumer *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		processJob(message)
		session.MarkMessage(message, "")
	}
	return nil
}

// Add loadEnv function at the top level
func loadEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

func main() {
	// Load Configs
	dbConn := loadEnv("DATABASE_URL", "user=admin password=admin dbname=taskmaster sslmode=disable")
	kafkaBrokers := loadEnv("KAFKA_BROKER", "kafka:9092")
	kafkaGroup := loadEnv("KAFKA_GROUP", "job-workers")

	// Connect to PostgreSQL
	var err error
	db, err = sqlx.Connect("postgres", dbConn)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	group, err := setupKafkaConsumer(kafkaBrokers, kafkaGroup)
	if err != nil {
		log.Fatal("Failed to start Kafka consumer:", err)
	}
	defer func() {
		if err := group.Close(); err != nil {
			log.Printf("Error closing Kafka consumer group: %v", err)
		}
	}()

	log.Println("Worker listening for jobs... 👂")

	consumer := Consumer{
		ready: make(chan bool),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start consuming in a separate goroutine
	topics := []string{"jobs"}

	// Add error handling goroutine
	go func() {
		for err := range group.Errors() {
			slog.Error("Kafka consumer group error", "error", err)
		}
	}()

	go func() {
		for {
			if err := group.Consume(ctx, topics, &consumer); err != nil {
				log.Printf("Error from consumer: %v", err)
			}
			// Check if context was cancelled, indicating shutdown
			if ctx.Err() != nil {
				return
			}
		}
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("Received shutdown signal: %v", sig)
	cancel() // Trigger graceful shutdown
	log.Println("Shutting down gracefully...")

	// Give some time for in-flight messages to be processed
	time.Sleep(time.Second * 5)
	log.Println("Worker stopped")
}

// Update setupKafkaConsumer to accept parameters
func setupKafkaConsumer(broker, group string) (sarama.ConsumerGroup, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Return.Errors = true // Enable error reporting
	brokers := []string{broker}
	return sarama.NewConsumerGroup(brokers, group, config)
}

// 🔥 Process Job
func processJob(msg *sarama.ConsumerMessage) {
	var job struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	// Parse JSON message
	if err := json.Unmarshal(msg.Value, &job); err != nil {
		// Log error and skip processing this message
		slog.Error("Failed to parse job", "error", err, "message", string(msg.Value))
		return
	}

	slog.Info("Processing job", "jobID", job.ID, "jobName", job.Name)

	const maxRetries = 3
	var err error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Simulate job processing (or call your actual job logic here)
		err = processJobLogic(job)
		if err == nil {
			break // success, exit the retry loop
		}
		slog.Error("Job processing failed, retrying", "jobID", job.ID, "attempt", attempt, "error", err)
		time.Sleep(2 * time.Second) // simple backoff
	}

	if err != nil {
		// After max retries, mark job as failed.
		slog.Error("Job processing failed after retries", "jobID", job.ID, "error", err)
		if _, dbErr := db.Exec("UPDATE jobs SET status = 'failed' WHERE id = $1", job.ID); dbErr != nil {
			slog.Error("Failed to update job status to failed", "jobID", job.ID, "error", dbErr)
		}
		// Optionally: publish the job message to a DLQ topic here
		return
	}

	// Update job status in the database as "completed"
	if _, err := db.Exec("UPDATE jobs SET status = 'completed' WHERE id = $1", job.ID); err != nil {
		slog.Error("Failed to update job status", "jobID", job.ID, "error", err)
		return
	}

	slog.Info("Job completed successfully", "jobID", job.ID)
}

// processJobLogic simulates the processing work (replace with your actual logic)
func processJobLogic(job struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}) error {
	// Simulate some processing time
	time.Sleep(3 * time.Second)
	// Simulate random failure (for demonstration only)
	// Remove or replace with real error checks in production.
	if job.ID%5 == 0 {
		return fmt.Errorf("simulated error for job %d", job.ID)
	}
	return nil
}
