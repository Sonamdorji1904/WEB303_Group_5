package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ DB *pgxpool.Pool }

const columns = `id::text,patient_id::text,patient_name,coalesce(patient_email,''),doctor_id::text,doctor_name,to_char(date,'YYYY-MM-DD'),to_char(start_time,'HH24:MI'),reason,status,coalesce(rejection_reason,''),coalesce(diagnosis,''),coalesce(prescription,''),coalesce(notes,''),created_at,updated_at`

func enqueue(ctx context.Context, tx pgx.Tx, e Event) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO outbox(event_id,event_type,payload) VALUES($1,$2,$3)`, e.EventID, e.EventType, body)
	return err
}
func scan(row pgx.Row) (Appointment, error) {
	var a Appointment
	err := row.Scan(&a.ID, &a.PatientID, &a.PatientName, &a.PatientEmail, &a.DoctorID, &a.DoctorName, &a.Date, &a.Time, &a.Reason, &a.Status, &a.RejectionReason, &a.Diagnosis, &a.Prescription, &a.Notes, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

func (s *Store) Create(ctx context.Context, in AppointmentInput) (Appointment, error) {
	if err := in.Validate(); err != nil {
		return Appointment{}, err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Appointment{}, err
	}
	defer tx.Rollback(ctx)
	a, err := scan(tx.QueryRow(ctx, `INSERT INTO appointments(id,patient_id,patient_name,patient_email,doctor_id,doctor_name,date,start_time,reason) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+columns, uuid.NewString(), in.PatientID, in.PatientName, in.PatientEmail, in.DoctorID, in.DoctorName, in.Date, in.Time, in.Reason))
	if err != nil {
		return a, err
	}
	if err = enqueue(ctx, tx, newEvent("AppointmentRequested", map[string]any{
		"appointmentId": a.ID,
		"patientId":     in.PatientID,
		"patientName":   in.PatientName,
		"patientEmail":  in.PatientEmail,
		"doctorId":      in.DoctorID,
		"doctorName":    in.DoctorName,
		"date":          in.Date,
		"time":          in.Time,
		"reason":        in.Reason,
	})); err != nil {
		return a, err
	}
	return a, tx.Commit(ctx)
}

