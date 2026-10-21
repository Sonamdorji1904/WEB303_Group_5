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

func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, code, message string) {
	respond(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func result(w http.ResponseWriter, status int, v any, err error) {
	switch {
	case err == nil:
		respond(w, status, v)
	case errors.Is(err, ErrNotFound):
		problem(w, 404, "not_found", "Resource not found")
	case errors.Is(err, ErrInvalid):
		problem(w, 400, "invalid_input", "Check required fields, field lengths, email and dates")
	default:
		problem(w, 503, "unavailable", "Service temporarily unavailable")
	}
}
func decode(w http.ResponseWriter, r *http.Request) (PatientInput, error) {
	var in PatientInput
	r.Body = http.MaxBytesReader(w, r.Body, 65536)
	defer r.Body.Close()
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil {
		return in, ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return in, ErrInvalid
	}
	return in, in.Validate()
}
func pagination(r *http.Request) (int, int, error) {
	limit, offset := 20, 0
	var e error
	if s := r.URL.Query().Get("limit"); s != "" {
		limit, e = strconv.Atoi(s)
		if e != nil {
			return 0, 0, ErrInvalid
		}
	}
	if s := r.URL.Query().Get("offset"); s != "" {
		offset, e = strconv.Atoi(s)
		if e != nil {
			return 0, 0, ErrInvalid
		}
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return 0, 0, ErrInvalid
	}
	return limit, offset, nil
}
func (s *Store) Handler(cors ...string) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if s.DB.Ping(ctx) != nil {
			problem(w, 503, "unavailable", "Database unavailable")
			return
		}
		respond(w, 200, map[string]string{"status": "ok", "database": "up"})
	})
	m.HandleFunc("POST /api/patients", func(w http.ResponseWriter, r *http.Request) {
		in, e := decode(w, r)
		if e != nil {
			result(w, 0, nil, e)
			return
		}
		p, e := s.Register(r.Context(), in)
		if e == nil {
			w.Header().Set("Location", "/api/patients/"+p.ID)
		}
		result(w, 201, p, e)
	})
	m.HandleFunc("GET /api/patients", func(w http.ResponseWriter, r *http.Request) {
		l, o, e := pagination(r)
		if e != nil {
			result(w, 0, nil, e)
			return
		}
		ps, e := s.Patients(r.Context(), l, o)
		result(w, 200, map[string]any{"data": ps, "limit": l, "offset": o}, e)
	})
	m.HandleFunc("GET /api/patients/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validID(id) {
			result(w, 0, nil, ErrInvalid)
			return
		}
		p, e := s.Patient(r.Context(), id)
		result(w, 200, p, e)
	})
	m.HandleFunc("PUT /api/patients/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validID(id) {
			result(w, 0, nil, ErrInvalid)
			return
		}
		in, e := decode(w, r)
		if e != nil {
			result(w, 0, nil, e)
			return
		}
		p, e := s.Update(r.Context(), id, in)
		result(w, 200, p, e)
	})
	m.HandleFunc("GET /api/patients/{id}/medical-records", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validID(id) {
			result(w, 0, nil, ErrInvalid)
			return
		}
		l, o, e := pagination(r)
		if e != nil {
			result(w, 0, nil, e)
			return
		}
		rs, e := s.Records(r.Context(), id, l, o)
		result(w, 200, map[string]any{"data": rs, "limit": l, "offset": o}, e)
	})
	m.HandleFunc("GET /api/medical-records/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validID(id) {
			result(w, 0, nil, ErrInvalid)
			return
		}
		v, e := s.Record(r.Context(), id)
		result(w, 200, v, e)
	})
	// Match before ServeMux to keep 404 and 405 responses in the JSON error format.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pattern := m.Handler(r)
		if pattern == "" {
			if r.Method != "GET" {
				clone := r.Clone(r.Context())
				clone.Method = "GET"
				_, p := m.Handler(clone)
				if p != "" {
					w.Header().Set("Allow", "GET, HEAD")
					if r.URL.Path == "/api/patients" {
						w.Header().Set("Allow", "GET, HEAD, POST")
					}
					if validPatientPath(r.URL.Path) {
						w.Header().Set("Allow", "GET, HEAD, PUT")
					}
					problem(w, 405, "method_not_allowed", "Method not allowed")
					return
				}
			}
			problem(w, 404, "not_found", "Resource not found")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		m.ServeHTTP(w, r.WithContext(ctx))
	})
	origin := "*"
	if len(cors) > 0 && cors[0] != "" {
		origin = cors[0]
	}
	return withCORS(origin, handler)
}

func withCORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func validPatientPath(p string) bool {
	const pre = "/api/patients/"
	return len(p) > len(pre) && p[:len(pre)] == pre && validID(p[len(pre):])
}
