package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Store struct{ DB *pgxpool.Pool }

const patientColumns = "id::text,name,to_char(date_of_birth,'YYYY-MM-DD'),gender,phone,coalesce(email,''),created_at,updated_at"
const recordColumns = "id::text,patient_id::text,appointment_id::text,doctor_id::text,doctor_name,visit_date,diagnosis,prescription,created_at"

func scanPatient(row pgx.Row) (Patient, error) {
	var p Patient
	err := row.Scan(&p.ID, &p.Name, &p.DateOfBirth, &p.Gender, &p.Phone, &p.Email, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return p, err
}
func scanRecord(row pgx.Row) (Record, error) {
	var r Record
	err := row.Scan(&r.ID, &r.PatientID, &r.AppointmentID, &r.DoctorID, &r.DoctorName, &r.VisitDate, &r.Diagnosis, &r.Prescription, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return r, err
}
func enqueue(ctx context.Context, tx pgx.Tx, e Event) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO outbox(event_id,event_type,payload) VALUES($1,$2,$3)", e.EventID, e.EventType, b)
	return err
}
func (s *Store) Register(ctx context.Context, in PatientInput) (Patient, error) {
	if err := in.Validate(); err != nil {
		return Patient{}, err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Patient{}, err
	}
	defer tx.Rollback(ctx)
	p, err := scanPatient(tx.QueryRow(ctx, "INSERT INTO patients(id,name,date_of_birth,gender,phone,email) VALUES($1,$2,$3,$4,$5,NULLIF($6,'')) RETURNING "+patientColumns, uuid.NewString(), in.Name, in.DateOfBirth, in.Gender, in.Phone, in.Email))
	if err != nil {
		return p, err
	}
	err = enqueue(ctx, tx, newEvent("PatientRegistered", map[string]any{"patientId": p.ID, "patientName": p.Name, "phone": p.Phone, "patientEmail": p.Email}))
	if err != nil {
		return p, err
	}
	return p, tx.Commit(ctx)
}
func (s *Store) Patient(ctx context.Context, id string) (Patient, error) {
	return scanPatient(s.DB.QueryRow(ctx, "SELECT "+patientColumns+" FROM patients WHERE id=$1", id))
}
func (s *Store) Update(ctx context.Context, id string, in PatientInput) (Patient, error) {
	if e := in.Validate(); e != nil {
		return Patient{}, e
	}
	return scanPatient(s.DB.QueryRow(ctx, "UPDATE patients SET name=$2,date_of_birth=$3,gender=$4,phone=$5,email=NULLIF($6,''),updated_at=now() WHERE id=$1 RETURNING "+patientColumns, id, in.Name, in.DateOfBirth, in.Gender, in.Phone, in.Email))
}
func (s *Store) Patients(ctx context.Context, limit, offset int) ([]Patient, error) {
	rows, err := s.DB.Query(ctx, "SELECT "+patientColumns+" FROM patients ORDER BY created_at DESC,id LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ps := []Patient{}
	for rows.Next() {
		p, e := scanPatient(rows)
		if e != nil {
			return nil, e
		}
		ps = append(ps, p)
	}
	return ps, rows.Err()
}
func (s *Store) Record(ctx context.Context, id string) (Record, error) {
	return scanRecord(s.DB.QueryRow(ctx, "SELECT "+recordColumns+" FROM medical_records WHERE id=$1", id))
}
func (s *Store) Records(ctx context.Context, id string, limit, offset int) ([]Record, error) {
	if _, e := s.Patient(ctx, id); e != nil {
		return nil, e
	}
	rows, err := s.DB.Query(ctx, "SELECT "+recordColumns+" FROM medical_records WHERE patient_id=$1 ORDER BY visit_date DESC,id LIMIT $2 OFFSET $3", id, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rs := []Record{}
	for rows.Next() {
		r, e := scanRecord(rows)
		if e != nil {
			return nil, e
		}
		rs = append(rs, r)
	}
	return rs, rows.Err()
}
func (s *Store) Process(ctx context.Context, body []byte) error {
	e, a, err := ParseAppointment(body)
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Serialize matching IDs across replicas; constraints remain the final safeguard.
	_, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", e.EventID)
	if err != nil {
		return err
	}
	var exists bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM processed_events WHERE event_id=$1)", e.EventID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return tx.Commit(ctx)
	}
	p, err := scanPatient(tx.QueryRow(ctx, "SELECT "+patientColumns+" FROM patients WHERE id=$1 FOR SHARE", a.PatientID))
	if errors.Is(err, ErrNotFound) {
		return ErrMissingPatient
	}
	if err != nil {
		return err
	}
	id := uuid.NewString()
	tag, err := tx.Exec(ctx, "INSERT INTO medical_records(id,patient_id,appointment_id,doctor_id,doctor_name,visit_date,diagnosis,prescription) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(appointment_id) DO NOTHING", id, a.PatientID, a.AppointmentID, a.DoctorID, a.DoctorName, a.VisitDate, a.Diagnosis, a.Prescription)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		err = enqueue(ctx, tx, newEvent("MedicalRecordCreated", map[string]any{"recordId": id, "appointmentId": a.AppointmentID, "patientId": p.ID, "patientName": p.Name, "doctorId": a.DoctorID, "doctorName": a.DoctorName, "visitDate": a.VisitDate.UTC(), "phone": p.Phone, "patientEmail": p.Email}))
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, "INSERT INTO processed_events(event_id,event_type) VALUES($1,$2)", e.EventID, e.EventType)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Dispatch holds a row lock until the broker confirms. A crash after confirmation
// can redeliver the SAME eventId; subscribers must deduplicate it.
func (s *Store) Dispatch(ctx context.Context, publish func(context.Context, Event) error) (bool, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var body []byte
	var attempts int
	var id string
	err = tx.QueryRow(ctx, "SELECT event_id::text,payload,attempts FROM outbox WHERE published_at IS NULL AND failed_at IS NULL AND next_attempt_at<=now() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1").Scan(&id, &body, &attempts)
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
	if publishErr := publish(ctx, e); publishErr != nil {
		attempts++
		delay := time.Second * time.Duration(1<<min(attempts, 8))
		_, err = tx.Exec(ctx, "UPDATE outbox SET attempts=$2,next_attempt_at=$3,failed_at=CASE WHEN $2>=10 THEN now() ELSE NULL END WHERE event_id=$1", id, attempts, time.Now().Add(delay))
		if err != nil {
			return true, err
		}
		return true, tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, "UPDATE outbox SET published_at=now(),attempts=attempts+1 WHERE event_id=$1", id)
	if err != nil {
		return true, err
	}
	return true, tx.Commit(ctx)
}
