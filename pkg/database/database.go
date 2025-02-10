package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

type Clients struct {
	DB    *sqlx.DB
	Redis *redis.Client
}

func NewClients(dbURL, redisAddr string) (*Clients, error) {
	// Connect to PostgreSQL
	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       0,
	})

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Clients{
		DB:    db,
		Redis: redisClient,
	}, nil
}

func (c *Clients) CreateJobsTable() error {
	schema := `CREATE TABLE IF NOT EXISTS jobs (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		status TEXT DEFAULT 'pending',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := c.DB.Exec(schema); err != nil {
		return fmt.Errorf("failed to create jobs table: %w", err)
	}

	slog.Info("✅ Jobs table is ready!")
	return nil
}
