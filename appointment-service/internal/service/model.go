package service

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalid  = errors.New("invalid input")
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("invalid state transition")
)

type AppointmentInput struct {
	PatientID    string `json:"patientId"`
	PatientName  string `json:"patientName"`
	PatientEmail string `json:"patientEmail"`
	DoctorID     string `json:"doctorId"`
	DoctorName   string `json:"doctorName"`
	Date         string `json:"date"`
	Time         string `json:"time"`
	Reason       string `json:"reason"`
}

func (a *AppointmentInput) Validate() error {
	a.PatientName, a.PatientEmail, a.DoctorName, a.Date, a.Time, a.Reason = strings.TrimSpace(a.PatientName), strings.TrimSpace(a.PatientEmail), strings.TrimSpace(a.DoctorName), strings.TrimSpace(a.Date), strings.TrimSpace(a.Time), strings.TrimSpace(a.Reason)
	if !validID(a.PatientID) || !validID(a.DoctorID) || a.PatientName == "" || len(a.PatientName) > 200 || a.DoctorName == "" || len(a.DoctorName) > 200 || len(a.PatientEmail) > 254 || a.Date == "" || a.Time == "" || len(a.Reason) > 1000 {
		return ErrInvalid
	}
	if _, err := time.Parse("2006-01-02", a.Date); err != nil {
		return ErrInvalid
	}
	if _, err := time.Parse("15:04", a.Time); err != nil || len(a.Time) != 5 {
		return ErrInvalid
	}
	return nil
}

type CompletionInput struct {
	Diagnosis    string `json:"diagnosis"`
	Prescription string `json:"prescription"`
	Notes        string `json:"notes"`
}

func (c *CompletionInput) Validate() error {
	c.Diagnosis, c.Prescription, c.Notes = strings.TrimSpace(c.Diagnosis), strings.TrimSpace(c.Prescription), strings.TrimSpace(c.Notes)
	if c.Diagnosis == "" || len(c.Diagnosis) > 10000 || c.Prescription == "" || len(c.Prescription) > 10000 || len(c.Notes) > 10000 {
		return ErrInvalid
	}
	return nil
}

type Appointment struct {
	ID string `json:"id"`
	AppointmentInput
	Status          string    `json:"status"`
	RejectionReason string    `json:"rejectionReason,omitempty"`
	Diagnosis       string    `json:"diagnosis,omitempty"`
	Prescription    string    `json:"prescription,omitempty"`
	Notes           string    `json:"notes,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type Event struct {
	EventID   string          `json:"eventId"`
	EventType string          `json:"eventType"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

type SlotEvent struct {
	AppointmentID string `json:"appointmentId"`
	DoctorID      string `json:"doctorId"`
	DoctorName    string `json:"doctorName"`
	PatientID     string `json:"patientId"`
	PatientName   string `json:"patientName"`
	PatientEmail  string `json:"patientEmail"`
	Date          string `json:"date"`
	Time          string `json:"time"`
	Reason        string `json:"reason"`
}

func validID(s string) bool {
	u, err := uuid.Parse(s)
	return err == nil && u != uuid.Nil && s == u.String()
}

func parseSlotEvent(body []byte, typ string) (Event, SlotEvent, error) {
	var e Event
	var d SlotEvent
	if json.Unmarshal(body, &e) != nil || e.EventType != typ || !validID(e.EventID) || e.Timestamp.IsZero() || json.Unmarshal(e.Data, &d) != nil {
		return e, d, ErrInvalid
	}
	if !validID(d.AppointmentID) || !validID(d.PatientID) || !validID(d.DoctorID) || d.PatientName == "" || d.DoctorName == "" {
		return e, d, ErrInvalid
	}
	return e, d, nil
}

func newEvent(kind string, data any) Event {
	body, _ := json.Marshal(data)
	return Event{EventID: uuid.NewString(), EventType: kind, Timestamp: time.Now().UTC(), Data: body}
}
