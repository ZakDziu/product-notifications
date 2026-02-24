package products

import (
	"errors"
	"time"
)

var (
	ErrNotFound    = errors.New("product not found")
	ErrInvalidName = errors.New("product name is required")
)

const (
	EventsQueue  = "products.events"
	EventCreated = "product_created"
	EventDeleted = "product_deleted"
)

type Product struct {
	ID        int64     `json:"id" example:"1"`
	Name      string    `json:"name" example:"iPhone 16"`
	CreatedAt time.Time `json:"created_at" example:"2026-02-24T12:00:00Z"`
}

type ProductEvent struct {
	EventType string    `json:"event_type"`
	ProductID int64     `json:"product_id"`
	Name      string    `json:"name,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