func (s *Store) List(ctx context.Context, status, patientID string) ([]Appointment, error) {
	query := `SELECT ` + columns + ` FROM appointments WHERE 1=1`
	args := []any{}
	n := 0
	if status != "" {
		if status != "PENDING" && status != "CONFIRMED" && status != "REJECTED" && status != "COMPLETED" {
			return nil, ErrInvalid
		}
		n++
		query += ` AND status=$` + itoa(n)
		args = append(args, status)
	}
	if patientID != "" {
		if !validID(patientID) {
			return nil, ErrInvalid
		}
		n++
		query += ` AND patient_id=$` + itoa(n)
		args = append(args, patientID)
	}
	query += ` ORDER BY created_at DESC,id`
	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Appointment{}
	for rows.Next() {
		a, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func itoa(n int) string { return string(rune('0' + n)) }
func (s *Store) Get(ctx context.Context, id string) (Appointment, error) {
	if !validID(id) {
		return Appointment{}, ErrInvalid
	}
	return scan(s.DB.QueryRow(ctx, `SELECT `+columns+` FROM appointments WHERE id=$1`, id))
}

func (s *Store) Complete(ctx context.Context, id string, in CompletionInput) (Appointment, error) {
	if !validID(id) || in.Validate() != nil {
		return Appointment{}, ErrInvalid
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Appointment{}, err
	}
	defer tx.Rollback(ctx)
	a, err := scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM appointments WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return a, err
	}
	if a.Status != "CONFIRMED" {
		return Appointment{}, ErrConflict
	}
	visit, _ := time.Parse("2006-01-02 15:04", a.Date+" "+a.Time)
	visit = visit.UTC()
	e := newEvent("AppointmentCompleted", map[string]any{"appointmentId": a.ID, "patientId": a.PatientID, "patientName": a.PatientName, "patientEmail": a.PatientEmail, "doctorId": a.DoctorID, "doctorName": a.DoctorName, "date": a.Date, "time": a.Time, "visitDate": visit, "diagnosis": in.Diagnosis, "prescription": in.Prescription, "notes": in.Notes})
	a, err = scan(tx.QueryRow(ctx, `UPDATE appointments SET status='COMPLETED',diagnosis=$2,prescription=$3,notes=$4,updated_at=now() WHERE id=$1 RETURNING `+columns, id, in.Diagnosis, in.Prescription, in.Notes))
	if err != nil {
		return a, err
	}
	if err = enqueue(ctx, tx, e); err != nil {
		return a, err
	}
	return a, tx.Commit(ctx)
}

func (s *Store) processSlot(ctx context.Context, body []byte, typ string) error {
	e, d, err := parseSlotEvent(body, typ)
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, e.EventID); err != nil {
		return err
	}
	var done bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM processed_events WHERE event_id=$1)`, e.EventID).Scan(&done); err != nil {
		return err
	}
	if done {
		return tx.Commit(ctx)
	}
	a, err := scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM appointments WHERE id=$1 FOR UPDATE`, d.AppointmentID))
	if errors.Is(err, ErrNotFound) {
		_, err = tx.Exec(ctx, `INSERT INTO processed_events(event_id,event_type) VALUES($1,$2)`, e.EventID, e.EventType)
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if typ == "SlotReserved" {
		if a.Status == "PENDING" {
			a, err = scan(tx.QueryRow(ctx, `UPDATE appointments SET status='CONFIRMED',updated_at=now() WHERE id=$1 RETURNING `+columns, a.ID))
			if err != nil {
				return err
			}
			if err = enqueue(ctx, tx, newEvent("AppointmentConfirmed", map[string]any{"appointmentId": a.ID, "patientId": a.PatientID, "patientName": a.PatientName, "patientEmail": a.PatientEmail, "doctorId": a.DoctorID, "doctorName": a.DoctorName, "date": a.Date, "time": a.Time})); err != nil {
				return err
			}
		}
	} else if a.Status == "PENDING" {
		a, err = scan(tx.QueryRow(ctx, `UPDATE appointments SET status='REJECTED',rejection_reason=$2,updated_at=now() WHERE id=$1 RETURNING `+columns, a.ID, d.Reason))
		if err != nil {
			return err
		}
		if err = enqueue(ctx, tx, newEvent("AppointmentRejected", map[string]any{"appointmentId": a.ID, "patientId": a.PatientID, "patientName": a.PatientName, "patientEmail": a.PatientEmail, "doctorName": a.DoctorName, "date": a.Date, "time": a.Time, "reason": a.RejectionReason})); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO processed_events(event_id,event_type) VALUES($1,$2)`, e.EventID, e.EventType)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) ProcessReserved(ctx context.Context, body []byte) error {
	return s.processSlot(ctx, body, "SlotReserved")
}
func (s *Store) ProcessRejected(ctx context.Context, body []byte) error {
	return s.processSlot(ctx, body, "SlotRejected")
}

func (s *Store) Dispatch(ctx context.Context, publish func(context.Context, Event) error) (bool, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var id string
	var body []byte
	var attempts int
	err = tx.QueryRow(ctx, `SELECT event_id::text,payload,attempts FROM outbox WHERE published_at IS NULL AND next_attempt_at<=now() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &body, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var e Event
	if err = json.Unmarshal(body, &e); err != nil {
		return false, err
	}
	if err = publish(ctx, e); err != nil {
		attempts++
		_, uerr := tx.Exec(ctx, `UPDATE outbox SET attempts=$2,next_attempt_at=$3 WHERE event_id=$1`, id, attempts, time.Now().Add(time.Second*time.Duration(1<<min(attempts, 8))))
		if uerr != nil {
			return true, uerr
		}
		return true, tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, `UPDATE outbox SET published_at=now(),attempts=attempts+1 WHERE event_id=$1`, id); err != nil {
		return true, err
	}
	return true, tx.Commit(ctx)
}
