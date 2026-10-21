package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
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
func result(w http.ResponseWriter, status int, value any, err error) {
	if err == nil {
		writeJSON(w, status, value)
		return
	}
	if errors.Is(err, ErrInvalid) {
		writeError(w, http.StatusBadRequest, "invalid_input")
		return
	}
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if errors.Is(err, ErrConflict) {
		writeError(w, http.StatusConflict, "conflict")
		return
	}
	writeError(w, http.StatusServiceUnavailable, "unavailable")
}

func (s *Store) Handler(cors string) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.DB.Ping(ctx); err != nil {
			writeError(w, 503, "unavailable")
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok", "database": "up"})
	})
	m.HandleFunc("POST /api/doctors", func(w http.ResponseWriter, r *http.Request) {
		var in DoctorInput
		if err := decode(w, r, &in); err != nil {
			result(w, 0, nil, err)
			return
		}
		d, err := s.CreateDoctor(r.Context(), in)
		if err == nil {
			w.Header().Set("Location", "/api/doctors/"+d.ID)
		}
		result(w, 201, d, err)
	})
	m.HandleFunc("GET /api/doctors", func(w http.ResponseWriter, r *http.Request) {
		ds, err := s.Doctors(r.Context())
		result(w, 200, ds, err)
	})
	m.HandleFunc("GET /api/doctors/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !validID(r.PathValue("id")) {
			result(w, 0, nil, ErrInvalid)
			return
		}
		d, err := s.Doctor(r.Context(), r.PathValue("id"))
		result(w, 200, d, err)
	})
	m.HandleFunc("POST /api/doctors/{id}/slots", func(w http.ResponseWriter, r *http.Request) {
		if !validID(r.PathValue("id")) {
			result(w, 0, nil, ErrInvalid)
			return
		}
		var in SlotInput
		if err := decode(w, r, &in); err != nil || in.Validate() != nil {
			result(w, 0, nil, ErrInvalid)
			return
		}
		slot, err := s.CreateSlot(r.Context(), r.PathValue("id"), in)
		result(w, 201, slot, err)
	})
	m.HandleFunc("GET /api/doctors/{id}/slots", func(w http.ResponseWriter, r *http.Request) {
		slots, err := s.Slots(r.Context(), r.PathValue("id"), r.URL.Query().Get("status"))
		result(w, 200, slots, err)
	})
	m.HandleFunc("GET /api/requests", func(w http.ResponseWriter, r *http.Request) {
		requests, err := s.Requests(r.Context(), r.URL.Query().Get("status"))
		result(w, 200, requests, err)
	})
	m.HandleFunc("POST /api/requests/{id}/accept", func(w http.ResponseWriter, r *http.Request) {
		request, err := s.DecideRequest(r.Context(), r.PathValue("id"), "accept", "")
		result(w, 200, request, err)
	})
	m.HandleFunc("POST /api/requests/{id}/reject", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Reason string `json:"reason"`
		}
		if err := decode(w, r, &input); err != nil {
			result(w, 0, nil, err)
			return
		}
		request, err := s.DecideRequest(r.Context(), r.PathValue("id"), "reject", input.Reason)
		result(w, 200, request, err)
	})
	return withCORS(cors, m)
}

func withCORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func pagination(r *http.Request) (int, error) {
	n := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > 200 {
			return 0, ErrInvalid
		}
		n = v
	}
	return n, nil
}
