package messaging

import (
	"errors"
	"fmt"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestRetryCount(t *testing.T) {
	cases := []struct {
		name string
		h    amqp.Table
		want int
	}{
		{"missing", amqp.Table{}, 0},
		{"nil table", nil, 0},
		{"int32", amqp.Table{"x-retry-count": int32(2)}, 2},
		{"int64", amqp.Table{"x-retry-count": int64(3)}, 3},
		{"wrong type", amqp.Table{"x-retry-count": "2"}, 0},
	}
	for _, c := range cases {
		if got := retryCount(c.h); got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
}

func TestPermanentError(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", Permanent(errors.New("bad payload")))
	if !errors.Is(err, ErrPermanent) {
		t.Fatal("wrapped permanent error should match ErrPermanent")
	}
	if errors.Is(errors.New("db down"), ErrPermanent) {
		t.Fatal("ordinary errors are not permanent")
	}
}

func TestQueueNames(t *testing.T) {
	c := NewConsumer(Config{Queue: "notification-service.queue"}, nil)
	if c.retryQ() != "notification-service.retry" || c.dlq() != "notification-service.dlq" {
		t.Fatalf("unexpected names: %s %s", c.retryQ(), c.dlq())
	}
}
