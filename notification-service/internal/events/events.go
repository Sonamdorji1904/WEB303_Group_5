// Package events defines the shared event contract (see docs/event-contract.md).
// Every event on the hospital.events exchange uses the same envelope.
package events

import (
	"encoding/json"
	"fmt"
	"time"
)

// Event types published by the other services.
const (
	PatientRegistered    = "PatientRegistered"
	AppointmentRequested = "AppointmentRequested"
	SlotReserved         = "SlotReserved"
	SlotRejected         = "SlotRejected"
	AppointmentConfirmed = "AppointmentConfirmed"
	AppointmentRejected  = "AppointmentRejected"
	AppointmentCompleted = "AppointmentCompleted"
	MedicalRecordCreated = "MedicalRecordCreated"
)

// BindingKeys are the routing keys the Notification Service binds to on the
// topic exchange.
var BindingKeys = []string{
	PatientRegistered,
	AppointmentRequested,
	AppointmentConfirmed,
	AppointmentRejected,
	AppointmentCompleted,
	MedicalRecordCreated,
}

// Envelope is the common wrapper around every event.
type Envelope struct {
	EventID   string          `json:"eventId"`
	EventType string          `json:"eventType"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// Data is the union of the fields the Notification Service reads from the
// different event payloads. Unused fields simply stay empty.
type Data struct {
	AppointmentID FlexString `json:"appointmentId"`
	RecordID      FlexString `json:"recordId"`

	PatientID    FlexString `json:"patientId"`
	PatientName  string     `json:"patientName"`
	PatientEmail string     `json:"patientEmail"`

	DoctorID    FlexString `json:"doctorId"`
	DoctorName  string     `json:"doctorName"`
	DoctorEmail string     `json:"doctorEmail"`

	Date string `json:"date"`
	Time string `json:"time"`

	Diagnosis    string `json:"diagnosis"`
	Prescription string `json:"prescription"`
	Reason       string `json:"reason"`
}

// FlexString accepts either a JSON string ("p-1") or a JSON number (42), so
// the service keeps working whichever ID type another team member chooses.
type FlexString string

func (f *FlexString) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = FlexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("id must be a string or number: %w", err)
	}
	*f = FlexString(n.String())
	return nil
}

func (f FlexString) String() string { return string(f) }
