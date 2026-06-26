package models

import "time"

// Task represents a task in the system.
type Task struct {
	Handle      string    `json:"handle"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"`  // todo, in_progress, done
	Priority    string    `json:"priority"` // low, medium, high
	Workspace   string    `json:"workspace"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Workspace represents a tenant in the system.
type Workspace struct {
	Handle string `json:"handle"`
	Name   string `json:"name"`
	Plan   string `json:"plan"`
}

// Token represents an auth token.
type Token struct {
	Value     string    `json:"value"`
	Workspace string    `json:"workspace"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
