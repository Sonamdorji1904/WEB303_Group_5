package main

import (
	"context"
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
	fmt.Println("AppointmentCompleted confirmed by RabbitMQ")
}
func run(path string) error {
	body, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	event, _, e := service.ParseAppointment(body)
	if e != nil {
		return e
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
