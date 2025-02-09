package main

import (
	"context"
	"encoding/json"
	"log"
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

func main() {
	// Connect to PostgreSQL
	var err error
	db, err = sqlx.Connect("postgres", "user=admin password=admin dbname=taskmaster sslmode=disable")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	group, err := setupKafkaConsumer()
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

// ✅ Setup Kafka Consumer
func setupKafkaConsumer() (sarama.ConsumerGroup, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	brokers := []string{"localhost:9092"}
	group := "job-workers"
	return sarama.NewConsumerGroup(brokers, group, config)
}

// 🔥 Process Job
func processJob(msg *sarama.ConsumerMessage) {
	var job struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	// Parse JSON message
	err := json.Unmarshal(msg.Value, &job)
	if err != nil {
		log.Println("Failed to parse job:", err)
		return
	}

	log.Printf("Processing job %d: %s...\n", job.ID, job.Name)

	// Simulate job processing
	time.Sleep(3 * time.Second)

	// Update job status in database
	_, err = db.Exec("UPDATE jobs SET status = 'completed' WHERE id = $1", job.ID)
	if err != nil {
		log.Println("Failed to update job status:", err)
		return
	}

	log.Printf("Job %d completed! ✅\n", job.ID)
}
