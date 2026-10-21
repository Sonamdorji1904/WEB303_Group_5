package service

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestRetryCount(t *testing.T) {
	if retryCount(amqp.Table{"x-retry-count": int32(3)}) != 3 {
		t.Fatal("retry count was not decoded")
	}
	if retryCount(nil) != 0 {
		t.Fatal("missing retry count should be zero")
	}
}

func TestQueueNamesAndEventTypes(t *testing.T) {
	if Queue != "appointment-service.queue" || RetryQueue != "appointment-service.retry" || DLQ != "appointment-service.dlq" {
		t.Fatal("unexpected queue topology")
	}
}
