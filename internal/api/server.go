package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/IBM/sarama"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	jwtware "github.com/gofiber/jwt/v3"

	"github.com/illegalcall/task-master/internal/config"
	"github.com/illegalcall/task-master/internal/models"
	"github.com/illegalcall/task-master/pkg/database"
	"github.com/illegalcall/task-master/internal/storage"
)

type Server struct {
	app      *fiber.App
	cfg      *config.Config
	db       *database.Clients
	producer sarama.SyncProducer
	storage  storage.Storage
}

func NewServer(cfg *config.Config, db *database.Clients, producer sarama.SyncProducer) (*Server, error) {
	// Initialize storage
	localStorage, err := storage.NewLocalStorage(cfg.Storage.TempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	app := fiber.New()

	// Middleware
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${ip} ${method} ${path} ${status}\n",
	}))
	app.Use(limiter.New(limiter.Config{
		Max:        cfg.Server.MaxRequests,
		Expiration: cfg.Server.RequestTimeout,
	}))
	app.Use(cache.New(cache.Config{
		Expiration:   cfg.Server.CacheExpiration,
		CacheControl: true,
	}))

	server := &Server{
		app:      app,
		cfg:      cfg,
		db:       db,
		producer: producer,
		storage:  localStorage,
	}

	// Routes
	server.setupRoutes()

	return server, nil
}

func (s *Server) setupRoutes() {
	api := s.app.Group("/api")

	// Public routes
	api.Post("/login", s.handleLogin)

	// Protected routes
	protected := api.Use(jwtware.New(jwtware.Config{
		SigningKey: []byte(s.cfg.JWT.Secret),
	}))
	protected.Post("/jobs", s.handleCreateJob)
	protected.Get("/jobs/:id", s.handleGetJob)
	protected.Get("/jobs", s.handleListJobs)
	protected.Post("/jobs/parse-document", s.handlePDFParseJob)
}

func (s *Server) Start() error {
	return s.app.Listen(s.cfg.Server.Port)
}

func (s *Server) handleCreateJob(c *fiber.Ctx) error {
	// Parse request
	var req struct {
		Name string `json:"name"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Job name is required",
		})
	}

	// Insert job into database
	var jobID int
	err := s.db.DB.QueryRow(
		"INSERT INTO jobs (name, status) VALUES ($1, $2) RETURNING id",
		req.Name, models.StatusPending,
	).Scan(&jobID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create job",
		})
	}

	// Create job object
	job := models.Job{
		ID:     jobID,
		Name:   req.Name,
		Status: models.StatusPending,
	}

	// Set initial status in Redis
	redisKey := fmt.Sprintf("job:%d", jobID)
	if err := s.db.Redis.Set(c.Context(), redisKey, models.StatusPending, 0).Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to set job status",
		})
	}

	// Send to Kafka
	jobBytes, _ := json.Marshal(job)
	msg := &sarama.ProducerMessage{
		Topic: s.cfg.Kafka.Topic,
		Value: sarama.StringEncoder(jobBytes),
	}
	if _, _, err := s.producer.SendMessage(msg); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to queue job",
		})
	}

	return c.JSON(fiber.Map{
		"job": job,
	})
}

func (s *Server) handleGetJob(c *fiber.Ctx) error {
	jobID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid job ID",
		})
	}

	var job models.Job
	query := "SELECT id, name, status FROM jobs WHERE id = $1"
	err = s.db.DB.Get(&job, query, jobID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Job not found",
		})
	}

	// Update status from Redis
	redisKey := fmt.Sprintf("job:%d", job.ID)
	if redisStatus, err := s.db.Redis.Get(c.Context(), redisKey).Result(); err == nil {
		job.Status = redisStatus
	}

	return c.JSON(fiber.Map{
		"job": job,
	})
}

func (s *Server) handleListJobs(c *fiber.Ctx) error {
	var jobs []models.Job
	err := s.db.DB.Select(&jobs, "SELECT id, name, status FROM jobs ORDER BY created_at DESC")
	if err != nil {
		slog.Error("Error fetching jobs", "error", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch jobs"})
	}

	// Update statuses from Redis
	ctx := context.Background()
	for i, job := range jobs {
		redisKey := fmt.Sprintf("job:%d", job.ID)
		if redisStatus, err := s.db.Redis.Get(ctx, redisKey).Result(); err == nil {
			jobs[i].Status = redisStatus
		}
	}

	return c.JSON(fiber.Map{"jobs": jobs})
}
