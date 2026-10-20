// Package messaging is the RabbitMQ consumer used by the Notification Service.
//
// Reliability features (Lab 6, Part E):
//   - durable exchange + durable queue, persistent messages  -> nothing lost while the service is down
//   - manual acknowledgements                                 -> a message is only removed once processed
//   - retry queue with a delay, then a dead-letter queue      -> failures are retried, poison messages parked
//   - automatic reconnect with backoff                        -> processing resumes after RabbitMQ/service restarts
//
// Topology created for queue "<name>.queue":
//
//	hospital.events (topic) --keys--> <name>.queue
//	<name>.queue      failed message -> publish to <name>.retry (per-message TTL)
//	<name>.retry      TTL expires    -> dead-lettered back into <name>.queue
//	<name>.dlq        retries exhausted or permanent error
package messaging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Handler processes one message body. See service.Handler.Handle for the
// meaning of the returned error.
type Handler func(ctx context.Context, body []byte) error

type Config struct {
	URL         string
	Exchange    string
	Queue       string
	BindingKeys []string
	MaxRetries  int
	RetryDelay  time.Duration
	Prefetch    int
}

type Consumer struct {
	cfg     Config
	handler Handler
	ready   atomic.Bool
}

func NewConsumer(cfg Config, h Handler) *Consumer {
	if cfg.Prefetch <= 0 {
		cfg.Prefetch = 5
	}
	return &Consumer{cfg: cfg, handler: h}
}

// Ready reports whether the consumer is currently connected and consuming.
func (c *Consumer) Ready() bool { return c.ready.Load() }

func (c *Consumer) base() string   { return strings.TrimSuffix(c.cfg.Queue, ".queue") }
func (c *Consumer) retryQ() string { return c.base() + ".retry" }
func (c *Consumer) dlq() string    { return c.base() + ".dlq" }

