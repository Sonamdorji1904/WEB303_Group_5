package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"hospital-system/notification-service/internal/messaging"
	"hospital-system/notification-service/internal/store"
)

// countingSender records every "send" so tests can prove duplicates do not
// produce a second message.
type countingSender struct {
	mu   sync.Mutex
	sent []store.Notification
	err  error
}

func (c *countingSender) Send(_ context.Context, n store.Notification) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	c.sent = append(c.sent, n)
	return nil
}

func (c *countingSender) count() int { c.mu.Lock(); defer c.mu.Unlock(); return len(c.sent) }

func event(id, typ, data string) []byte {
	return []byte(fmt.Sprintf(`{"eventId":%q,"eventType":%q,"timestamp":"2026-10-20T10:00:00Z","data":%s}`, id, typ, data))
}

const apptData = `{"appointmentId":"a-1","patientId":"p-1","patientName":"Pema Dorji","patientEmail":"pema@example.com",
"doctorId":"d-1","doctorName":"Karma Wangchuk","doctorEmail":"karma@example.com","date":"2026-10-25","time":"10:30"}`

func setup() (*Handler, *store.Memory, *countingSender) {
	st := store.NewMemory()
	snd := &countingSender{}
	return New(st, snd), st, snd
}

func list(t *testing.T, st *store.Memory, f store.Filter) []store.Notification {
	t.Helper()
	out, err := st.List(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestPatientRegistered_SendsWelcome(t *testing.T) {
	h, st, _ := setup()
	err := h.Handle(context.Background(), event("e1", "PatientRegistered",
		`{"patientId":"p-1","patientName":"Pema Dorji","patientEmail":"pema@example.com"}`))
	if err != nil {
		t.Fatal(err)
	}
	got := list(t, st, store.Filter{})
	if len(got) != 1 || got[0].RecipientType != store.RecipientPatient || got[0].Status != store.StatusSent {
		t.Fatalf("unexpected notifications: %+v", got)
	}
	if !strings.Contains(got[0].Message, "Pema Dorji") {
		t.Errorf("message should address the patient: %q", got[0].Message)
	}
}

func TestAppointmentRequested_NotifiesPatient(t *testing.T) {
	h, st, _ := setup()
	if err := h.Handle(context.Background(), event("e2", "AppointmentRequested", apptData)); err != nil {
		t.Fatal(err)
	}
	got := list(t, st, store.Filter{})
	if len(got) != 1 || got[0].Subject != "Appointment request received" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestAppointmentConfirmed_NotifiesPatientAndDoctor(t *testing.T) {
	h, st, snd := setup()
	if err := h.Handle(context.Background(), event("e3", "AppointmentConfirmed", apptData)); err != nil {
		t.Fatal(err)
	}
	if got := list(t, st, store.Filter{RecipientType: "PATIENT"}); len(got) != 1 {
		t.Errorf("want 1 patient notification, got %d", len(got))
	}
	doc := list(t, st, store.Filter{RecipientType: "DOCTOR", RecipientID: "d-1"})
	if len(doc) != 1 || !strings.Contains(doc[0].Message, "Pema Dorji") {
		t.Errorf("doctor notification wrong: %+v", doc)
	}
	if snd.count() != 2 {
		t.Errorf("want 2 sends, got %d", snd.count())
	}
}

func TestAppointmentRejected_ExplainsReason(t *testing.T) {
	h, st, _ := setup()
	if err := h.Handle(context.Background(), event("e4", "AppointmentRejected", apptData)); err != nil {
		t.Fatal(err)
	}
	got := list(t, st, store.Filter{})
	if len(got) != 1 || !strings.Contains(got[0].Message, "not available") {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestAppointmentCompleted_NotifiesPatient(t *testing.T) {
	h, st, _ := setup()
	data := `{"appointmentId":"a-1","patientId":"p-1","patientName":"Pema Dorji","doctorName":"Karma Wangchuk","date":"2026-10-25","diagnosis":"Flu","prescription":"Rest"}`
	if err := h.Handle(context.Background(), event("e5", "AppointmentCompleted", data)); err != nil {
		t.Fatal(err)
	}
	got := list(t, st, store.Filter{})
	if len(got) != 1 || got[0].Subject != "Your visit is complete" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestMedicalRecordCreated_DoesNotLeakDiagnosis(t *testing.T) {
	h, st, _ := setup()
	data := `{"recordId":"r-1","patientId":"p-1","patientName":"Pema Dorji","diagnosis":"Influenza"}`
	if err := h.Handle(context.Background(), event("e6", "MedicalRecordCreated", data)); err != nil {
		t.Fatal(err)
	}
	got := list(t, st, store.Filter{})
	if len(got) != 1 || strings.Contains(got[0].Message, "Influenza") {
		t.Fatalf("diagnosis must not appear in notification text: %+v", got)
	}
}

// Part G: "Duplicate event" - repeated delivery must not create a second
// notification or send a second message.
func TestDuplicateEvent_ProcessedOnce(t *testing.T) {
	h, st, snd := setup()
	body := event("dup-1", "AppointmentConfirmed", apptData)
	for i := 0; i < 3; i++ {
		if err := h.Handle(context.Background(), body); err != nil {
			t.Fatalf("delivery %d: %v", i+1, err)
		}
	}
	if got := list(t, st, store.Filter{}); len(got) != 2 {
		t.Errorf("want 2 notifications (patient+doctor) once, got %d", len(got))
	}
	if snd.count() != 2 {
		t.Errorf("duplicates must not re-send: sent %d", snd.count())
	}
	pe, _ := st.ListProcessedEvents(context.Background(), 10)
	if len(pe) != 1 {
		t.Errorf("want 1 processed event, got %d", len(pe))
	}
}

func TestNumericIDsAreAccepted(t *testing.T) {
	h, st, _ := setup()
	data := `{"appointmentId":7,"patientId":12,"patientName":"Pema","doctorId":3,"doctorName":"Karma","date":"2026-10-25","time":"09:00"}`
	if err := h.Handle(context.Background(), event("e7", "AppointmentConfirmed", data)); err != nil {
		t.Fatal(err)
	}
	if got := list(t, st, store.Filter{RecipientID: "12"}); len(got) != 1 {
		t.Errorf("numeric patientId should be stored as \"12\": %+v", got)
	}
}

func TestPermanentErrors(t *testing.T) {
	cases := map[string][]byte{
		"invalid json":      []byte(`{not json`),
		"missing eventId":   []byte(`{"eventType":"PatientRegistered","data":{}}`),
		"unknown eventType": event("x1", "SomethingElse", `{}`),
		"missing fields":    event("x2", "PatientRegistered", `{"patientId":"p-1"}`),
		"bad data":          event("x3", "PatientRegistered", `"oops"`),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			h, st, snd := setup()
			err := h.Handle(context.Background(), body)
			if !errors.Is(err, messaging.ErrPermanent) {
				t.Fatalf("want ErrPermanent, got %v", err)
			}
			if snd.count() != 0 || len(list(t, st, store.Filter{})) != 0 {
				t.Error("nothing should be sent or stored for a rejected event")
			}
		})
	}
}

// Part E: a database outage is transient (retry), and must leave the event
// unprocessed so the retry can succeed.
func TestDatabaseFailure_IsTransientAndRecoverable(t *testing.T) {
	h, st, _ := setup()
	st.FailSave = errors.New("connection refused")
	body := event("t1", "PatientRegistered", `{"patientId":"p-1","patientName":"Pema"}`)

	err := h.Handle(context.Background(), body)
	if err == nil || errors.Is(err, messaging.ErrPermanent) {
		t.Fatalf("want transient error, got %v", err)
	}

	st.FailSave = nil // database is back; the retried message succeeds
	if err := h.Handle(context.Background(), body); err != nil {
		t.Fatal(err)
	}
	if got := list(t, st, store.Filter{}); len(got) != 1 {
		t.Errorf("want 1 notification after recovery, got %d", len(got))
	}
}

func TestSenderFailure_IsTransientAndStoresNothing(t *testing.T) {
	h, st, snd := setup()
	snd.err = errors.New("smtp down")
	err := h.Handle(context.Background(), event("s1", "PatientRegistered", `{"patientId":"p-1","patientName":"Pema"}`))
	if err == nil || errors.Is(err, messaging.ErrPermanent) {
		t.Fatalf("want transient error, got %v", err)
	}
	if processed, _ := st.IsProcessed(context.Background(), "s1"); processed {
		t.Error("failed event must not be marked processed")
	}
}

func TestDoctorLabel(t *testing.T) {
	for in, want := range map[string]string{
		"Karma Wangchuk": "Dr. Karma Wangchuk",
		"Dr. Karma":      "Dr. Karma",
		"dr tshering":    "dr tshering",
		"":               "your doctor",
	} {
		if got := doctorLabel(in); got != want {
			t.Errorf("doctorLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
