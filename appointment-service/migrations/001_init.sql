CREATE TABLE IF NOT EXISTS appointments (
 id uuid PRIMARY KEY, patient_id uuid NOT NULL, patient_name text NOT NULL, patient_email text,
 doctor_id uuid NOT NULL, doctor_name text NOT NULL, date date NOT NULL, start_time time NOT NULL,
 reason text NOT NULL DEFAULT '', status text NOT NULL DEFAULT 'PENDING' CHECK(status IN ('PENDING','CONFIRMED','REJECTED','COMPLETED')),
 rejection_reason text, diagnosis text, prescription text, notes text,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS appointments_status ON appointments(status,created_at DESC);
CREATE INDEX IF NOT EXISTS appointments_patient ON appointments(patient_id,created_at DESC);
CREATE TABLE IF NOT EXISTS processed_events(event_id uuid PRIMARY KEY,event_type text NOT NULL,processed_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS outbox(event_id uuid PRIMARY KEY,event_type text NOT NULL,payload jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),published_at timestamptz,attempts integer NOT NULL DEFAULT 0,next_attempt_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX IF NOT EXISTS outbox_pending ON outbox(next_attempt_at,created_at) WHERE published_at IS NULL;
