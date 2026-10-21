package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ DB *pgxpool.Pool }

func enqueue(ctx context.Context, tx pgx.Tx, e Event) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO outbox(event_id,event_type,payload) VALUES($1,$2,$3)`, e.EventID, e.EventType, body)
	return err
}

func (s *Store) CreateDoctor(ctx context.Context, in DoctorInput) (Doctor, error) {
	if err := in.Validate(); err != nil {
		return Doctor{}, err
	}
	var d Doctor
	err := s.DB.QueryRow(ctx, `INSERT INTO doctors(id,name,specialization,email,phone) VALUES($1,$2,$3,NULLIF($4,''),$5) RETURNING id::text,name,specialization,coalesce(email,''),phone,created_at`, uuid.NewString(), in.Name, in.Specialization, in.Email, in.Phone).Scan(&d.ID, &d.Name, &d.Specialization, &d.Email, &d.Phone, &d.CreatedAt)
	return d, err
}

func (s *Store) Doctors(ctx context.Context) ([]Doctor, error) {
	rows, err := s.DB.Query(ctx, `SELECT id::text,name,specialization,coalesce(email,''),phone,created_at FROM doctors ORDER BY created_at DESC,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Doctor{}
	for rows.Next() {
		var d Doctor
		if err := rows.Scan(&d.ID, &d.Name, &d.Specialization, &d.Email, &d.Phone, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) Doctor(ctx context.Context, id string) (Doctor, error) {
	var d Doctor
	err := s.DB.QueryRow(ctx, `SELECT id::text,name,specialization,coalesce(email,''),phone,created_at FROM doctors WHERE id=$1`, id).Scan(&d.ID, &d.Name, &d.Specialization, &d.Email, &d.Phone, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Doctor{}, ErrNotFound
	}
	return d, err
}

func (s *Store) CreateSlot(ctx context.Context, doctorID string, in SlotInput) (Slot, error) {
	if !validID(doctorID) || in.Validate() != nil {
		return Slot{}, ErrInvalid
	}
	if _, err := s.Doctor(ctx, doctorID); err != nil {
		return Slot{}, err
	}
	var slot Slot
	err := s.DB.QueryRow(ctx, `INSERT INTO slots(id,doctor_id,date,start_time) VALUES($1,$2,$3,$4) RETURNING id::text,doctor_id::text,to_char(date,'YYYY-MM-DD'),to_char(start_time,'HH24:MI'),status,coalesce(appointment_id::text,'')`, uuid.NewString(), doctorID, in.Date, in.StartTime).Scan(&slot.ID, &slot.DoctorID, &slot.Date, &slot.StartTime, &slot.Status, &slot.AppointmentID)
	return slot, err
}

func (s *Store) Slots(ctx context.Context, doctorID, status string) ([]Slot, error) {
	if !validID(doctorID) {
		return nil, ErrInvalid
	}
	query := `SELECT id::text,doctor_id::text,to_char(date,'YYYY-MM-DD'),to_char(start_time,'HH24:MI'),status,coalesce(appointment_id::text,'') FROM slots WHERE doctor_id=$1`
	args := []any{doctorID}
	if status != "" {
		if status != "AVAILABLE" && status != "HELD" && status != "RESERVED" {
			return nil, ErrInvalid
		}
		query += ` AND status=$2`
		args = append(args, status)
	}
	query += ` ORDER BY date,start_time,id`
	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Slot{}
	for rows.Next() {
		var slot Slot
		if err := rows.Scan(&slot.ID, &slot.DoctorID, &slot.Date, &slot.StartTime, &slot.Status, &slot.AppointmentID); err != nil {
			return nil, err
		}
		out = append(out, slot)
	}
	return out, rows.Err()
}

func (s *Store) Process(ctx context.Context, body []byte) error {
	e, a, err := parseRequested(body)
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
	var doctorName string
	err = tx.QueryRow(ctx, `SELECT name FROM doctors WHERE id=$1`, a.DoctorID).Scan(&doctorName)
	if errors.Is(err, pgx.ErrNoRows) {
		doctorName = a.DoctorName
	} else if err != nil {
		return err
	}
	var slotID string
	var status string
	err = tx.QueryRow(ctx, `SELECT id::text,status FROM slots WHERE doctor_id=$1 AND date=$2 AND start_time=$3 FOR UPDATE`, a.DoctorID, a.Date, a.Time).Scan(&slotID, &status)
	eventType := "SlotRejected"
	data := map[string]any{"appointmentId": a.AppointmentID, "doctorId": a.DoctorID, "doctorName": doctorName, "patientId": a.PatientID, "patientName": a.PatientName, "patientEmail": a.PatientEmail, "date": a.Date, "time": a.Time}
	if err == nil && status == "AVAILABLE" {
		if _, err = tx.Exec(ctx, `UPDATE slots SET status='HELD',appointment_id=$2 WHERE id=$1`, slotID, a.AppointmentID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO appointment_requests(id,appointment_id,doctor_id,patient_id,patient_name,patient_email,doctor_name,date,start_time,reason) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9,$10)`, uuid.NewString(), a.AppointmentID, a.DoctorID, a.PatientID, a.PatientName, a.PatientEmail, doctorName, a.Date, a.Time, a.Reason)
		if err != nil {
			return err
		}
		eventType = ""
	} else if err == nil && status == "HELD" {
		data["reason"] = "requested slot is awaiting doctor approval"
		eventType = ""
	} else {
		data["reason"] = "requested slot is unavailable"
		if errors.Is(err, pgx.ErrNoRows) {
			data["reason"] = "requested slot does not exist"
		} else if err != nil {
			return err
		}
	}
	if eventType != "" {
		if err = enqueue(ctx, tx, newEvent(eventType, data)); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO processed_events(event_id,event_type) VALUES($1,$2)`, e.EventID, e.EventType); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const requestColumns = `id::text,appointment_id::text,doctor_id::text,patient_id::text,patient_name,coalesce(patient_email,''),doctor_name,to_char(date,'YYYY-MM-DD'),to_char(start_time,'HH24:MI'),reason,status,coalesce(rejection_reason,''),created_at,updated_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRequest(row rowScanner) (AppointmentRequest, error) {
	var r AppointmentRequest
	err := row.Scan(&r.ID, &r.AppointmentID, &r.DoctorID, &r.PatientID, &r.PatientName, &r.PatientEmail, &r.DoctorName, &r.Date, &r.Time, &r.Reason, &r.Status, &r.RejectionReason, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

func (s *Store) Requests(ctx context.Context, status string) ([]AppointmentRequest, error) {
	if status == "" {
		status = "PENDING"
	}
	if status != "PENDING" && status != "ACCEPTED" && status != "REJECTED" {
		return nil, ErrInvalid
	}
	rows, err := s.DB.Query(ctx, `SELECT `+requestColumns+` FROM appointment_requests WHERE status=$1 ORDER BY created_at ASC,id`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AppointmentRequest{}
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DecideRequest(ctx context.Context, id, decision, reason string) (AppointmentRequest, error) {
	if !validID(id) || (decision != "accept" && decision != "reject") {
		return AppointmentRequest{}, ErrInvalid
	}
	if decision == "reject" {
		reason = strings.TrimSpace(reason)
		if reason == "" || len(reason) > 1000 {
			return AppointmentRequest{}, ErrInvalid
		}
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return AppointmentRequest{}, err
	}
	defer tx.Rollback(ctx)
	r, err := scanRequest(tx.QueryRow(ctx, `SELECT `+requestColumns+` FROM appointment_requests WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return r, err
	}
	if r.Status != "PENDING" {
		return r, ErrConflict
	}
	var slotStatus string
	if err = tx.QueryRow(ctx, `SELECT status FROM slots WHERE appointment_id=$1 FOR UPDATE`, r.AppointmentID).Scan(&slotStatus); err != nil {
		return r, err
	}
	if slotStatus != "HELD" {
		return r, ErrConflict
	}
	eventType := "SlotReserved"
	if decision == "accept" {
		_, err = tx.Exec(ctx, `UPDATE slots SET status='RESERVED' WHERE appointment_id=$1`, r.AppointmentID)
	} else {
		eventType = "SlotRejected"
		_, err = tx.Exec(ctx, `UPDATE slots SET status='AVAILABLE',appointment_id=NULL WHERE appointment_id=$1`, r.AppointmentID)
	}
	if err != nil {
		return r, err
	}
	status := "ACCEPTED"
	if decision == "reject" {
		status = "REJECTED"
	}
	r, err = scanRequest(tx.QueryRow(ctx, `UPDATE appointment_requests SET status=$2,rejection_reason=NULLIF($3,''),updated_at=now() WHERE id=$1 RETURNING `+requestColumns, id, status, reason))
	if err != nil {
		return r, err
	}
	data := map[string]any{"appointmentId": r.AppointmentID, "doctorId": r.DoctorID, "doctorName": r.DoctorName, "patientId": r.PatientID, "patientName": r.PatientName, "patientEmail": r.PatientEmail, "date": r.Date, "time": r.Time}
	if decision == "reject" {
		data["reason"] = reason
	}
	if err = enqueue(ctx, tx, newEvent(eventType, data)); err != nil {
		return r, err
	}
	return r, tx.Commit(ctx)
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
		_, updateErr := tx.Exec(ctx, `UPDATE outbox SET attempts=$2,next_attempt_at=$3 WHERE event_id=$1`, id, attempts, time.Now().Add(time.Second*time.Duration(1<<min(attempts, 8))))
		if updateErr != nil {
			return true, updateErr
		}
		return true, tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, `UPDATE outbox SET published_at=now(),attempts=attempts+1 WHERE event_id=$1`, id)
	if err != nil {
		return true, err
	}
	return true, tx.Commit(ctx)
}
