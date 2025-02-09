package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/IBM/sarama"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	jwtware "github.com/gofiber/jwt/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Global Variables
var (
	db       *sqlx.DB
	producer sarama.SyncProducer
)

// ✅ Load Environment Variables (or set defaults)
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
	secretKey := loadEnv("JWT_SECRET", "supersecretkey")

	// Initialize Fiber
	app := fiber.New()

	// Logging Middleware
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${ip} ${method} ${path} ${status}\n",
	}))

	// Rate Limiting Middleware (Prevent DDoS)
	app.Use(limiter.New(limiter.Config{
		Max:        100,
		Expiration: 60 * time.Second,
	}))

	// Caching Middleware (Speeds up repeated API calls)
	app.Use(cache.New(cache.Config{
		Expiration:   10 * time.Second,
		CacheControl: true,
	}))

	// Initialize PostgreSQL Connection
	var err error
	db, err = sqlx.Connect("postgres", dbConn)
	if err != nil {
		slog.Error("❌ Database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("✅ Connected to PostgreSQL")

	// Initialize Kafka Producer
	producer, err = setupKafkaProducer(kafkaBrokers)
	if err != nil {
		slog.Error("❌ Kafka connection failed", "error", err)
		os.Exit(1)
	}
	defer producer.Close()
	slog.Info("✅ Connected to Kafka")

	// Create Jobs Table (if not exists)
	createJobsTable()

	// Public Routes
	app.Post("/login", login(secretKey))

	// Protected Routes (Require JWT)
	app.Use(jwtware.New(jwtware.Config{SigningKey: []byte(secretKey)}))
	app.Post("/jobs", createJob)
	app.Get("/jobs/:id", getJob)
	app.Get("/jobs", listJobs)

	// Monitoring Route
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	// Graceful Shutdown Handling
	go func() {
		slog.Info("🚀 Server running on port 8080")
		if err := app.Listen(":8080"); err != nil {
			slog.Error("❌ Server error", "error", err)
		}
	}()

	// Wait for Interrupt Signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	slog.Info("🛑 Server shutting down...")
}

// ✅ Setup Kafka Producer
func setupKafkaProducer(broker string) (sarama.SyncProducer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.Retry.Max = 5
	config.Producer.Retry.Backoff = 500 * time.Millisecond
	return sarama.NewSyncProducer([]string{broker}, config)
}

// ✅ Ensure Jobs Table Exists
func createJobsTable() {
	schema := `CREATE TABLE IF NOT EXISTS jobs (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		status TEXT DEFAULT 'pending',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(schema); err != nil {
		slog.Error("❌ Failed to create jobs table", "error", err)
		os.Exit(1)
	} else {
		slog.Info("✅ Jobs table is ready!")
	}
}

// ✅ Handler: Create Job
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

	// Insert Job into Database
	var jobID int
	err := db.QueryRow("INSERT INTO jobs (name) VALUES ($1) RETURNING id", req.Name).Scan(&jobID)
	if err != nil {
		slog.Error("❌ Database insert error", "error", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to insert job"})
	}

	// Publish to Kafka
	jobMsg, _ := json.Marshal(fiber.Map{"id": jobID, "name": req.Name})
	message := &sarama.ProducerMessage{
		Topic: "jobs",
		Value: sarama.StringEncoder(jobMsg),
	}
	if _, _, err := producer.SendMessage(message); err != nil {
		slog.Error("❌ Failed to send job to Kafka", "error", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to send job to Kafka"})
	}

	return c.JSON(fiber.Map{"message": "✅ Job created successfully", "job": fiber.Map{"id": jobID, "name": req.Name, "status": "pending"}})
}

// ✅ Handler: Fetch Job by ID
func getJob(c *fiber.Ctx) error {
	id := c.Params("id")

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

// ✅ Handler: List All Jobs
func listJobs(c *fiber.Ctx) error {
	var jobs []struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}

	err := db.Select(&jobs, "SELECT id, name, status FROM jobs ORDER BY created_at DESC")
	if err != nil {
		slog.Error("❌ Error fetching jobs", "error", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch jobs"})
	}

	return c.JSON(fiber.Map{"jobs": jobs})
}

// ✅ Handler: Login (JWT Token Generation)
func login(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}

		if req.Username != "admin" || req.Password != "password" {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"username": req.Username,
			"exp":      time.Now().Add(72 * time.Hour).Unix(),
		})

		t, _ := token.SignedString([]byte(secret))
		return c.JSON(fiber.Map{"token": t, "type": "Bearer"})
	}
}
