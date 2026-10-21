package service

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestRetryCount(t *testing.T) {
	if retryCount(amqp.Table{"x-retry-count": int32(2)}) != 2 {
		t.Fatal("retry count was not decoded")
	}
	if retryCount(nil) != 0 {
		t.Fatal("missing retry count should be zero")
	}
}

func TestRequestedEventValidation(t *testing.T) {
	body := `{"eventId":"00000000-0000-0000-0000-000000000001","eventType":"AppointmentRequested","timestamp":"2026-10-20T10:00:00Z","data":{"appointmentId":"00000000-0000-0000-0000-000000000002","patientId":"00000000-0000-0000-0000-000000000003","patientName":"P","doctorId":"00000000-0000-0000-0000-000000000004","date":"2026-11-01","time":"09:30"}}`
	if _, _, err := parseRequested([]byte(body)); err != nil {
		t.Fatal(err)
	}
}
