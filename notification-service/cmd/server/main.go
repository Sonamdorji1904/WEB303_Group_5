package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hospital-system/notification-service/internal/api"
	"hospital-system/notification-service/internal/config"
	"hospital-system/notification-service/internal/events"
	"hospital-system/notification-service/internal/messaging"
	"hospital-system/notification-service/internal/sender"
	"hospital-system/notification-service/internal/service"
	"hospital-system/notification-service/internal/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := store.OpenPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("cannot start database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	handler := service.New(db, sender.LogSender{})

	consumer := messaging.NewConsumer(messaging.Config{
		URL:         cfg.RabbitURL,
		Exchange:    cfg.Exchange,
		Queue:       cfg.Queue,
		BindingKeys: events.BindingKeys,
		MaxRetries:  cfg.MaxRetries,
		RetryDelay:  cfg.RetryDelay,
		Prefetch:    cfg.Prefetch,
	}, handler.Handle)

	// The consumer reconnects on its own, so the REST API stays up even while
	// RabbitMQ is unavailable (/health then reports DEGRADED).
	go consumer.Run(ctx)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.New(db, consumer.Ready, cfg.CORSOrigin).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		slog.Info("notification-service listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
