CREATE TABLE IF NOT EXISTS doctors (
 id uuid PRIMARY KEY, name text NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 200),
 specialization text NOT NULL CHECK (length(trim(specialization)) BETWEEN 1 AND 200),
 email text, phone text NOT NULL CHECK (length(trim(phone)) BETWEEN 1 AND 50),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS slots (
 id uuid PRIMARY KEY, doctor_id uuid NOT NULL REFERENCES doctors(id),
 date date NOT NULL, start_time time NOT NULL, status text NOT NULL DEFAULT 'AVAILABLE' CHECK (status IN ('AVAILABLE','HELD','RESERVED')),
 appointment_id uuid UNIQUE, UNIQUE(doctor_id,date,start_time)
);
ALTER TABLE slots DROP CONSTRAINT IF EXISTS slots_status_check;
ALTER TABLE slots ADD CONSTRAINT slots_status_check CHECK (status IN ('AVAILABLE','HELD','RESERVED'));
CREATE INDEX IF NOT EXISTS slots_lookup ON slots(doctor_id,date,start_time);
CREATE TABLE IF NOT EXISTS appointment_requests (
 id uuid PRIMARY KEY,
 appointment_id uuid NOT NULL UNIQUE,
 doctor_id uuid NOT NULL REFERENCES doctors(id),
 patient_id uuid NOT NULL,
 patient_name text NOT NULL,
 patient_email text,
 doctor_name text NOT NULL,
 date date NOT NULL,
 start_time time NOT NULL,
 reason text NOT NULL DEFAULT '',
 status text NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','ACCEPTED','REJECTED')),
 rejection_reason text,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS appointment_requests_doctor_status ON appointment_requests(doctor_id,status,created_at DESC);
CREATE TABLE IF NOT EXISTS processed_events (
 event_id uuid PRIMARY KEY, event_type text NOT NULL, processed_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS outbox (
 event_id uuid PRIMARY KEY, event_type text NOT NULL, payload jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), published_at timestamptz,
 attempts integer NOT NULL DEFAULT 0, next_attempt_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS outbox_pending ON outbox(next_attempt_at,created_at) WHERE published_at IS NULL;
