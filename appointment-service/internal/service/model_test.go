package service

import "testing"

func TestAppointmentValidation(t *testing.T) {
	in := AppointmentInput{
		PatientID:   "00000000-0000-0000-0000-000000000001",
		PatientName: "Pema", DoctorID: "00000000-0000-0000-0000-000000000002",
		DoctorName: "Karma", Date: "2026-11-01", Time: "09:30",
	}
	if err := in.Validate(); err != nil {
		t.Fatal(err)
	}
	in.Time = "9:30"
	if err := in.Validate(); err != ErrInvalid {
		t.Fatalf("want invalid, got %v", err)
	}
}

func TestCompletionRequiresClinicalFields(t *testing.T) {
	if err := (&CompletionInput{Diagnosis: "Flu", Prescription: ""}).Validate(); err != ErrInvalid {
		t.Fatalf("want invalid, got %v", err)
	}
}
