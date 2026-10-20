// Package api exposes the Notification Service's REST interface, used by the
// frontend to display notifications that were created by event processing.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"hospital-system/notification-service/internal/store"
)

type Server struct {
	store      store.Store
	ready      func() bool // RabbitMQ consumer connected?
	corsOrigin string
}

func New(s store.Store, brokerReady func() bool, corsOrigin string) *Server {
	return &Server{store: s, ready: brokerReady, corsOrigin: corsOrigin}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /api/notifications", s.list)
	mux.HandleFunc("GET /api/notifications/{id}", s.get)
	mux.HandleFunc("PATCH /api/notifications/{id}/read", s.markRead)
	mux.HandleFunc("GET /api/processed-events", s.processedEvents)
	return s.cors(s.logRequests(mux))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	dbOK := s.store.Ping(ctx) == nil
	brokerOK := s.ready()
	status, code := "UP", http.StatusOK
	if !dbOK || !brokerOK {
		status, code = "DEGRADED", http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]any{
		"service": "notification-service", "status": status,
		"database": up(dbOK), "rabbitmq": up(brokerOK),
	})
}

func up(ok bool) string {
	if ok {
		return "UP"
	}
	return "DOWN"
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	items, err := s.store.List(r.Context(), store.Filter{
		RecipientType: q.Get("recipientType"),
		RecipientID:   q.Get("recipientId"),
		EventType:     q.Get("eventType"),
		UnreadOnly:    q.Get("unread") == "true",
		Limit:         limit,
	})
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	n, err := s.store.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "notification not found")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) markRead(w http.ResponseWriter, r *http.Request) {
	n, err := s.store.MarkRead(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "notification not found")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

// processedEvents lists the event IDs already handled; handy as evidence for
// the duplicate-event test.
func (s *Server) processedEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.store.ListProcessedEvents(r.Context(), limit)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.corsOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if r.URL.Path != "/health" {
			slog.Info("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start).String())
		}
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func serverError(w http.ResponseWriter, err error) {
	slog.Error("request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}
