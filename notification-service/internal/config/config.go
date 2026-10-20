// Package config loads the service configuration from environment variables.
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port        string
	DatabaseURL string
	RabbitURL   string
	CORSOrigin  string

	Exchange   string // topic exchange shared by every service
	Queue      string // this service's durable queue
	MaxRetries int    // redeliveries before a message goes to the DLQ
	RetryDelay time.Duration
	Prefetch   int
}

func Load() Config {
	return Config{
		Port:        get("PORT", "8084"),
		DatabaseURL: get("DATABASE_URL", "postgres://notification:notification@localhost:5435/notification_db?sslmode=disable"),
		RabbitURL:   get("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		CORSOrigin:  get("CORS_ORIGIN", "*"),
		Exchange:    get("RABBITMQ_EXCHANGE", "hospital.events"),
		Queue:       get("RABBITMQ_QUEUE", "notification-service.queue"),
		MaxRetries:  getInt("MAX_RETRIES", 3),
		RetryDelay:  time.Duration(getInt("RETRY_DELAY_SECONDS", 5)) * time.Second,
		Prefetch:    getInt("PREFETCH", 5),
	}
}

func get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
