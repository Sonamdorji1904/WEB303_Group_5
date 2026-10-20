package service

import (
	"fmt"
	"strings"

	"hospital-system/notification-service/internal/events"
	"hospital-system/notification-service/internal/store"
)

// draft is a notification before it is stored.
type draft struct {
	recipientType string
	recipientID   string
	recipientName string
	email         string
	subject       string
	message       string
}

func doctorLabel(name string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return "your doctor"
	}
	if strings.HasPrefix(strings.ToLower(n), "dr") {
		return n
	}
	return "Dr. " + n
}

func missing(fields map[string]string) error {
	var m []string
	for k, v := range fields {
		if strings.TrimSpace(v) == "" {
			m = append(m, k)
		}
	}
	if len(m) == 0 {
		return nil
	}
	// map order is random; sort for stable error messages
	sortStrings(m)
	return fmt.Errorf("missing required fields: %s", strings.Join(m, ", "))
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// build turns one event into the notifications it should produce.
// It returns an error for unknown event types or missing fields.
func build(eventType string, d events.Data) ([]draft, error) {
	switch eventType {

	case events.PatientRegistered:
		if err := missing(map[string]string{
			"patientId": d.PatientID.String(), "patientName": d.PatientName,
		}); err != nil {
			return nil, err
		}
		return []draft{{
			recipientType: store.RecipientPatient, recipientID: d.PatientID.String(),
			recipientName: d.PatientName, email: d.PatientEmail,
			subject: "Welcome to the Hospital Management System",
			message: fmt.Sprintf("Hello %s, welcome! Your patient account has been created successfully.", d.PatientName),
		}}, nil

	case events.AppointmentRequested:
		if err := missing(map[string]string{
			"appointmentId": d.AppointmentID.String(), "patientId": d.PatientID.String(),
			"patientName": d.PatientName, "doctorName": d.DoctorName,
			"date": d.Date, "time": d.Time,
		}); err != nil {
			return nil, err
		}
		return []draft{{
			recipientType: store.RecipientPatient, recipientID: d.PatientID.String(),
			recipientName: d.PatientName, email: d.PatientEmail,
			subject: "Appointment request received",
			message: fmt.Sprintf("Hello %s, we received your request for an appointment with %s on %s at %s. You will be notified once it is confirmed.",
				d.PatientName, doctorLabel(d.DoctorName), d.Date, d.Time),
		}}, nil

	case events.AppointmentConfirmed:
		if err := missing(map[string]string{
			"appointmentId": d.AppointmentID.String(), "patientId": d.PatientID.String(),
			"patientName": d.PatientName, "doctorId": d.DoctorID.String(),
			"doctorName": d.DoctorName, "date": d.Date, "time": d.Time,
		}); err != nil {
			return nil, err
		}
		return []draft{
			{
				recipientType: store.RecipientPatient, recipientID: d.PatientID.String(),
				recipientName: d.PatientName, email: d.PatientEmail,
				subject: "Appointment confirmed",
				message: fmt.Sprintf("Hello %s, your appointment with %s on %s at %s is confirmed.",
					d.PatientName, doctorLabel(d.DoctorName), d.Date, d.Time),
			},
			{
				recipientType: store.RecipientDoctor, recipientID: d.DoctorID.String(),
				recipientName: d.DoctorName, email: d.DoctorEmail,
				subject: "New appointment scheduled",
				message: fmt.Sprintf("%s, you have a confirmed appointment with patient %s on %s at %s.",
					doctorLabel(d.DoctorName), d.PatientName, d.Date, d.Time),
			},
		}, nil

	case events.AppointmentRejected:
		if err := missing(map[string]string{
			"appointmentId": d.AppointmentID.String(), "patientId": d.PatientID.String(),
			"patientName": d.PatientName, "date": d.Date, "time": d.Time,
		}); err != nil {
			return nil, err
		}
		reason := d.Reason
		if strings.TrimSpace(reason) == "" {
			reason = "the requested slot is not available"
		}
		return []draft{{
			recipientType: store.RecipientPatient, recipientID: d.PatientID.String(),
			recipientName: d.PatientName, email: d.PatientEmail,
			subject: "Appointment could not be confirmed",
			message: fmt.Sprintf("Hello %s, we could not confirm your appointment with %s on %s at %s because %s. Please choose another slot.",
				d.PatientName, doctorLabel(d.DoctorName), d.Date, d.Time, reason),
		}}, nil

	case events.AppointmentCompleted:
		if err := missing(map[string]string{
			"appointmentId": d.AppointmentID.String(), "patientId": d.PatientID.String(),
			"patientName": d.PatientName, "doctorName": d.DoctorName, "date": d.Date,
		}); err != nil {
			return nil, err
		}
		return []draft{{
			recipientType: store.RecipientPatient, recipientID: d.PatientID.String(),
			recipientName: d.PatientName, email: d.PatientEmail,
			subject: "Your visit is complete",
			message: fmt.Sprintf("Hello %s, your visit with %s on %s is complete. Your medical record will be updated shortly.",
				d.PatientName, doctorLabel(d.DoctorName), d.Date),
		}}, nil

	case events.MedicalRecordCreated:
		if err := missing(map[string]string{
			"patientId": d.PatientID.String(), "patientName": d.PatientName,
		}); err != nil {
			return nil, err
		}
		// Deliberately no diagnosis/prescription text: notifications may be
		// sent over email/SMS, so medical details stay in the Patient service.
		return []draft{{
			recipientType: store.RecipientPatient, recipientID: d.PatientID.String(),
			recipientName: d.PatientName, email: d.PatientEmail,
			subject: "Your medical record was updated",
			message: fmt.Sprintf("Hello %s, a new entry has been added to your medical record. Please log in to view it.", d.PatientName),
		}}, nil
	}

	return nil, fmt.Errorf("unsupported event type %q", eventType)
}
