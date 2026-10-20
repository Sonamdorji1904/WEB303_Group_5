package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

//go:embed migrations/001_init.sql
var initSQL string

type Postgres struct {
	db *sql.DB
}

// OpenPostgres connects (retrying while the database starts up) and applies
// the schema.
func OpenPostgres(ctx context.Context, url string) (*Postgres, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	var lastErr error
	for attempt := 1; attempt <= 30; attempt++ {
		pctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		lastErr = db.PingContext(pctx)
		cancel()
		if lastErr == nil {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("database not reachable: %w", lastErr)
	}
	if _, err := db.ExecContext(ctx, initSQL); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Postgres{db: db}, nil
}

func (p *Postgres) Close() error { return p.db.Close() }

func (p *Postgres) Ping(ctx context.Context) error { return p.db.PingContext(ctx) }

func (p *Postgres) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	var exists bool
	err := p.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM processed_events WHERE event_id = $1)`, eventID).Scan(&exists)
	return exists, err
}

func (p *Postgres) SaveEvent(ctx context.Context, eventID, eventType string, notifs []Notification) (bool, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.ExecContext(ctx,
		`INSERT INTO processed_events (event_id, event_type) VALUES ($1, $2)
		 ON CONFLICT (event_id) DO NOTHING`, eventID, eventType)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return false, nil // duplicate delivery
	}

	for _, n := range notifs {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO notifications
			   (event_id, event_type, recipient_type, recipient_id, recipient_name,
			    recipient_email, channel, subject, message, status, sent_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			 ON CONFLICT (event_id, recipient_type, recipient_id) DO NOTHING`,
			eventID, eventType, n.RecipientType, n.RecipientID, n.RecipientName,
			n.RecipientEmail, n.Channel, n.Subject, n.Message, n.Status, n.SentAt)
		if err != nil {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

const cols = `id, event_id, event_type, recipient_type, recipient_id, recipient_name,
	recipient_email, channel, subject, message, status, created_at, sent_at, read_at`

func scan(r interface{ Scan(...any) error }) (Notification, error) {
	var n Notification
	var sent, read sql.NullTime
	err := r.Scan(&n.ID, &n.EventID, &n.EventType, &n.RecipientType, &n.RecipientID,
		&n.RecipientName, &n.RecipientEmail, &n.Channel, &n.Subject, &n.Message,
		&n.Status, &n.CreatedAt, &sent, &read)
	if sent.Valid {
		n.SentAt = &sent.Time
	}
	if read.Valid {
		n.ReadAt = &read.Time
	}
	return n, err
}

func (p *Postgres) List(ctx context.Context, f Filter) ([]Notification, error) {
	var where []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.RecipientType != "" {
		add("recipient_type = $%d", strings.ToUpper(f.RecipientType))
	}
	if f.RecipientID != "" {
		add("recipient_id = $%d", f.RecipientID)
	}
	if f.EventType != "" {
		add("event_type = $%d", f.EventType)
	}
	if f.UnreadOnly {
		where = append(where, "read_at IS NULL")
	}
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := "SELECT " + cols + " FROM notifications"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT %d", limit)

	rows, err := p.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Notification{}
	for rows.Next() {
		n, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (p *Postgres) Get(ctx context.Context, id string) (*Notification, error) {
	n, err := scan(p.db.QueryRowContext(ctx, "SELECT "+cols+" FROM notifications WHERE id::text = $1", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (p *Postgres) MarkRead(ctx context.Context, id string) (*Notification, error) {
	n, err := scan(p.db.QueryRowContext(ctx,
		`UPDATE notifications SET read_at = COALESCE(read_at, now())
		 WHERE id::text = $1 RETURNING `+cols, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (p *Postgres) ListProcessedEvents(ctx context.Context, limit int) ([]ProcessedEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := p.db.QueryContext(ctx,
		`SELECT event_id, event_type, processed_at FROM processed_events
		 ORDER BY processed_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProcessedEvent{}
	for rows.Next() {
		var e ProcessedEvent
		if err := rows.Scan(&e.EventID, &e.EventType, &e.ProcessedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
