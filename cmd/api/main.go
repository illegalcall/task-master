package main

import (
	"log/slog"
	"os"

	"github.com/illegalcall/task-master/internal/api"
	"github.com/illegalcall/task-master/internal/config"
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

	// Initialize Kafka producer
	producer, err := kafka.NewProducer(cfg.Kafka.Broker, cfg.Kafka.RetryMax, int64(cfg.Kafka.RetryBackoff))
	if err != nil {
		slog.Error("Failed to create Kafka producer", "error", err)
		os.Exit(1)
	}
	defer producer.Close()
	slog.Info("✅ Connected to Kafka")

	// Create and start server
	server := api.NewServer(cfg, db, producer)
	if err := server.Start(); err != nil {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}
}
