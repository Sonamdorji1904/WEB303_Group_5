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
	ErrConflict = errors.New("conflict")
)

type DoctorInput struct {
	Name           string `json:"name"`
	Specialization string `json:"specialization"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
}

func (d *DoctorInput) Validate() error {
	d.Name, d.Specialization, d.Email, d.Phone = strings.TrimSpace(d.Name), strings.TrimSpace(d.Specialization), strings.TrimSpace(d.Email), strings.TrimSpace(d.Phone)
	if len(d.Name) == 0 || len(d.Name) > 200 || len(d.Specialization) == 0 || len(d.Specialization) > 200 || len(d.Email) > 254 || len(d.Phone) == 0 || len(d.Phone) > 50 {
		return ErrInvalid
	}
	return nil
}

type Doctor struct {
	ID string `json:"id"`
	DoctorInput
	CreatedAt time.Time `json:"createdAt"`
}

type SlotInput struct {
	Date      string `json:"date"`
	StartTime string `json:"startTime"`
}

func (s *SlotInput) Validate() error {
	if _, err := time.Parse("2006-01-02", s.Date); err != nil {
		return ErrInvalid
	}
	if _, err := time.Parse("15:04", s.StartTime); err != nil || len(s.StartTime) != 5 {
		return ErrInvalid
	}
	return nil
}

type Slot struct {
	ID            string `json:"id"`
	DoctorID      string `json:"doctorId"`
	Date          string `json:"date"`
	StartTime     string `json:"startTime"`
	Status        string `json:"status"`
	AppointmentID string `json:"appointmentId,omitempty"`
}

type AppointmentRequest struct {
	ID              string    `json:"id"`
	AppointmentID   string    `json:"appointmentId"`
	DoctorID        string    `json:"doctorId"`
	PatientID       string    `json:"patientId"`
	PatientName     string    `json:"patientName"`
	PatientEmail    string    `json:"patientEmail"`
	DoctorName      string    `json:"doctorName"`
	Date            string    `json:"date"`
	Time            string    `json:"time"`
	Reason          string    `json:"reason"`
	Status          string    `json:"status"`
	RejectionReason string    `json:"rejectionReason,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type Event struct {
	EventID   string          `json:"eventId"`
	EventType string          `json:"eventType"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

type AppointmentRequested struct {
	AppointmentID string `json:"appointmentId"`
	PatientID     string `json:"patientId"`
	PatientName   string `json:"patientName"`
	PatientEmail  string `json:"patientEmail"`
	DoctorID      string `json:"doctorId"`
	DoctorName    string `json:"doctorName"`
	Date          string `json:"date"`
	Time          string `json:"time"`
	Reason        string `json:"reason"`
}

func validID(s string) bool {
	u, err := uuid.Parse(s)
	return err == nil && u != uuid.Nil && s == u.String()
}

func parseRequested(body []byte) (Event, AppointmentRequested, error) {
	var e Event
	var a AppointmentRequested
	if json.Unmarshal(body, &e) != nil || e.EventType != "AppointmentRequested" || !validID(e.EventID) || e.Timestamp.IsZero() || json.Unmarshal(e.Data, &a) != nil {
		return e, a, ErrInvalid
	}
	if !validID(a.AppointmentID) || !validID(a.PatientID) || !validID(a.DoctorID) || strings.TrimSpace(a.PatientName) == "" || strings.TrimSpace(a.Date) == "" || strings.TrimSpace(a.Time) == "" {
		return e, a, ErrInvalid
	}
	if _, err := time.Parse("2006-01-02", a.Date); err != nil {
		return e, a, ErrInvalid
	}
	if _, err := time.Parse("15:04", a.Time); err != nil || len(a.Time) != 5 {
		return e, a, ErrInvalid
	}
	return e, a, nil
}

func newEvent(kind string, data any) Event {
	body, _ := json.Marshal(data)
	return Event{EventID: uuid.NewString(), EventType: kind, Timestamp: time.Now().UTC(), Data: body}
}
