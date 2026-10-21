package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	Exchange   = "hospital.events"
	Queue      = "appointment-service.queue"
	RetryQueue = "appointment-service.retry"
	DLQ        = "appointment-service.dlq"
	MaxRetries = 3
)

type Broker struct {
	conn    *amqp.Connection
	ch      *amqp.Channel
	returns chan amqp.Return
	stop    func() bool
}

func Connect(ctx context.Context, url string) (*Broker, error) {
	var stop func() bool
	conn, err := amqp.DialConfig(url, amqp.Config{
		Heartbeat: 10 * time.Second,
		Dial: func(network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			c, err := d.DialContext(ctx, network, address)
			if err != nil {
				return nil, err
			}
			stop = context.AfterFunc(ctx, func() { _ = c.Close() })
			return c, nil
		},
	})
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	b := &Broker{conn: conn, ch: ch, stop: stop}
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

func (b *Broker) Close() {
	if b.stop != nil {
		b.stop()
	}
	_ = b.conn.Close()
}

func (b *Broker) topology() error {
	if err := b.ch.ExchangeDeclare(Exchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := b.ch.QueueDeclare(DLQ, true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := b.ch.QueueDeclare(Queue, true, false, false, false, nil); err != nil {
		return err
	}
	for _, key := range []string{"SlotReserved", "SlotRejected"} {
		if err := b.ch.QueueBind(Queue, key, Exchange, false, nil); err != nil {
			return err
		}
	}
	_, err := b.ch.QueueDeclare(RetryQueue, true, false, false, false, amqp.Table{
		"x-message-ttl":             int32(5000),
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": Queue,
	})
	return err
}

func (b *Broker) publish(ctx context.Context, exchange, key string, msg amqp.Publishing) error {
	confirm, err := b.ch.PublishWithDeferredConfirmWithContext(ctx, exchange, key, true, false, msg)
	if err != nil {
		return err
	}
	ok, err := confirm.WaitContext(ctx)
	if err != nil {
		return err
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
		return err
	}
	err = b.publish(ctx, Exchange, e.EventType, amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Persistent,
		MessageId: e.EventID, Type: e.EventType, Timestamp: e.Timestamp, Body: body,
	})
	if err != nil {
		slog.Error("event publish failed", "eventId", e.EventID, "eventType", e.EventType, "error", err)
		return err
	}
	slog.Info("event published", "eventId", e.EventID, "eventType", e.EventType)
	return nil
}

func retryCount(h amqp.Table) int {
	switch v := h["x-retry-count"].(type) {
	case int32:
		return int(v)
	case int64:
		return int(v)
	case int:
		return v
	}
	return 0
}

func eventID(body []byte) string {
	var e Event
	_ = json.Unmarshal(body, &e)
	return e.EventID
}

func (b *Broker) consumeOne(ctx context.Context, s *Store, d amqp.Delivery) error {
	op, cancel := context.WithTimeout(ctx, 30*time.Second)
	var err error
	if d.RoutingKey == "SlotRejected" {
		err = s.ProcessRejected(op, d.Body)
	} else {
		err = s.ProcessReserved(op, d.Body)
	}
	cancel()
	id := eventID(d.Body)
	if err == nil {
		if err = d.Ack(false); err != nil {
			return err
		}
		slog.Info("event consumed", "eventId", id, "eventType", d.RoutingKey)
		return nil
	}
	n := retryCount(d.Headers)
	if n >= MaxRetries {
		if err = b.publish(ctx, "", DLQ, amqp.Publishing{
			ContentType: "application/json", DeliveryMode: amqp.Persistent,
			MessageId: d.MessageId, Type: d.Type,
			Headers: amqp.Table{"x-retry-count": int32(n), "x-last-error": err.Error()},
			Body:    d.Body,
		}); err != nil {
			return err
		}
		if err = d.Ack(false); err != nil {
			return err
		}
		slog.Error("event dead-lettered", "eventId", id, "eventType", d.RoutingKey, "error", err)
		return nil
	}
	if err = b.publish(ctx, "", RetryQueue, amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Persistent,
		MessageId: d.MessageId, Type: d.Type,
		Headers: amqp.Table{"x-retry-count": int32(n + 1)}, Body: d.Body,
	}); err != nil {
		return err
	}
	if err = d.Ack(false); err != nil {
		return err
	}
	slog.Warn("event retry scheduled", "eventId", id, "eventType", d.RoutingKey, "retry", n+1)
	return nil
}

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
			if err = b.consumeOne(ctx, s, d); err != nil {
				return err
			}
		}
	}
}

func RunConsumer(ctx context.Context, s *Store, url string) {
	for ctx.Err() == nil {
		b, err := Connect(ctx, url)
		if err == nil {
			err = b.Consume(ctx, s)
			b.Close()
		}
		if ctx.Err() == nil {
			slog.Warn("consumer reconnecting", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
		}
	}
}

func RunOutbox(ctx context.Context, s *Store, url string) {
	for ctx.Err() == nil {
		b, err := Connect(ctx, url)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
			continue
		}
		for ctx.Err() == nil {
			op, cancel := context.WithTimeout(ctx, 15*time.Second)
			found, dispatchErr := s.Dispatch(op, b.Publish)
			cancel()
			if dispatchErr != nil {
				slog.Error("outbox dispatch failed", "error", dispatchErr)
				break
			}
			if !found {
				select {
				case <-ctx.Done():
					break
				case <-time.After(time.Second):
				}
			}
		}
		b.Close()
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}
