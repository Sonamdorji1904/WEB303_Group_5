CREATE TABLE IF NOT EXISTS schema_migrations(version integer PRIMARY KEY);
CREATE TABLE IF NOT EXISTS patients (
 id uuid PRIMARY KEY, name text NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 200),
 date_of_birth date NOT NULL CHECK (date_of_birth >= DATE '0001-01-01' AND date_of_birth <= CURRENT_DATE), gender text NOT NULL CHECK (length(trim(gender)) BETWEEN 1 AND 50),
 phone text NOT NULL CHECK (length(trim(phone)) BETWEEN 1 AND 50), email text,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS medical_records (
 id uuid PRIMARY KEY, patient_id uuid NOT NULL REFERENCES patients(id), appointment_id uuid NOT NULL UNIQUE,
 doctor_id uuid NOT NULL, doctor_name text NOT NULL CHECK (length(trim(doctor_name)) BETWEEN 1 AND 10000), visit_date timestamptz NOT NULL,
 diagnosis text NOT NULL CHECK (length(trim(diagnosis)) BETWEEN 1 AND 10000), prescription text NOT NULL CHECK (length(trim(prescription)) BETWEEN 1 AND 10000), created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS medical_records_patient_visit ON medical_records(patient_id, visit_date DESC, id);
CREATE INDEX IF NOT EXISTS patients_created ON patients(created_at DESC, id);
CREATE TABLE IF NOT EXISTS processed_events (
 event_id uuid PRIMARY KEY, event_type text NOT NULL, processed_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS outbox (
 event_id uuid PRIMARY KEY, event_type text NOT NULL, payload jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), published_at timestamptz,
 attempts integer NOT NULL DEFAULT 0, next_attempt_at timestamptz NOT NULL DEFAULT now(),
 failed_at timestamptz
);
CREATE INDEX IF NOT EXISTS outbox_pending ON outbox(next_attempt_at, created_at) WHERE published_at IS NULL AND failed_at IS NULL;
INSERT INTO schema_migrations(version) VALUES (1) ON CONFLICT DO NOTHING;
