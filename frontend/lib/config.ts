export const serviceUrls = {
  patient: process.env.NEXT_PUBLIC_PATIENT_URL ?? "http://localhost:8081",
  doctor: process.env.NEXT_PUBLIC_DOCTOR_URL ?? "http://localhost:8082",
  appointment: process.env.NEXT_PUBLIC_APPOINTMENT_URL ?? "http://localhost:8083",
  notification: process.env.NEXT_PUBLIC_NOTIFICATION_URL ?? "http://localhost:8084",
} as const;
