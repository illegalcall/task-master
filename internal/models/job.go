package models

import "time"

type Job struct {
	ID        int       `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Status    string    `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

const (
	StatusPending   = "pending"
	StatusFailed    = "failed"
	StatusCompleted = "completed"
)

type Result struct {
	Message string `json:"message"`
	Data    interface{} `json:"data"`
}

type JobHandlerFunc func(payload []byte) (Result, error)