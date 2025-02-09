// main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/redis/go-redis/v9"
)

var (
	db          *sqlx.DB
	producer    sarama.SyncProducer
	redisClient *redis.Client
)

// Add these constants after the imports
const (
	// Server configuration
	serverPort     = ":8080"
	serverShutdown = 5 * time.Second

	// Rate limiting
	maxRequests     = 100
	requestTimeout  = 60 * time.Second
	cacheExpiration = 10 * time.Second

	// Authentication
	jwtExpiration = 72 * time.Hour
	adminUser     = "admin"
	adminPass     = "password"

	// Database defaults
	defaultDBURL  = "user=admin password=admin dbname=taskmaster sslmode=disable"
	defaultKafka  = "kafka:9092"
	defaultJWTKey = "supersecretkey"
	defaultRedis  = "localhost:6379"

	// Job statuses
	statusPending   = "pending"
	statusFailed    = "failed"
	statusCompleted = "completed"

	// Redis configuration
	redisDefaultDB   = 0
	redisKeyTemplate = "job:%d"

	// Kafka configuration
	kafkaTopic        = "jobs"
	kafkaRetryMax     = 5
	kafkaRetryBackoff = 500 * time.Millisecond
)

// loadEnv loads an environment variable or returns a default value.
func loadEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

func main() {
	// Load configuration.
	dbConn := loadEnv("DATABASE_URL", defaultDBURL)
	kafkaBrokers := loadEnv("KAFKA_BROKER", defaultKafka)
	secretKey := loadEnv("JWT_SECRET", defaultJWTKey)
	redisAddr := loadEnv("REDIS_ADDR", defaultRedis)

	// Initialize Redis client.
	redisClient = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // no password set
		DB:       0,
	})
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		slog.Error("❌ Failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	slog.Info("✅ Connected to Redis")

	// Initialize Fiber.
	app := fiber.New()
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${ip} ${method} ${path} ${status}\n",
	}))
	app.Use(limiter.New(limiter.Config{
		Max:        maxRequests,
		Expiration: requestTimeout,
	}))
	app.Use(cache.New(cache.Config{
		Expiration:   cacheExpiration,
		CacheControl: true,
	}))

	// Connect to PostgreSQL.
	var err error
	db, err = sqlx.Connect("postgres", dbConn)
	if err != nil {
		slog.Error("❌ Database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("✅ Connected to PostgreSQL")

	// Initialize Kafka Producer.
	producer, err = setupKafkaProducer(kafkaBrokers)
	if err != nil {
		slog.Error("❌ Kafka connection failed", "error", err)
		os.Exit(1)
	}
	defer producer.Close()
	slog.Info("✅ Connected to Kafka")

	// Create the Jobs table if it doesn't exist.
	createJobsTable()

	// Public Routes.
	app.Post("/login", login(secretKey))

	// Protected Routes (Require JWT).
	app.Use(jwtware.New(jwtware.Config{SigningKey: []byte(secretKey)}))
	app.Post("/jobs", createJob)
	app.Get("/jobs/:id", getJob)
	app.Get("/jobs", listJobs)

	// Monitoring Route.
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	// Graceful shutdown.
	go func() {
		slog.Info("🚀 Server running on port 8080")
		if err := app.Listen(serverPort); err != nil {
			slog.Error("❌ Server error", "error", err)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	slog.Info("🛑 Server shutting down...")
	time.Sleep(serverShutdown)
}

// setupKafkaProducer configures and returns a Kafka SyncProducer.
func setupKafkaProducer(broker string) (sarama.SyncProducer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.Retry.Max = kafkaRetryMax
	config.Producer.Retry.Backoff = kafkaRetryBackoff
	return sarama.NewSyncProducer([]string{broker}, config)
}

// createJobsTable ensures the jobs table exists.
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

// createJob handles job creation, sets the status in both PostgreSQL and Redis,
// and publishes the job to Kafka.
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

	// Insert job into PostgreSQL.
	var jobID int
	err := db.QueryRow("INSERT INTO jobs (name) VALUES ($1) RETURNING id", req.Name).Scan(&jobID)
	if err != nil {
		slog.Error("❌ Database insert error", "error", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to insert job"})
	}

	// Set job status to "pending" in Redis.
	ctx := context.Background()
	redisKey := fmt.Sprintf(redisKeyTemplate, jobID)
	if err := redisClient.Set(ctx, redisKey, statusPending, 0).Err(); err != nil {
		slog.Error("❌ Failed to set job status in Redis", "error", err)
		// Log error without blocking job creation.
	}

	// Publish job to Kafka.
	jobMsg, _ := json.Marshal(fiber.Map{"id": jobID, "name": req.Name})
	message := &sarama.ProducerMessage{
		Topic: kafkaTopic,
		Value: sarama.StringEncoder(jobMsg),
	}
	if _, _, err := producer.SendMessage(message); err != nil {
		slog.Error("❌ Failed to send job to Kafka", "error", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to send job to Kafka"})
	}

	return c.JSON(fiber.Map{
		"message": "✅ Job created successfully",
		"job":     fiber.Map{"id": jobID, "name": req.Name, "status": statusPending},
	})
}

// getJob retrieves a job from PostgreSQL and (optionally) updates its status from Redis.
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
	// Optionally update the status from Redis.
	ctx := context.Background()
	redisKey := fmt.Sprintf(redisKeyTemplate, job.ID)
	if redisStatus, err := redisClient.Get(ctx, redisKey).Result(); err == nil {
		job.Status = redisStatus
	}
	return c.JSON(job)
}

// listJobs retrieves all jobs from PostgreSQL and updates their statuses from Redis.
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
	// Optionally update each job's status from Redis.
	ctx := context.Background()
	for i, job := range jobs {
		redisKey := fmt.Sprintf(redisKeyTemplate, job.ID)
		if redisStatus, err := redisClient.Get(ctx, redisKey).Result(); err == nil {
			jobs[i].Status = redisStatus
		}
	}
	return c.JSON(fiber.Map{"jobs": jobs})
}

// login handles JWT-based login.
func login(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}
		if req.Username != adminUser || req.Password != adminPass {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"username": req.Username,
			"exp":      time.Now().Add(jwtExpiration).Unix(),
		})
		t, _ := token.SignedString([]byte(secret))
		return c.JSON(fiber.Map{"token": t, "type": "Bearer"})
	}
}
