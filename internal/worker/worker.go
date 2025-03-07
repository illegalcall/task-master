package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"

	"github.com/illegalcall/task-master/internal/config"
	"github.com/illegalcall/task-master/internal/jobs"
	"github.com/illegalcall/task-master/internal/models"
	"github.com/illegalcall/task-master/pkg/database"
)

type Worker struct {
	cfg      *config.Config
	db       *database.Clients
	consumer sarama.ConsumerGroup
	ready    chan bool
}

func NewWorker(cfg *config.Config, db *database.Clients, consumer sarama.ConsumerGroup) *Worker {
	slog.Info("Initializing new Worker")
	return &Worker{
		cfg:      cfg,
		db:       db,
		consumer: consumer,
		ready:    make(chan bool),
	}
}

func (w *Worker) Start(ctx context.Context) error {
	topics := []string{w.cfg.Kafka.Topic}
	slog.Info("Starting worker", "topics", topics)

	// Initialize the jobs database
	jobs.InitDB(w.db)
	slog.Info("Jobs database initialized")

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	slog.Info("Signal handler setup complete")

	// Start error logging for consumer errors
	go func() {
		for err := range w.consumer.Errors() {
			slog.Error("Kafka consumer error received", "error", err)
		}
	}()

	// Start consuming messages
	go func() {
		slog.Info("Consumer goroutine started")
		for {
			slog.Info("Calling consumer.Consume")
			if err := w.consumer.Consume(ctx, topics, w); err != nil {
				slog.Error("Error from consumer.Consume", "error", err)
			}
			if ctx.Err() != nil {
				slog.Info("Context error detected, exiting consumer loop", "error", ctx.Err())
				return
			}
			// Reset the ready channel after a new session is created
			w.ready = make(chan bool)
		}
	}()

	<-w.ready // Wait till the consumer has been set up
	slog.Info("Worker setup complete; consumer ready")

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		slog.Info("Received shutdown signal", "signal", sig)
	case <-ctx.Done():
		slog.Info("Context cancelled; shutting down worker")
	}

	slog.Info("Worker shutting down gracefully")
	return nil
}

