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
	return &Worker{
		cfg:      cfg,
		db:       db,
		consumer: consumer,
		ready:    make(chan bool),
	}
}

func (w *Worker) Start(ctx context.Context) error {
	topics := []string{w.cfg.Kafka.Topic}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start error logging for consumer errors
	go func() {
		for err := range w.consumer.Errors() {
			slog.Error("Kafka consumer error", "error", err)
		}
	}()

	// Start consuming messages
	go func() {
		for {
			if err := w.consumer.Consume(ctx, topics, w); err != nil {
				slog.Error("Error from consumer", "error", err)
			}
			if ctx.Err() != nil {
				return
			}
			w.ready = make(chan bool)
		}
	}()

	<-w.ready // Wait till the consumer has been set up
	slog.Info("Worker started successfully", "topics", topics)

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		slog.Info("Received shutdown signal", "signal", sig)
	case <-ctx.Done():
		slog.Info("Context cancelled")
	}

	return nil
}

// Setup is run at the beginning of a new session, before ConsumeClaim.
func (w *Worker) Setup(sarama.ConsumerGroupSession) error {
	close(w.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited.
func (w *Worker) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (w *Worker) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		if err := w.processJob(message); err != nil {
			slog.Error("Failed to process job", "error", err)
		}
		session.MarkMessage(message, "")
	}
	return nil
}

func (w *Worker) processJob(msg *sarama.ConsumerMessage) error {
	var job struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	// Parse JSON message
	if err := json.Unmarshal(msg.Value, &job); err != nil {
		return fmt.Errorf("failed to parse job: %w", err)
	}

	slog.Info("Processing job", "jobID", job.ID, "jobName", job.Name)

	// Process job with retries
	var err error
	for attempt := 1; attempt <= w.cfg.Kafka.RetryMax; attempt++ {
		err = w.processJobLogic(job)
		if err == nil {
			break
		}
		slog.Error("Job processing failed, retrying", "jobID", job.ID, "attempt", attempt, "error", err)
		time.Sleep(w.cfg.Kafka.RetryBackoff)
	}

	// Update job status based on processing result
	ctx := context.Background()
	redisKey := fmt.Sprintf("job:%d", job.ID)

	if err != nil {
		// Job failed after all retries
		slog.Error("Job processing failed after retries", "jobID", job.ID, "error", err)
		if _, dbErr := w.db.DB.Exec("UPDATE jobs SET status = $1 WHERE id = $2", models.StatusFailed, job.ID); dbErr != nil {
			slog.Error("Failed to update job status to failed in DB", "jobID", job.ID, "error", dbErr)
		}
		if err := w.db.Redis.Set(ctx, redisKey, models.StatusFailed, 0).Err(); err != nil {
			slog.Error("Failed to update Redis status to failed", "jobID", job.ID, "error", err)
		}
		return err
	}

	// Job completed successfully
	if _, err := w.db.DB.Exec("UPDATE jobs SET status = $1 WHERE id = $2", models.StatusCompleted, job.ID); err != nil {
		slog.Error("Failed to update job status in DB", "jobID", job.ID, "error", err)
		return err
	}
	if err := w.db.Redis.Set(ctx, redisKey, models.StatusCompleted, 0).Err(); err != nil {
		slog.Error("Failed to update Redis status", "jobID", job.ID, "error", err)
	}

	slog.Info("Job completed successfully", "jobID", job.ID)
	return nil
}

// processJobLogic simulates the actual job processing
func (w *Worker) processJobLogic(job struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}) error {
	time.Sleep(w.cfg.Kafka.ProcessingTime)
	if job.ID%5 == 0 {
		return fmt.Errorf("simulated error for job %d", job.ID)
	}
	return nil
}
