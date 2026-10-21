package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port, DatabaseURL, RabbitURL, CORSOrigin string
}

func Load() Config {
	return Config{Port: get("PORT", "8082"), DatabaseURL: get("DATABASE_URL", "postgres://doctor:doctor@localhost:5434/doctor_db?sslmode=disable"), RabbitURL: get("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"), CORSOrigin: get("CORS_ORIGIN", "*")}
}
func get(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func GetInt(k string, d int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil && v > 0 {
		return v
	}
	return d
}