// Setup is run at the beginning of a new session, before ConsumeClaim.
func (w *Worker) Setup(sarama.ConsumerGroupSession) error {
	slog.Info("Consumer group session setup complete")
	close(w.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited.
func (w *Worker) Cleanup(sarama.ConsumerGroupSession) error {
	slog.Info("Consumer group session cleanup complete")
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (w *Worker) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	slog.Info("Starting ConsumeClaim loop")
	for message := range claim.Messages() {
		slog.Info("Message received from Kafka", "offset", message.Offset, "partition", message.Partition)
		if err := w.processJob(message); err != nil {
			slog.Error("Failed to process job", "error", err)
		} else {
			slog.Info("Job processed successfully", "offset", message.Offset)
		}
		session.MarkMessage(message, "")
		slog.Info("Message marked as processed", "offset", message.Offset)
	}
	return nil
}

func (w *Worker) processJob(msg *sarama.ConsumerMessage) error {
	var job struct {
		ID        int       `json:"id"`
		Name      string    `json:"name"`
		Status    string    `json:"status"`
		Type      string    `json:"type"`
		CreatedAt time.Time `json:"created_at"`
	}

	slog.Info("Received Kafka message", "msg", string(msg.Value))
	// Parse JSON message
	if err := json.Unmarshal(msg.Value, &job); err != nil {
		slog.Error("JSON unmarshalling failed", "error", err, "raw", string(msg.Value))
		return fmt.Errorf("failed to parse job: %w", err)
	}
	slog.Info("Job parsed successfully", "jobID", job.ID, "jobName", job.Name)

	// Process job with retries
	var err error
	for attempt := 1; attempt <= w.cfg.Kafka.RetryMax; attempt++ {
		slog.Info("Attempting job processing", "jobID", job.ID, "attempt", attempt)
		err = w.processJobLogic(job)
		if err == nil {
			slog.Info("Job logic processed successfully", "jobID", job.ID, "attempt", attempt)
			break
		}
		slog.Error("Job processing logic failed", "jobID", job.ID, "attempt", attempt, "error", err)
		time.Sleep(w.cfg.Kafka.RetryBackoff)
	}

	// Update job status based on processing result
	ctx := context.Background()
	redisKey := fmt.Sprintf("job:%d", job.ID)
	if err != nil {
		// Job failed after all retries
		slog.Error("Job processing ultimately failed", "jobID", job.ID, "error", err)
		if _, dbErr := w.db.DB.Exec("UPDATE jobs SET status = $1 WHERE id = $2", models.StatusFailed, job.ID); dbErr != nil {
			slog.Error("Failed to update job status to failed in DB", "jobID", job.ID, "error", dbErr)
		} else {
			slog.Info("Job status updated to failed in DB", "jobID", job.ID)
		}
		if err := w.db.Redis.Set(ctx, redisKey, models.StatusFailed, 0).Err(); err != nil {
			slog.Error("Failed to update Redis status to failed", "jobID", job.ID, "error", err)
		} else {
			slog.Info("Redis job status set to failed", "jobID", job.ID)
		}
		return err
	}

	// Job completed successfully
	slog.Info("Job processing completed without errors", "jobID", job.ID)
	if _, err := w.db.DB.Exec("UPDATE jobs SET status = $1 WHERE id = $2", models.StatusCompleted, job.ID); err != nil {
		slog.Error("Failed to update job status to completed in DB", "jobID", job.ID, "error", err)
		return err
	}
	slog.Info("Job status updated to completed in DB", "jobID", job.ID)

	if err := w.db.Redis.Set(ctx, redisKey, models.StatusCompleted, 0).Err(); err != nil {
		slog.Error("Failed to update Redis status to completed", "jobID", job.ID, "error", err)
	} else {
		slog.Info("Redis job status set to completed", "jobID", job.ID)
	}

	return nil
}

func (w *Worker) processJobLogic(job struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}) error {
	ctx := context.Background()

	// Retrieve job payload from Redis
	redisKey := fmt.Sprintf("job:%d:payload", job.ID)
	slog.Info("Fetching full job payload from Redis", "redisKey", redisKey)
	payloadBytes, err := w.db.Redis.Get(ctx, redisKey).Bytes()
	if err != nil {
		slog.Error("Failed to get job payload from Redis", "jobID", job.ID, "error", err)
		return fmt.Errorf("failed to get job payload: %w", err)
	}
	slog.Info("Job payload retrieved from Redis", "jobID", job.ID)

	switch job.Type {
	case models.JobTypePDFParse:
		slog.Info("Processing PDF parsing job", "jobID", job.ID)
		// Process PDF parsing job
		result, err := jobs.ParseDocumentHandler(ctx, payloadBytes, job.ID)
		if err != nil {
			slog.Error("PDF parsing failed", "jobID", job.ID, "error", err)
			return fmt.Errorf("failed to process PDF: %w", err)
		}
		slog.Info("PDF parsed successfully", "jobID", job.ID)

		// Store result in Redis
		resultKey := fmt.Sprintf("job:%d:result", job.ID)
		resultBytes, _ := json.Marshal(result)
		slog.Info("Storing job result in Redis", "resultKey", resultKey)
		if err := w.db.Redis.Set(ctx, resultKey, resultBytes, w.cfg.Storage.TTL).Err(); err != nil {
			slog.Error("Failed to store job result in Redis", "jobID", job.ID, "error", err)
			return fmt.Errorf("failed to store result: %w", err)
		}
		slog.Info("Job result stored successfully in Redis", "jobID", job.ID)
		return nil

	default:
		// For other job types, use default processing
		slog.Info("Default job processing for non-PDF job", "jobID", job.ID)
		time.Sleep(w.cfg.Kafka.ProcessingTime)
		if job.ID%5 == 0 {
			slog.Error("Simulated error triggered for job", "jobID", job.ID)
			return fmt.Errorf("simulated error for job %d", job.ID)
		}
		slog.Info("Default job processing completed", "jobID", job.ID)
		return nil
	}
}
