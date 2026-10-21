package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Memory is an in-memory Store used by unit tests (no database needed).
type Memory struct {
	mu        sync.Mutex
	notifs    []Notification
	processed map[string]ProcessedEvent
	seq       int

	// FailSave makes SaveEvent return an error (simulates a database outage).
	FailSave error
}

func NewMemory() *Memory { return &Memory{processed: map[string]ProcessedEvent{}} }

func (m *Memory) Ping(context.Context) error { return nil }

func (m *Memory) IsProcessed(_ context.Context, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.processed[id]
	return ok, nil
}

func (m *Memory) SaveEvent(_ context.Context, eventID, eventType string, notifs []Notification) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FailSave != nil {
		return false, m.FailSave
	}
	if _, ok := m.processed[eventID]; ok {
		return false, nil
	}
	m.processed[eventID] = ProcessedEvent{EventID: eventID, EventType: eventType, ProcessedAt: time.Now()}
	for _, n := range notifs {
		m.seq++
		n.ID = fmt.Sprintf("n-%d", m.seq)
		n.CreatedAt = time.Now()
		m.notifs = append(m.notifs, n)
	}
	return true, nil
}

func (m *Memory) List(_ context.Context, f Filter) ([]Notification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Notification{}
	for i := len(m.notifs) - 1; i >= 0; i-- {
		n := m.notifs[i]
		if f.RecipientType != "" && !strings.EqualFold(n.RecipientType, f.RecipientType) {
			continue
		}
		if f.RecipientID != "" && n.RecipientID != f.RecipientID {
			continue
		}
		if f.EventType != "" && n.EventType != f.EventType {
			continue
		}
		if f.UnreadOnly && n.ReadAt != nil {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}

func (m *Memory) Get(_ context.Context, id string) (*Notification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.notifs {
		if m.notifs[i].ID == id {
			n := m.notifs[i]
			return &n, nil
		}
	}
	return nil, ErrNotFound
}

func (m *Memory) MarkRead(_ context.Context, id string) (*Notification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.notifs {
		if m.notifs[i].ID == id {
			if m.notifs[i].ReadAt == nil {
				t := time.Now()
				m.notifs[i].ReadAt = &t
			}
			n := m.notifs[i]
			return &n, nil
		}
	}
	return nil, ErrNotFound
}

func (m *Memory) ListProcessedEvents(context.Context, int) ([]ProcessedEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ProcessedEvent, 0, len(m.processed))
	for _, e := range m.processed {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ProcessedAt.After(out[j].ProcessedAt) })
	return out, nil
}
