package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"hospital/doctor-service/internal/config"
	"hospital/doctor-service/internal/service"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("cannot start database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = db.Ping(pctx)
	cancel()
	if err != nil {
		slog.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	store := &service.Store{DB: db}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); service.RunConsumer(ctx, store, cfg.RabbitURL) }()
	go func() { defer wg.Done(); service.RunOutbox(ctx, store, cfg.RabbitURL) }()
	server := &http.Server{Addr: ":" + cfg.Port, Handler: store.Handler(cfg.CORSOrigin), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, MaxHeaderBytes: 16384}
	go func() {
		if e := server.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
			slog.Error("http server failed", "error", e)
			stop()
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
	wg.Wait()
}
