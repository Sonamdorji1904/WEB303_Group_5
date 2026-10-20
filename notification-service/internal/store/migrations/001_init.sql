-- Notification Service owns these tables; no other service reads or writes them.

CREATE TABLE IF NOT EXISTS notifications (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id        TEXT        NOT NULL,
    event_type      TEXT        NOT NULL,
    recipient_type  TEXT        NOT NULL,           -- PATIENT | DOCTOR
    recipient_id    TEXT        NOT NULL,
    recipient_name  TEXT        NOT NULL DEFAULT '',
    recipient_email TEXT        NOT NULL DEFAULT '',
    channel         TEXT        NOT NULL DEFAULT 'EMAIL',
    subject         TEXT        NOT NULL,
    message         TEXT        NOT NULL,
    status          TEXT        NOT NULL,           -- SENT | FAILED
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at         TIMESTAMPTZ,
    read_at         TIMESTAMPTZ,
    -- second line of defence against duplicates (first is processed_events)
    CONSTRAINT notifications_event_recipient_uniq UNIQUE (event_id, recipient_type, recipient_id)
);

CREATE INDEX IF NOT EXISTS idx_notifications_recipient
    ON notifications (recipient_type, recipient_id, created_at DESC);

-- Idempotency: one row per event this consumer has fully processed.
CREATE TABLE IF NOT EXISTS processed_events (
    event_id     TEXT PRIMARY KEY,
    event_type   TEXT        NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
