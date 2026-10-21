// Package sender delivers a notification to its recipient.
//
// The lab does not need a real mail/SMS provider, so the default sender
// writes the message to the service log. To use a real provider, implement
// Sender (e.g. SMTP) and pass it to service.New in cmd/server/main.go.
package sender

import (
	"context"
	"log/slog"

	"hospital-system/notification-service/internal/store"
)

type Sender interface {
	Send(ctx context.Context, n store.Notification) error
}

// LogSender "sends" notifications by logging them.
type LogSender struct{}

func (LogSender) Send(_ context.Context, n store.Notification) error {
	slog.Info("notification sent",
		"channel", n.Channel,
		"recipientType", n.RecipientType,
		"recipientId", n.RecipientID,
		"to", n.RecipientEmail,
		"subject", n.Subject,
		"message", n.Message,
		"eventId", n.EventID)
	return nil
}
