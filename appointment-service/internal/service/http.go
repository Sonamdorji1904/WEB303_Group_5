package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code}})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 65536)
	defer r.Body.Close()
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		return ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return ErrInvalid
	}
	return nil
}
func result(w http.ResponseWriter, status int, v any, err error) {
	if err == nil {
		writeJSON(w, status, v)
		return
	}
	switch {
	case errors.Is(err, ErrInvalid):
		writeError(w, 400, "invalid_input")
	case errors.Is(err, ErrNotFound):
		writeError(w, 404, "not_found")
	case errors.Is(err, ErrConflict):
		writeError(w, 409, "conflict")
	default:
		writeError(w, 503, "unavailable")
	}
}
func (s *Store) Handler(cors string) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		ctx, c := context.WithTimeout(r.Context(), 2*time.Second)
		defer c()
		if err := s.DB.Ping(ctx); err != nil {
			writeError(w, 503, "unavailable")
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok", "database": "up"})
	})
	m.HandleFunc("POST /api/appointments", func(w http.ResponseWriter, r *http.Request) {
		var in AppointmentInput
		if err := decode(w, r, &in); err != nil || in.Validate() != nil {
			result(w, 0, nil, ErrInvalid)
			return
		}
		a, err := s.Create(r.Context(), in)
		if err == nil {
			w.Header().Set("Location", "/api/appointments/"+a.ID)
		}
		result(w, 201, a, err)
	})
	m.HandleFunc("GET /api/appointments", func(w http.ResponseWriter, r *http.Request) {
		a, err := s.List(r.Context(), r.URL.Query().Get("status"), r.URL.Query().Get("patientId"))
		result(w, 200, a, err)
	})
	m.HandleFunc("GET /api/appointments/{id}", func(w http.ResponseWriter, r *http.Request) {
		a, err := s.Get(r.Context(), r.PathValue("id"))
		result(w, 200, a, err)
	})
	m.HandleFunc("PUT /api/appointments/{id}/complete", func(w http.ResponseWriter, r *http.Request) {
		var in CompletionInput
		if err := decode(w, r, &in); err != nil || in.Validate() != nil {
			result(w, 0, nil, ErrInvalid)
			return
		}
		a, err := s.Complete(r.Context(), r.PathValue("id"), in)
		result(w, 200, a, err)
	})
	return withCORS(cors, m)
}
func withCORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
