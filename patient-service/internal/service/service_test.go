package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestValidation(t *testing.T) {
	good := PatientInput{Name: "Yeshey", DateOfBirth: "2000-01-01", Gender: "unspecified", Phone: "+97512345678"}
	for _, tc := range []struct {
		name   string
		mutate func(*PatientInput)
	}{
		{"empty name", func(p *PatientInput) { p.Name = " " }}, {"future birth", func(p *PatientInput) { p.DateOfBirth = "2999-01-01" }}, {"invalid date", func(p *PatientInput) { p.DateOfBirth = "2000-02-30" }}, {"missing phone", func(p *PatientInput) { p.Phone = "" }}, {"missing gender", func(p *PatientInput) { p.Gender = "" }}, {"invalid email", func(p *PatientInput) { p.Email = "a@" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := good
			tc.mutate(&p)
			if !errors.Is(p.Validate(), ErrInvalid) {
				t.Fatal("expected invalid input")
			}
		})
	}
	if e := good.Validate(); e != nil {
		t.Fatal(e)
	}
	if Retryable(ErrInvalid) || !Retryable(ErrMissingPatient) || !Retryable(errors.New("database")) {
		t.Fatal("failure policy")
	}
}
func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL unset: PostgreSQL integration check not run")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	schema := "patient_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	cfg, e := pgxpool.ParseConfig(url)
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close(); _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); admin.Close() })
	sql, e := os.ReadFile("../../migrations/001_init.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, string(sql)); e != nil {
		t.Fatal(e)
	}
	return &Store{db}
}
func patient(t *testing.T, s *Store) Patient {
	t.Helper()
	p, e := s.Register(context.Background(), PatientInput{Name: "Test Patient", DateOfBirth: "2000-01-01", Gender: "other", Phone: "+97500000000", Email: "test@example.com"})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func appointment(p string) []byte {
	a := Appointment{AppointmentID: uuid.NewString(), PatientID: p, PatientName: "Test Patient", DoctorID: uuid.NewString(), DoctorName: "Test Doctor", VisitDate: time.Now().UTC().Add(-time.Minute), Diagnosis: "test diagnosis", Prescription: "test prescription"}
	b, _ := json.Marshal(newEvent("AppointmentCompleted", a))
	return b
}
func count(t *testing.T, s *Store, table string) int {
	t.Helper()
	var n int
	if e := s.DB.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}
func request(s *Store, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}
func TestPatientsHTTP(t *testing.T) {
	s := testStore(t)
	body := `{"name":"Yeshey","dateOfBirth":"2000-01-01","gender":"other","phone":"123","email":"yeshey@example.com"}`
	w := request(s, "POST", "/api/patients", body)
	if w.Code != 201 {
		t.Fatalf("register: %d %s", w.Code, w.Body.String())
	}
	var p Patient
	if e := json.Unmarshal(w.Body.Bytes(), &p); e != nil {
		t.Fatal(e)
	}
	if !validID(p.ID) || count(t, s, "outbox") != 1 {
		t.Fatal("patient and outbox not committed")
	}
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/api/patients/" + p.ID, "", 200}, {"GET", "/api/patients?limit=1&offset=0", "", 200}, {"GET", "/api/patients/" + uuid.NewString(), "", 404},
		{"GET", "/api/patients/bad", "", 400}, {"GET", "/api/patients?limit=101", "", 400}, {"POST", "/api/patients", `{}`, 400}, {"POST", "/api/patients", body + ` {}`, 400}, {"POST", "/api/patients", `{"unknown":1}`, 400},
		{"PUT", "/api/patients/" + p.ID, strings.ReplaceAll(body, "Yeshey", "Updated"), 200}, {"GET", "/api/patients/" + p.ID + "/medical-records", "", 200}, {"GET", "/api/medical-records/" + uuid.NewString(), "", 404}, {"GET", "/health", "", 200}, {"DELETE", "/api/patients/" + p.ID, "", 405}, {"POST", "/api/medical-records", `{}`, 404}, {"GET", "/unknown", "", 404},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			r := request(s, tc.method, tc.path, tc.body)
			if r.Code != tc.status {
				t.Fatalf("got %d: %s", r.Code, r.Body.String())
			}
			if !json.Valid(r.Body.Bytes()) {
				t.Fatal("non-JSON response")
			}
		})
	}
	if count(t, s, "patients") != 1 || count(t, s, "outbox") != 1 {
		t.Fatal("invalid input or updates published additional registration events")
	}
}
func TestAppointmentIdempotencyAndLookup(t *testing.T) {
	s := testStore(t)
	p := patient(t, s)
	ctx := context.Background()
	b := appointment(p.ID)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- s.Process(ctx, b) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var event Event
	_ = json.Unmarshal(b, &event)
	event.EventID = uuid.NewString()
	different, _ := json.Marshal(event)
	if e := s.Process(ctx, different); e != nil {
		t.Fatal(e)
	}
	if count(t, s, "medical_records") != 1 || count(t, s, "processed_events") != 2 || count(t, s, "outbox") != 2 {
		t.Fatal("duplicate created extra record or logical event")
	}
	rs, e := s.Records(ctx, p.ID, 20, 0)
	if e != nil || len(rs) != 1 {
		t.Fatalf("lookup: %v", e)
	}
	r, e := s.Record(ctx, rs[0].ID)
	if e != nil || r.Diagnosis != "test diagnosis" {
		t.Fatal("record lookup")
	}
	for _, path := range []string{"/api/medical-records/" + r.ID, "/api/patients/" + p.ID + "/medical-records"} {
		if w := request(s, "GET", path, ""); w.Code != 200 {
			t.Fatal(w.Code)
		}
	}
	var payload []byte
	if e = s.DB.QueryRow(ctx, "SELECT payload FROM outbox WHERE event_type='MedicalRecordCreated'").Scan(&payload); e != nil {
		t.Fatal(e)
	}
	var out Event
	_ = json.Unmarshal(payload, &out)
	var data map[string]any
	_ = json.Unmarshal(out.Data, &data)
	if data["phone"] != p.Phone || data["patientEmail"] != p.Email || data["recordId"] != r.ID {
		t.Fatal("missing notification contract fields")
	}
	if _, ok := data["diagnosis"]; ok {
		t.Fatal("unnecessary clinical data in notification event")
	}
}
func TestInvalidAndMissingPatient(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.Process(ctx, []byte(`{"eventId":"bad"}`)); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	b := appointment(uuid.NewString())
	if e := s.Process(ctx, b); !errors.Is(e, ErrMissingPatient) {
		t.Fatal(e)
	}
	if count(t, s, "processed_events") != 0 || count(t, s, "medical_records") != 0 || count(t, s, "outbox") != 0 {
		t.Fatal("failed event persisted")
	}
	var ev Event
	_ = json.Unmarshal(b, &ev)
	var a Appointment
	_ = json.Unmarshal(ev.Data, &a)
	for _, change := range []func(*Event, *Appointment){func(e *Event, a *Appointment) { a.Diagnosis = "" }, func(e *Event, a *Appointment) { a.Prescription = "" }, func(e *Event, a *Appointment) { a.PatientName = "" }, func(e *Event, a *Appointment) { a.DoctorID = "bad" }, func(e *Event, a *Appointment) { a.VisitDate = time.Now().Add(24 * time.Hour) }, func(e *Event, a *Appointment) { e.EventType = "Wrong" }, func(e *Event, a *Appointment) { e.Timestamp = time.Time{} }} {
		v, ap := ev, a
		change(&v, &ap)
		v.Data, _ = json.Marshal(ap)
		bad, _ := json.Marshal(v)
		if e := s.Process(ctx, bad); !errors.Is(e, ErrInvalid) {
			t.Fatal("accepted invalid event")
		}
	}
}
func TestDatabaseFailureRollsBack(t *testing.T) {
	s := testStore(t)
	p := patient(t, s)
	ctx := context.Background()
	_, e := s.DB.Exec(ctx, `CREATE FUNCTION fail_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected failure'; END $$; CREATE TRIGGER fail_outbox BEFORE INSERT ON outbox FOR EACH ROW EXECUTE FUNCTION fail_outbox()`)
	if e != nil {
		t.Fatal(e)
	}
	b := appointment(p.ID)
	if e = s.Process(ctx, b); e == nil {
		t.Fatal("expected failure")
	}
	if count(t, s, "medical_records") != 0 || count(t, s, "processed_events") != 0 || count(t, s, "outbox") != 1 {
		t.Fatal("transaction did not roll back")
	}
	if _, e = s.Register(ctx, p.PatientInput); e == nil {
		t.Fatal("registration must fail with outbox")
	}
	if count(t, s, "patients") != 1 {
		t.Fatal("registration not atomic")
	}
	_, e = s.DB.Exec(ctx, "DROP TRIGGER fail_outbox ON outbox")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Process(ctx, b); e != nil {
		t.Fatal(e)
	}
	if count(t, s, "processed_events") != 1 {
		t.Fatal("retry not processed")
	}
}
func TestOutboxRetryPreservesID(t *testing.T) {
	s := testStore(t)
	patient(t, s)
	ctx := context.Background()
	var id string
	for i := 0; i < 10; i++ {
		found, e := s.Dispatch(ctx, func(_ context.Context, e Event) error {
			if id != "" && id != e.EventID {
				t.Fatal("event ID changed")
			}
			id = e.EventID
			return errors.New("broker down")
		})
		if !found || e != nil {
			t.Fatalf("dispatch %v %v", found, e)
		}
		_, e = s.DB.Exec(ctx, "UPDATE outbox SET next_attempt_at=now()")
		if e != nil {
			t.Fatal(e)
		}
	}
	found, e := s.Dispatch(ctx, func(context.Context, Event) error { t.Fatal("exceeded bound"); return nil })
	if found || e != nil {
		t.Fatal("failed event should be parked")
	}
	_, e = s.DB.Exec(ctx, "UPDATE outbox SET failed_at=NULL,attempts=0,next_attempt_at=now()")
	if e != nil {
		t.Fatal(e)
	}
	found, e = s.Dispatch(ctx, func(_ context.Context, e Event) error {
		if id != e.EventID {
			t.Fatal("replay changed event ID")
		}
		return nil
	})
	if !found || e != nil {
		t.Fatal(e)
	}
	var published bool
	if e = s.DB.QueryRow(ctx, "SELECT published_at IS NOT NULL FROM outbox").Scan(&published); e != nil || !published {
		t.Fatal("not published")
	}
}
