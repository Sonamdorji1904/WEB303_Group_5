package config

import "os"

type Config struct {
	Port, DatabaseURL, RabbitURL, CORSOrigin string
}

func Load() Config {
	return Config{
		Port:        get("PORT", "8083"),
		DatabaseURL: get("DATABASE_URL", "postgres://appointment:appointment@localhost:5436/appointment_db?sslmode=disable"),
		RabbitURL:   get("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		CORSOrigin:  get("CORS_ORIGIN", "*"),
	}
}
func get(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
