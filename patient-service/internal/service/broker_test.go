package service

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Use only a dedicated, disposable vhost: this test purges service-owned queues.
func TestRabbitMQWorkflow(t *testing.T) {
	url := os.Getenv("TEST_RABBITMQ_URL")
	if url == "" {
		t.Skip("TEST_RABBITMQ_URL unset: live RabbitMQ retry/confirm/DLQ/restart checks not run")
	}
	s := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 65*time.Second)
	defer cancel()
	b, e := Connect(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	for _, q := range []string{Queue, RetryQueue, DLQ} {
		if _, e = b.ch.QueuePurge(q, false); e != nil {
			t.Fatal(e)
		}
	}
	sink, e := b.ch.QueueDeclare("", false, true, true, false, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, key := range []string{"PatientRegistered", "MedicalRecordCreated"} {
		if e = b.ch.QueueBind(sink.Name, key, Exchange, false, nil); e != nil {
			t.Fatal(e)
		}
	}
	p := patient(t, s)
	outCtx, stopOut := context.WithCancel(ctx)
	outDone := make(chan struct{})
	go func() { defer close(outDone); RunOutbox(outCtx, s, url) }()
	defer func() { stopOut(); <-outDone }()
	startConsumer := func() (context.CancelFunc, chan struct{}) {
		c, stop := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() { defer close(done); RunConsumer(c, s, url) }()
		return stop, done
	}
	stopConsumer, consumerDone := startConsumer()
	defer func() { stopConsumer(); <-consumerDone }()
	send := func(body []byte) {
		t.Helper()
		if e = b.publish(ctx, Exchange, "AppointmentCompleted", amqp.Publishing{DeliveryMode: amqp.Persistent, ContentType: "application/json", Body: body}); e != nil {
			t.Fatal(e)
		}
	}
	wait := func(timeout time.Duration, condition func() bool) {
		t.Helper()
		until := time.Now().Add(timeout)
		for time.Now().Before(until) {
			if condition() {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatal("timed out waiting for RabbitMQ workflow")
	}
	body := appointment(p.ID)
	send(body)
	wait(10*time.Second, func() bool { return count(t, s, "medical_records") == 1 })
	send(body)
	var ev Event
	_ = json.Unmarshal(body, &ev)
	ev.EventID = uuid.NewString()
	other, _ := json.Marshal(ev)
	send(other)
	wait(10*time.Second, func() bool { return count(t, s, "processed_events") == 2 })
	if count(t, s, "medical_records") != 1 || count(t, s, "outbox") != 2 {
		t.Fatal("broker redelivery created duplicates")
	}
	wait(10*time.Second, func() bool {
		var n int
		_ = s.DB.QueryRow(ctx, "SELECT count(*) FROM outbox WHERE published_at IS NOT NULL").Scan(&n)
		return n == 2
	})
	// Explicit returns must be treated as failed publication even with a broker ack.
	if e = b.Publish(ctx, newEvent("UnboundTestEvent", map[string]string{})); e == nil {
		t.Fatal("unroutable publish reported success")
	}
	// Stop the consumer, enqueue another visit, and prove the durable queue survives
	// a service worker restart (a broker process restart is a separate manual check).
	stopConsumer()
	<-consumerDone
	send(appointment(p.ID))
	stopConsumer, consumerDone = startConsumer()
	wait(10*time.Second, func() bool { return count(t, s, "medical_records") == 2 })
	send([]byte(`{"bad":"event"}`))
	send(appointment(uuid.NewString()))
	wait(40*time.Second, func() bool { q, err := b.ch.QueueInspect(DLQ); return err == nil && q.Messages >= 2 })
	if count(t, s, "processed_events") != 3 || count(t, s, "medical_records") != 2 {
		t.Fatal("invalid/missing-patient delivery changed database")
	}
	seenInvalid, seenMissing := false, false
	for i := 0; i < 2; i++ {
		d, ok, e := b.ch.Get(DLQ, false)
		if e != nil || !ok {
			t.Fatal("DLQ retrieval failed")
		}
		if retryCount(d.Headers) == MaxRetries {
			seenMissing = true
		} else {
			seenInvalid = true
		}
		_ = d.Ack(false)
	}
	if !seenInvalid || !seenMissing {
		t.Fatal("invalid or bounded missing-patient DLQ policy failed")
	}
}

// A TCP peer that never speaks AMQP must not prevent graceful cancellation.
func TestConnectCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, e := listener.Accept()
		if e == nil {
			accepted <- c
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		b, e := Connect(ctx, "amqp://guest:guest@"+listener.Addr().String()+"/")
		if b != nil {
			b.Close()
		}
		done <- e
	}()
	select {
	case c := <-accepted:
		defer c.Close()
	case <-time.After(3 * time.Second):
		t.Fatal("no connection")
	}
	cancel()
	select {
	case err = <-done:
		if err == nil {
			t.Fatal("expected cancelled handshake")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handshake did not cancel")
	}
}
