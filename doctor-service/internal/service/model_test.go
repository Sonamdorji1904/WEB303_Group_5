package service

import "testing"

func TestDoctorInputValidation(t *testing.T) {
	if err := (&DoctorInput{Name: "Dr A", Specialization: "Cardiology", Phone: "123"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (&DoctorInput{Name: "", Specialization: "Cardiology", Phone: "123"}).Validate(); err != ErrInvalid {
		t.Fatalf("want invalid, got %v", err)
	}
}

func TestSlotInputValidation(t *testing.T) {
	if err := (&SlotInput{Date: "2026-11-01", StartTime: "09:30"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (&SlotInput{Date: "2026-11-01", StartTime: "9:30"}).Validate(); err != ErrInvalid {
		t.Fatalf("want invalid, got %v", err)
	}
}

func TestAppointmentRequestedValidation(t *testing.T) {
	body := []byte(`{"eventId":"11111111-1111-4111-8111-111111111111","eventType":"AppointmentRequested","timestamp":"2026-11-01T09:30:00Z","data":{"appointmentId":"22222222-2222-4222-8222-222222222222","patientId":"33333333-3333-4333-8333-333333333333","patientName":"Patient","patientEmail":"patient@example.com","doctorId":"44444444-4444-4444-8444-444444444444","doctorName":"Doctor","date":"2026-11-01","time":"09:30","reason":"Checkup"}}`)
	if _, _, err := parseRequested(body); err != nil {
		t.Fatalf("valid request event rejected: %v", err)
	}
	invalid := []byte(`{"eventId":"11111111-1111-4111-8111-111111111111","eventType":"AppointmentRequested","timestamp":"2026-11-01T09:30:00Z","data":{"appointmentId":"not-a-uuid","patientId":"33333333-3333-4333-8333-333333333333","patientName":"Patient","doctorId":"44444444-4444-4444-8444-444444444444","date":"2026-11-01","time":"09:30"}}`)
	if _, _, err := parseRequested(invalid); err != ErrInvalid {
		t.Fatalf("want invalid request event, got %v", err)
	}
}
