package service

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"net/mail"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid input")
var ErrNotFound = errors.New("not found")
var ErrMissingPatient = errors.New("patient not yet available")

type PatientInput struct {
	Name        string `json:"name"`
	DateOfBirth string `json:"dateOfBirth"`
	Gender      string `json:"gender"`
	Phone       string `json:"phone"`
	Email       string `json:"email,omitempty"`
}

func (p *PatientInput) Validate() error {
	p.Name = strings.TrimSpace(p.Name)
	p.Gender = strings.TrimSpace(p.Gender)
	p.Phone = strings.TrimSpace(p.Phone)
	p.Email = strings.TrimSpace(p.Email)
	dob, err := time.Parse("2006-01-02", p.DateOfBirth)
	if err != nil || dob.After(time.Now().UTC()) || dob.Year() < 1 || len(p.Name) == 0 || len(p.Name) > 200 || len(p.Gender) == 0 || len(p.Gender) > 50 || len(p.Phone) == 0 || len(p.Phone) > 50 || len(p.Email) > 254 {
		return ErrInvalid
	}
	if p.Email != "" {
		a, e := mail.ParseAddress(p.Email)
		if e != nil || a.Address != p.Email {
			return ErrInvalid
		}
	}
	return nil
}

type Patient struct {
	ID string `json:"id"`
	PatientInput
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type Record struct {
	ID            string    `json:"id"`
	PatientID     string    `json:"patientId"`
	AppointmentID string    `json:"appointmentId"`
	DoctorID      string    `json:"doctorId"`
	DoctorName    string    `json:"doctorName"`
	VisitDate     time.Time `json:"visitDate"`
	Diagnosis     string    `json:"diagnosis"`
	Prescription  string    `json:"prescription"`
	CreatedAt     time.Time `json:"createdAt"`
}
type Event struct {
	EventID   string          `json:"eventId"`
	EventType string          `json:"eventType"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}
type Appointment struct {
	AppointmentID string    `json:"appointmentId"`
	PatientID     string    `json:"patientId"`
	PatientName   string    `json:"patientName"`
	DoctorID      string    `json:"doctorId"`
	DoctorName    string    `json:"doctorName"`
	VisitDate     time.Time `json:"visitDate"`
	Diagnosis     string    `json:"diagnosis"`
	Prescription  string    `json:"prescription"`
}

func validID(s string) bool {
	u, e := uuid.Parse(s)
	return e == nil && u != uuid.Nil && s == u.String()
}
func ParseAppointment(body []byte) (Event, Appointment, error) {
	var e Event
	var a Appointment
	if json.Unmarshal(body, &e) != nil || !validID(e.EventID) || e.EventType != "AppointmentCompleted" || e.Timestamp.IsZero() || e.Timestamp.Year() < 1 || e.Timestamp.After(time.Now().Add(5*time.Minute)) || json.Unmarshal(e.Data, &a) != nil {
		return e, a, ErrInvalid
	}
	if !validID(a.AppointmentID) || !validID(a.PatientID) || !validID(a.DoctorID) || a.VisitDate.IsZero() || a.VisitDate.Year() < 1 || a.VisitDate.After(time.Now().Add(5*time.Minute)) {
		return e, a, ErrInvalid
	}
	for _, s := range []string{a.PatientName, a.DoctorName, a.Diagnosis, a.Prescription} {
		if strings.TrimSpace(s) == "" || len(s) > 10000 {
			return e, a, ErrInvalid
		}
	}
	return e, a, nil
}
func newEvent(kind string, data any) Event {
	b, _ := json.Marshal(data)
	return Event{uuid.NewString(), kind, time.Now().UTC(), b}
}
