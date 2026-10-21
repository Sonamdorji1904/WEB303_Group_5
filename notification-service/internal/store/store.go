// Package store holds the Notification Service's persistence layer.
package store

import (
	"context"
	"errors"
	"time"
)

const (
	RecipientPatient = "PATIENT"
	RecipientDoctor  = "DOCTOR"

	StatusSent   = "SENT"
	StatusFailed = "FAILED"
)

var ErrNotFound = errors.New("notification not found")

type Notification struct {
	ID             string     `json:"id"`
	EventID        string     `json:"eventId"`
	EventType      string     `json:"eventType"`
	RecipientType  string     `json:"recipientType"`
	RecipientID    string     `json:"recipientId"`
	RecipientName  string     `json:"recipientName"`
	RecipientEmail string     `json:"recipientEmail"`
	Channel        string     `json:"channel"`
	Subject        string     `json:"subject"`
	Message        string     `json:"message"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	SentAt         *time.Time `json:"sentAt,omitempty"`
	ReadAt         *time.Time `json:"readAt,omitempty"`
}

type ProcessedEvent struct {
	EventID     string    `json:"eventId"`
	EventType   string    `json:"eventType"`
	ProcessedAt time.Time `json:"processedAt"`
}

type Filter struct {
	RecipientType string
	RecipientID   string
	EventType     string
	UnreadOnly    bool
	Limit         int
}

// Store is implemented by Postgres in production and by a fake in tests.
type Store interface {
	// IsProcessed reports whether eventID was already fully processed. The
	// handler checks it first so a duplicate delivery does not re-send anything.
	IsProcessed(ctx context.Context, eventID string) (bool, error)

	// SaveEvent atomically records eventID as processed and stores the
	// notifications. It returns processed=false (and stores nothing) when the
	// event was already processed, which makes redelivery harmless.
	SaveEvent(ctx context.Context, eventID, eventType string, notifs []Notification) (processed bool, err error)

	List(ctx context.Context, f Filter) ([]Notification, error)
	Get(ctx context.Context, id string) (*Notification, error)
	MarkRead(ctx context.Context, id string) (*Notification, error)
	ListProcessedEvents(ctx context.Context, limit int) ([]ProcessedEvent, error)
	Ping(ctx context.Context) error
}
