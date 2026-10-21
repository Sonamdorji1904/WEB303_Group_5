package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"hospital/patient-service/internal/service"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func env(k, f string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return f
}
func main() {
	if err := run(); err != nil {
		slog.Error("service stopped", "reason", err.Error())
		os.Exit(1)
	}
}
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := pgxpool.New(ctx, env("DATABASE_URL", "postgres://patient:patient_local@localhost:55433/patient_db?sslmode=disable"))
	if err != nil {
		return err
	}
	defer db.Close()
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = db.Ping(startup)
	cancel()
	if err != nil {
		return err
	}
	s := &service.Store{DB: db}
	url := env("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	var wg sync.WaitGroup
	for _, worker := range []func(context.Context, *service.Store, string){service.RunConsumer, service.RunOutbox} {
		wg.Add(1)
		go func(f func(context.Context, *service.Store, string)) { defer wg.Done(); f(ctx, s, url) }(worker)
	}
	server := &http.Server{Addr: env("HTTP_ADDR", ":8081"), Handler: s.Handler(env("CORS_ORIGIN", "*")), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	slog.Info("patient service listening", "address", server.Addr)
	select {
	case <-ctx.Done():
	case err = <-done:
		if err == http.ErrServerClosed {
			err = nil
		}
		stop()
	}
	shutdown, c := context.WithTimeout(context.Background(), 15*time.Second)
	defer c()
	_ = server.Shutdown(shutdown)
	wg.Wait()
	return err
}
