package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"hospital/appointment-service/internal/service"
)

func main() {
	file := flag.String("file", "examples/appointment-requested.json", "event JSON file")
	count := flag.Int("count", 1, "number of times to publish the same event")
	flag.Parse()
	body, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var event service.Event
	if err = json.Unmarshal(body, &event); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *count < 1 || event.EventType == "" || event.EventID == "" {
		fmt.Fprintln(os.Stderr, "event must contain eventId and eventType")
		os.Exit(1)
	}
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for i := 0; i < *count; i++ {
		if err := publish(ctx, url, event); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	fmt.Printf("published %s %d time(s)\n", event.EventID, *count)
}

func publish(ctx context.Context, url string, event service.Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	// The service broker intentionally exposes only outbox publishing. This
	// utility uses a short-lived confirmed channel for test-only raw events.
	ch, err := amqp.Dial(url)
	if err != nil {
		return err
	}
	defer ch.Close()
	channel, err := ch.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()
	if err := channel.Confirm(false); err != nil {
		return err
	}
	confirm, err := channel.PublishWithDeferredConfirmWithContext(ctx, service.Exchange, event.EventType, true, false, amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Persistent,
		MessageId: event.EventID, Type: event.EventType, Timestamp: event.Timestamp, Body: body,
	})
	if err != nil {
		return err
	}
	ok, err := confirm.WaitContext(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("broker nack")
	}
	return nil
}