// Run keeps consuming until ctx is cancelled, reconnecting after failures.
func (c *Consumer) Run(ctx context.Context) {
	backoff := time.Second
	for {
		started, err := c.session(ctx)
		c.ready.Store(false)
		if ctx.Err() != nil {
			return
		}
		if started {
			backoff = time.Second
		}
		slog.Error("rabbitmq session ended; reconnecting", "error", err, "retryIn", backoff.String())
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (c *Consumer) session(ctx context.Context) (started bool, err error) {
	conn, err := amqp.Dial(c.cfg.URL)
	if err != nil {
		return false, fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return false, fmt.Errorf("channel: %w", err)
	}
	defer ch.Close()

	if err := c.declare(ch); err != nil {
		return false, fmt.Errorf("declare topology: %w", err)
	}
	if err := ch.Qos(c.cfg.Prefetch, 0, false); err != nil {
		return false, fmt.Errorf("qos: %w", err)
	}
	// Publisher confirms: a message moved to the retry/DLQ queue is only
	// acknowledged on the main queue once the broker has confirmed it.
	if err := ch.Confirm(false); err != nil {
		return false, fmt.Errorf("confirm mode: %w", err)
	}

	deliveries, err := ch.Consume(c.cfg.Queue, c.base()+"-consumer", false, false, false, false, nil)
	if err != nil {
		return false, fmt.Errorf("consume: %w", err)
	}
	closed := conn.NotifyClose(make(chan *amqp.Error, 1))

	c.ready.Store(true)
	slog.Info("consuming events", "queue", c.cfg.Queue, "exchange", c.cfg.Exchange, "keys", c.cfg.BindingKeys)

	for {
		select {
		case <-ctx.Done():
			return true, nil
		case e := <-closed:
			if e == nil {
				return true, errors.New("connection closed")
			}
			return true, fmt.Errorf("connection closed: %w", e)
		case d, ok := <-deliveries:
			if !ok {
				return true, errors.New("delivery channel closed")
			}
			c.process(ctx, ch, d)
		}
	}
}

func (c *Consumer) declare(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(c.cfg.Exchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(c.cfg.Queue, true, false, false, false, nil); err != nil {
		return err
	}
	for _, key := range c.cfg.BindingKeys {
		if err := ch.QueueBind(c.cfg.Queue, key, c.cfg.Exchange, false, nil); err != nil {
			return err
		}
	}
	// Retry queue: messages wait here (per-message TTL) and are then
	// dead-lettered through the default exchange straight back to the main
	// queue, so other services' queues never see them again.
	if _, err := ch.QueueDeclare(c.retryQ(), true, false, false, false, amqp.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": c.cfg.Queue,
	}); err != nil {
		return err
	}
	_, err := ch.QueueDeclare(c.dlq(), true, false, false, false, nil)
	return err
}

func (c *Consumer) process(ctx context.Context, ch *amqp.Channel, d amqp.Delivery) {
	hctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err := c.safeHandle(hctx, d.Body)
	if err == nil {
		_ = d.Ack(false)
		return
	}

	retries := retryCount(d.Headers)
	var moveErr error
	switch {
	case errors.Is(err, ErrPermanent):
		slog.Error("permanent failure; sending to DLQ", "error", err, "queue", c.dlq(), "messageId", d.MessageId)
		moveErr = c.republish(hctx, ch, d, c.dlq(), retries, err, 0)
	case retries >= c.cfg.MaxRetries:
		slog.Error("retries exhausted; sending to DLQ", "error", err, "retries", retries, "queue", c.dlq(), "messageId", d.MessageId)
		moveErr = c.republish(hctx, ch, d, c.dlq(), retries, err, 0)
	default:
		slog.Warn("processing failed; scheduling retry", "error", err,
			"attempt", retries+1, "maxRetries", c.cfg.MaxRetries, "delay", c.cfg.RetryDelay.String())
		moveErr = c.republish(hctx, ch, d, c.retryQ(), retries+1, err, c.cfg.RetryDelay)
	}

	if moveErr != nil {
		// Could not park the message: put it back so it is not lost.
		slog.Error("could not move message; requeueing", "error", moveErr)
		_ = d.Nack(false, true)
		return
	}
	_ = d.Ack(false)
}

func (c *Consumer) safeHandle(ctx context.Context, body []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panic: %v", r)
		}
	}()
	return c.handler(ctx, body)
}

// republish copies the message to queue (via the default exchange) and waits
// for the broker to confirm it.
func (c *Consumer) republish(ctx context.Context, ch *amqp.Channel, d amqp.Delivery, queue string, retries int, cause error, delay time.Duration) error {
	headers := amqp.Table{}
	for k, v := range d.Headers {
		headers[k] = v
	}
	headers["x-retry-count"] = int32(retries)
	headers["x-last-error"] = cause.Error()
	headers["x-failed-at"] = time.Now().UTC().Format(time.RFC3339)
	if _, ok := headers["x-original-routing-key"]; !ok {
		headers["x-original-routing-key"] = d.RoutingKey
	}

	pub := amqp.Publishing{
		Headers:      headers,
		ContentType:  d.ContentType,
		DeliveryMode: amqp.Persistent,
		MessageId:    d.MessageId,
		Timestamp:    d.Timestamp,
		Body:         d.Body,
	}
	if delay > 0 {
		pub.Expiration = strconv.FormatInt(delay.Milliseconds(), 10)
	}

	conf, err := ch.PublishWithDeferredConfirmWithContext(ctx, "", queue, true, false, pub)
	if err != nil {
		return err
	}
	ok, err := conf.WaitContext(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("broker did not confirm republished message")
	}
	return nil
}

func retryCount(h amqp.Table) int {
	switch v := h["x-retry-count"].(type) {
	case int:
		return v
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint8:
		return int(v)
	case uint16:
		return int(v)
	case uint32:
		return int(v)
	}
	return 0
}
