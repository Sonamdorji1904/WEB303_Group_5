package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"hospital/patient-service/internal/service"
	"os"
	"time"
)

func main() {
	path := flag.String("file", "examples/appointment-completed.json", "event JSON file")
	flag.Parse()
	if err := run(*path); err != nil {
		fmt.Fprintln(os.Stderr, "Publish failed; check file, event validation and broker connectivity")
		os.Exit(1)
	}
	fmt.Println("Event confirmed by RabbitMQ")
}
func run(path string) error {
	body, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	var event service.Event
	if e = json.Unmarshal(body, &event); e != nil {
		return e
	}
	if event.EventType == "AppointmentCompleted" {
		event, _, e = service.ParseAppointment(body)
		if e != nil {
			return e
		}
	} else if event.EventType != "PatientRegistered" || event.EventID == "" || event.Timestamp.IsZero() || len(event.Data) == 0 {
		return fmt.Errorf("unsupported or invalid sample event")
	}
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	b, e := service.Connect(ctx, url)
	if e != nil {
		return e
	}
	defer b.Close()
	return b.Publish(ctx, event)
}
