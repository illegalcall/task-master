// worker.go
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
	"github.com/redis/go-redis/v9"
)

// Add these constants near the top of the file, after the imports
const (
	// Retry configuration
	maxRetries      = 3
	shutdownTimeout = 5 * time.Second

	// Redis configuration
	redisDefaultDB   = 0
	redisKeyTemplate = "job:%d"

	// Job statuses
	statusFailed    = "failed"
	statusCompleted = "completed"
)

var (
	processingTime = 10 * time.Second
	retryBackoff   = 2 * time.Second
)

// Global variables for PostgreSQL and Redis.
var (
	db          *sqlx.DB
	redisClient *redis.Client
)

// Consumer implements sarama.ConsumerGroupHandler.
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

// loadEnv returns the value of the environment variable or a default value.
func loadEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

func main() {
	// Load configuration.
	dbConn := loadEnv("DATABASE_URL", "user=admin password=admin dbname=taskmaster sslmode=disable")
	kafkaBrokers := loadEnv("KAFKA_BROKER", "kafka:9092")
	kafkaGroup := loadEnv("KAFKA_GROUP", "job-workers")
	redisAddr := loadEnv("REDIS_ADDR", "localhost:6379")

	// Connect to PostgreSQL.
	var err error
	db, err = sqlx.Connect("postgres", dbConn)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Initialize Redis client.
	redisClient = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // No password by default.
		DB:       0,  // Use default DB.
	})
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}
	slog.Info("✅ Connected to Redis", "address", redisAddr)

	// Setup Kafka consumer group.
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
	consumer := Consumer{ready: make(chan bool)}

	// Setup context and signal handling for graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start error logging for consumer errors.
	go func() {
		for err := range group.Errors() {
			slog.Error("Kafka consumer group error", "error", err)
		}
	}()

	// Start consuming messages.
	topics := []string{"jobs"}
	go func() {
		for {
			if err := group.Consume(ctx, topics, &consumer); err != nil {
				log.Printf("Error from consumer: %v", err)
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()

	// Wait for shutdown signal.
	sig := <-sigChan
	log.Printf("Received shutdown signal: %v", sig)
	cancel() // Trigger graceful shutdown.
	log.Println("Shutting down gracefully...")
	time.Sleep(shutdownTimeout) // Allow in-flight messages to complete.
	log.Println("Worker stopped")
}

// setupKafkaConsumer configures and returns a Kafka consumer group.
func setupKafkaConsumer(broker, group string) (sarama.ConsumerGroup, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Return.Errors = true // Enable error reporting.
	brokers := []string{broker}
	return sarama.NewConsumerGroup(brokers, group, config)
}

// processJob processes a job message and updates its status in both PostgreSQL and Redis.
func processJob(msg *sarama.ConsumerMessage) {
	var job struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	ctx := context.Background()

	// Parse JSON message.
	if err := json.Unmarshal(msg.Value, &job); err != nil {
		slog.Error("Failed to parse job", "error", err, "message", string(msg.Value))
		return
	}

	slog.Info("Processing job", "jobID", job.ID, "jobName", job.Name)

	const maxRetries = 3
	var err error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Execute job logic.
		err = processJobLogic(job)
		if err == nil {
			break // Success.
		}
		slog.Error("Job processing failed, retrying", "jobID", job.ID, "attempt", attempt, "error", err)
		time.Sleep(retryBackoff) // Use the retryBackoff variable.
	}

	redisKey := fmt.Sprintf(redisKeyTemplate, job.ID)
	if err != nil {
		// If job processing failed after retries, mark it as "failed" in the database and Redis.
		slog.Error("Job processing failed after retries", "jobID", job.ID, "error", err)
		if _, dbErr := db.Exec("UPDATE jobs SET status = $1 WHERE id = $2", statusFailed, job.ID); dbErr != nil {
			slog.Error("Failed to update job status to failed in DB", "jobID", job.ID, "error", dbErr)
		}
		if err := redisClient.Set(ctx, redisKey, statusFailed, 0).Err(); err != nil {
			slog.Error("Failed to update Redis status to failed", "jobID", job.ID, "error", err)
		}
		return
	}

	// On success, update job status as "completed" in both PostgreSQL and Redis.
	if _, err := db.Exec("UPDATE jobs SET status = $1 WHERE id = $2", statusCompleted, job.ID); err != nil {
		slog.Error("Failed to update job status in DB", "jobID", job.ID, "error", err)
		return
	}
	if err := redisClient.Set(ctx, redisKey, statusCompleted, 0).Err(); err != nil {
		slog.Error("Failed to update Redis status to completed", "jobID", job.ID, "error", err)
	}
	slog.Info("Job completed successfully", "jobID", job.ID)
}

// processJobLogic simulates the processing work (replace with your actual logic).
// For demonstration, it returns an error if the job ID is divisible by 5.
func processJobLogic(job struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}) error {
	time.Sleep(processingTime)
	if job.ID%5 == 0 {
		return fmt.Errorf("simulated error for job %d", job.ID)
	}
	return nil
}
