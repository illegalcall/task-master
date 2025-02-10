package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/illegalcall/task-master/internal/config"
	"github.com/illegalcall/task-master/internal/worker"
	"github.com/illegalcall/task-master/pkg/database"
	"github.com/illegalcall/task-master/pkg/kafka"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Initialize database clients
	db, err := database.NewClients(cfg.Database.URL, cfg.Redis.Addr)
	if err != nil {
		slog.Error("Failed to initialize database clients", "error", err)
		os.Exit(1)
	}
	defer db.DB.Close()
	slog.Info("✅ Connected to databases")

	// Initialize Kafka consumer
	consumer, err := kafka.NewConsumer(cfg.Kafka.Broker, cfg.Kafka.Group)
	if err != nil {
		slog.Error("Failed to create Kafka consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()
	slog.Info("✅ Connected to Kafka")

	// Create and start worker
	worker := worker.NewWorker(cfg, db, consumer)

	ctx := context.Background()
	if err := worker.Start(ctx); err != nil {
		slog.Error("Worker error", "error", err)
		os.Exit(1)
	}
}
