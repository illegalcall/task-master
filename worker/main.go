package main

import (
	"context"
	"encoding/json"
	"log"
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

	consumer := Consumer{
		ready: make(chan bool),
	}

	ctx := context.Background()
	topics := []string{"jobs"}

	for {
		err := group.Consume(ctx, topics, &consumer)
		if err != nil {
			log.Printf("Error from consumer: %v", err)
		}
	}
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
