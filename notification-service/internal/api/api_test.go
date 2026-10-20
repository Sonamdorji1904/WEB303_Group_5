package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"hospital-system/notification-service/internal/store"
)

func seeded(t *testing.T) (*Server, *store.Memory) {
	t.Helper()
	m := store.NewMemory()
	_, err := m.SaveEvent(context.Background(), "e1", "AppointmentConfirmed", []store.Notification{
		{RecipientType: "PATIENT", RecipientID: "p-1", Subject: "Appointment confirmed", Message: "m1", Status: "SENT"},
		{RecipientType: "DOCTOR", RecipientID: "d-1", Subject: "New appointment scheduled", Message: "m2", Status: "SENT"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return New(m, func() bool { return true }, "*"), m
}

func do(s *Server, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestListFiltersByRecipient(t *testing.T) {
	s, _ := seeded(t)
	rec := do(s, "GET", "/api/notifications?recipientType=PATIENT&recipientId=p-1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var got []store.Notification
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].RecipientID != "p-1" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestMarkReadAndNotFound(t *testing.T) {
	s, _ := seeded(t)
	rec := do(s, "PATCH", "/api/notifications/n-1/read")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var n store.Notification
	_ = json.Unmarshal(rec.Body.Bytes(), &n)
	if n.ReadAt == nil {
		t.Error("readAt should be set")
	}
	if rec := do(s, "GET", "/api/notifications/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

func TestHealthReflectsBroker(t *testing.T) {
	m := store.NewMemory()
	down := New(m, func() bool { return false }, "*")
	if rec := do(down, "GET", "/health"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503 when broker down, got %d", rec.Code)
	}
	up := New(m, func() bool { return true }, "*")
	if rec := do(up, "GET", "/health"); rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

func TestCORSPreflight(t *testing.T) {
	s, _ := seeded(t)
	rec := do(s, "OPTIONS", "/api/notifications")
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("preflight failed: %d %v", rec.Code, rec.Header())
	}
}
