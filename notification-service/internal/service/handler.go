// Package service turns incoming events into stored, delivered notifications.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"hospital-system/notification-service/internal/events"
	"hospital-system/notification-service/internal/messaging"
	"hospital-system/notification-service/internal/sender"
	"hospital-system/notification-service/internal/store"
)

type Handler struct {
	store  store.Store
	sender sender.Sender
	now    func() time.Time
}

func New(s store.Store, snd sender.Sender) *Handler {
	return &Handler{store: s, sender: snd, now: time.Now}
}

// Handle processes one raw message body from the queue.
//
//   - returns nil          -> message is acknowledged (processed or duplicate)
//   - returns ErrPermanent -> message is dead-lettered without retries
//   - returns other error  -> message is retried, then dead-lettered
func (h *Handler) Handle(ctx context.Context, body []byte) error {
	var env events.Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return messaging.Permanent(fmt.Errorf("invalid event JSON: %w", err))
	}
	if env.EventID == "" || env.EventType == "" {
		return messaging.Permanent(errors.New("event is missing eventId or eventType"))
	}

	log := slog.With("eventId", env.EventID, "eventType", env.EventType)

	var data events.Data
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, &data); err != nil {
			return messaging.Permanent(fmt.Errorf("invalid event data: %w", err))
		}
	}

	drafts, err := build(env.EventType, data)
	if err != nil {
		return messaging.Permanent(err)
	}

	// Idempotency: an event we already handled must not send anything again.
	done, err := h.store.IsProcessed(ctx, env.EventID)
	if err != nil {
		return fmt.Errorf("check processed event: %w", err)
	}
	if done {
		log.Warn("duplicate event ignored", "reason", "already processed")
		return nil
	}

	notifs := make([]store.Notification, 0, len(drafts))
	for _, d := range drafts {
		n := store.Notification{
			EventID: env.EventID, EventType: env.EventType,
			RecipientType: d.recipientType, RecipientID: d.recipientID,
			RecipientName: d.recipientName, RecipientEmail: d.email,
			Channel: "EMAIL", Subject: d.subject, Message: d.message,
		}
		if err := h.sender.Send(ctx, n); err != nil {
			// Transient: nothing has been stored yet, so a retry is safe.
			return fmt.Errorf("send notification to %s %s: %w", d.recipientType, d.recipientID, err)
		}
		sent := h.now()
		n.Status, n.SentAt = store.StatusSent, &sent
		notifs = append(notifs, n)
	}

	processed, err := h.store.SaveEvent(ctx, env.EventID, env.EventType, notifs)
	if err != nil {
		return fmt.Errorf("save notifications: %w", err)
	}
	if !processed {
		// Lost a race with a concurrent delivery of the same event.
		log.Warn("duplicate event ignored", "reason", "processed concurrently")
		return nil
	}
	log.Info("event processed", "notifications", len(notifs))
	return nil
}
