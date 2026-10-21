package service

import (
	"context"
	"encoding/json"
	"errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"log/slog"
	"net"
	"time"
)

const Exchange = "hospital.events"
const Queue = "patient-service.appointment-completed"
const RetryQueue = Queue + ".retry"
const DLQ = Queue + ".dlq"
const DeadExchange = "patient-service.dead"
const MaxRetries = 5

// Each worker owns its connection and publisher channel, so confirms are ordered.
type Broker struct {
	conn       *amqp.Connection
	ch         *amqp.Channel
	returns    chan amqp.Return
	stopCancel func() bool
}

func Connect(ctx context.Context, url string) (*Broker, error) {
	var stopCancel func() bool
	conn, err := amqp.DialConfig(url, amqp.Config{Heartbeat: 10 * time.Second, Dial: func(network, address string) (net.Conn, error) {
		d := net.Dialer{Timeout: 5 * time.Second}
		socket, err := d.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		// amqp clears this deadline after completing its protocol handshake.
		if err = socket.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
			socket.Close()
			return nil, err
		}
		stopCancel = context.AfterFunc(ctx, func() { _ = socket.Close() })
		return socket, nil
	}})
	if err != nil {
		if stopCancel != nil {
			stopCancel()
		}
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		stopCancel()
		conn.CloseDeadline(time.Now().Add(2 * time.Second))
		return nil, err
	}
	b := &Broker{conn: conn, ch: ch, stopCancel: stopCancel}
	if err = b.topology(); err != nil {
		b.Close()
		return nil, err
	}
	if err = ch.Confirm(false); err != nil {
		b.Close()
		return nil, err
	}
	b.returns = ch.NotifyReturn(make(chan amqp.Return, 1))
	return b, nil
}
func (b *Broker) Close() { b.stopCancel(); _ = b.conn.CloseDeadline(time.Now().Add(2 * time.Second)) }
func (b *Broker) topology() error {
	for _, x := range []struct{ name, kind string }{{Exchange, "topic"}, {DeadExchange, "direct"}} {
		if e := b.ch.ExchangeDeclare(x.name, x.kind, true, false, false, false, nil); e != nil {
			return e
		}
	}
	if _, e := b.ch.QueueDeclare(DLQ, true, false, false, false, amqp.Table{"x-queue-type": "quorum"}); e != nil {
		return e
	}
	if e := b.ch.QueueBind(DLQ, "failed", DeadExchange, false, nil); e != nil {
		return e
	}
	if _, e := b.ch.QueueDeclare(Queue, true, false, false, false, amqp.Table{"x-queue-type": "quorum", "x-overflow": "reject-publish", "x-dead-letter-strategy": "at-least-once", "x-dead-letter-exchange": DeadExchange, "x-dead-letter-routing-key": "failed"}); e != nil {
		return e
	}
	if e := b.ch.QueueBind(Queue, "AppointmentCompleted", Exchange, false, nil); e != nil {
		return e
	}
	_, e := b.ch.QueueDeclare(RetryQueue, true, false, false, false, amqp.Table{"x-queue-type": "quorum", "x-overflow": "reject-publish", "x-dead-letter-strategy": "at-least-once", "x-message-ttl": int32(5000), "x-dead-letter-exchange": "", "x-dead-letter-routing-key": Queue})
	return e
}
func (b *Broker) publish(ctx context.Context, exchange, key string, msg amqp.Publishing) error {
	c, e := b.ch.PublishWithDeferredConfirmWithContext(ctx, exchange, key, true, false, msg)
	if e != nil {
		return e
	}
	ok, e := c.WaitContext(ctx)
	if e != nil {
		return e
	}
	if !ok {
		return errors.New("broker nack")
	}
	select {
	case <-b.returns:
		return errors.New("unroutable message")
	default:
		return nil
	}
}
func (b *Broker) Publish(ctx context.Context, e Event) error {
	body, err := json.Marshal(e)
	if err != nil {
		slog.Error("event publish failed", "eventId", e.EventID, "eventType", e.EventType, "error", err)
		return err
	}
	err = b.publish(ctx, Exchange, e.EventType, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, MessageId: e.EventID, Type: e.EventType, Timestamp: e.Timestamp, Body: body})
	if err != nil {
		slog.Error("event publish failed", "eventId", e.EventID, "eventType", e.EventType, "error", err)
		return err
	}
	slog.Info("event published", "eventId", e.EventID, "eventType", e.EventType)
	return nil
}
func retryCount(h amqp.Table) int {
	switch v := h["x-patient-retries"].(type) {
	case int32:
		if v > 0 {
			return int(v)
		}
	case int64:
		if v > 0 {
			return int(v)
		}
	case int:
		if v > 0 {
			return v
		}
	}
	return 0
}
func Retryable(err error) bool { return err != nil && !errors.Is(err, ErrInvalid) }
func (b *Broker) Consume(ctx context.Context, s *Store) error {
	ch, err := b.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	if err = ch.Qos(1, 0, false); err != nil {
		return err
	}
	ds, err := ch.Consume(Queue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-ds:
			if !ok {
				return errors.New("consumer disconnected")
			}
			op, cancel := context.WithTimeout(ctx, 10*time.Second)
			var processErr error
			if len(d.Body) > 1024*1024 {
				processErr = ErrInvalid
			} else {
				processErr = s.Process(op, d.Body)
			}
			cancel()
			if processErr == nil {
				if err = d.Ack(false); err != nil {
					return err
				}
				var event Event
				_ = json.Unmarshal(d.Body, &event)
				slog.Info("event consumed", "eventId", event.EventID, "eventType", event.EventType)
				continue
			}
			n := retryCount(d.Headers)
			var event Event
			_ = json.Unmarshal(d.Body, &event)
			if !Retryable(processErr) || n >= MaxRetries {
				if err = d.Reject(false); err != nil {
					return err
				}
				slog.Warn("appointment delivery dead-lettered", "eventId", event.EventID, "eventType", event.EventType, "reason", failureClass(processErr), "retry", n)
				continue
			}
			headers := amqp.Table{"x-patient-retries": int32(n + 1)}
			op, cancel = context.WithTimeout(ctx, 10*time.Second)
			err = b.publish(op, "", RetryQueue, amqp.Publishing{Headers: headers, ContentType: "application/json", DeliveryMode: amqp.Persistent, MessageId: d.MessageId, Type: d.Type, Timestamp: d.Timestamp, Body: d.Body})
			cancel()
			if err != nil {
				slog.Error("appointment delivery retry publish failed", "eventId", event.EventID, "eventType", event.EventType, "error", err, "retry", n+1)
				return err
			} // Connection closes, returning the unacknowledged original to its queue.
			if err = d.Ack(false); err != nil {
				return err
			}
			slog.Warn("appointment delivery retry scheduled", "eventId", event.EventID, "eventType", event.EventType, "retry", n+1)
		}
	}
}
func failureClass(e error) string {
	if errors.Is(e, ErrInvalid) {
		return "invalid_event"
	}
	if errors.Is(e, ErrMissingPatient) {
		return "missing_patient"
	}
	return "database_unavailable"
}
func pause(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
func RunConsumer(ctx context.Context, s *Store, url string) {
	for ctx.Err() == nil {
		b, e := Connect(ctx, url)
		if e == nil {
			e = b.Consume(ctx, s)
			b.Close()
		}
		if ctx.Err() != nil {
			return
		}
		slog.Warn("consumer reconnecting")
		if !pause(ctx, 3*time.Second) {
			return
		}
	}
}
func RunOutbox(ctx context.Context, s *Store, url string) {
	for ctx.Err() == nil {
		b, e := Connect(ctx, url)
		if e != nil {
			if !pause(ctx, 3*time.Second) {
				return
			}
			continue
		}
		for ctx.Err() == nil {
			op, cancel := context.WithTimeout(ctx, 12*time.Second)
			var pubErr error
			found, err := s.Dispatch(op, func(c context.Context, e Event) error { pubErr = b.Publish(c, e); return pubErr })
			cancel()
			if err != nil || pubErr != nil {
				break
			}
			if !found && !pause(ctx, time.Second) {
				break
			}
		}
		b.Close()
		if !pause(ctx, 3*time.Second) {
			return
		}
	}
}
