package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/IBM/sarama"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	jwtware "github.com/gofiber/jwt/v3"
	"github.com/golang-jwt/jwt/v4"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Database connection
var db *sqlx.DB
var producer sarama.SyncProducer

func main() {
	// Initialize Fiber app
	app := fiber.New()

	// Public routes (no JWT required)
	app.Post("/login", login) // Add login route before JWT middleware

	app.Use(limiter.New(limiter.Config{
		Max:        100,
		Expiration: 60 * time.Second,
	}))

	// Apply Caching (Cache responses for 10 seconds)
	app.Use(cache.New(cache.Config{
		Expiration:   10 * time.Second,
		CacheControl: true,
	}))

	// Enable logging for API requests
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${ip} ${method} ${path} ${status}\n",
	}))

	app.Use(jwtware.New(jwtware.Config{
		SigningKey: []byte("supersecretkey"),
	}))

	// Connect to PostgreSQL
	var err error
	db, err = sqlx.Connect("postgres", "user=admin password=admin dbname=taskmaster sslmode=disable")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Connect to Kafka
	producer, err = setupKafkaProducer()
	if err != nil {
		log.Fatal("Failed to connect to Kafka:", err)
	}
	defer producer.Close()

	// Create table if not exists
	createJobsTable()

	// API Routes
	app.Post("/jobs", createJob)
	app.Get("/jobs/:id", getJob)
	app.Get("/jobs", listJobs)
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	// Start server
	log.Println("Server running on port 8080 🚀")
	log.Fatal(app.Listen(":8080"))
}

// ✅ Setup Kafka Producer
func setupKafkaProducer() (sarama.SyncProducer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	brokers := []string{"localhost:9092"} // Kafka running inside Docker
	fmt.Println("Brokers:", brokers)
	return sarama.NewSyncProducer(brokers, config)
}

func createJobsTable() {
	schema := `CREATE TABLE IF NOT EXISTS jobs (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		status TEXT DEFAULT 'pending',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	_, err := db.Exec(schema)
	if err != nil {
		log.Fatal("❌ Failed to create jobs table:", err)
	} else {
		log.Println("✅ Jobs table is ready!")
	}
}

// Handler to create a new job
func createJob(c *fiber.Ctx) error {
	type JobRequest struct {
		Name string `json:"name"`
	}
	var req JobRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Job name cannot be empty"})
	}

	// Insert job into DB
	var jobID int
	err := db.QueryRow("INSERT INTO jobs (name) VALUES ($1) RETURNING id", req.Name).Scan(&jobID)
	if err != nil {
		log.Println("❌ Database insert error:", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to insert job", "details": err.Error()})
	}

	// Publish job to Kafka
	jobMsg := map[string]interface{}{
		"id":   jobID,
		"name": req.Name,
	}
	jobData, _ := json.Marshal(jobMsg)

	message := &sarama.ProducerMessage{
		Topic: "jobs",
		Value: sarama.StringEncoder(jobData),
	}
	_, _, err = producer.SendMessage(message)
	if err != nil {
		log.Println("❌ Failed to send job to Kafka:", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to send job to Kafka"})
	}

	return c.JSON(fiber.Map{
		"message": "✅ Job created successfully",
		"job": fiber.Map{
			"id":     jobID,
			"name":   req.Name,
			"status": "pending",
		},
	})
}

// Handler to fetch a single job
func getJob(c *fiber.Ctx) error {
	id := c.Params("id")
	// id := 1
	var job struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	err := db.Get(&job, "SELECT id, name, status FROM jobs WHERE id = $1", id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Job not found"})
	}

	return c.JSON(job)
}

// Handler to list all jobs
func listJobs(c *fiber.Ctx) error {
	var jobs []struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}

	err := db.Select(&jobs, "SELECT id, name, status FROM jobs ORDER BY created_at DESC")
	if err != nil {
		log.Println("Error fetching jobs:", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch jobs"})
	}

	// If no jobs are found, return an empty array
	if len(jobs) == 0 {
		return c.JSON(fiber.Map{"jobs": []interface{}{}})
	}

	return c.JSON(fiber.Map{"jobs": jobs})
}

// Login handler to generate JWT token
func login(c *fiber.Ctx) error {
	type LoginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	var request LoginRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	// For demo purposes, using simple credentials
	// In production, you should validate against a database
	if request.Username != "admin" || request.Password != "password" {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	// Create the Claims
	claims := jwt.MapClaims{
		"username": request.Username,
		"admin":    true,
		"exp":      time.Now().Add(time.Hour * 72).Unix(), // Token expires in 72 hours
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Generate encoded token
	t, err := token.SignedString([]byte("supersecretkey"))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate token"})
	}

	return c.JSON(fiber.Map{
		"token": t,
		"type":  "Bearer",
	})
}
