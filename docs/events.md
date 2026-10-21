# Hospital events

This is the source of truth for the event contract shared by all services.

## Envelope

Every message is JSON with:

```json
{
  "eventId": "uuid",
  "eventType": "PatientRegistered",
  "timestamp": "2026-10-20T10:15:30Z",
  "data": {}
}
```

`eventId` is a UUID, `timestamp` is ISO/RFC3339, and `data` is an object.
The RabbitMQ topic exchange is `hospital.events`. Every routing key is exactly
the PascalCase `eventType`.

## Events

| Event type and routing key | Publisher | Consumers |
|---|---|---|
| `PatientRegistered` | Patient | Notification |
| `AppointmentRequested` | Appointment | Doctor, Notification |
| `SlotReserved` | Doctor | Appointment |
| `SlotRejected` | Doctor | Appointment |
| `AppointmentConfirmed` | Appointment | Notification |
| `AppointmentRejected` | Appointment | Notification |
| `AppointmentCompleted` | Appointment | Patient, Notification |
| `MedicalRecordCreated` | Patient | Notification |

`PatientRegistered` data includes `patientId`, `patientName`, `phone`, and
`patientEmail`.

`MedicalRecordCreated` data includes `recordId`, `appointmentId`, `patientId`,
`patientName`, `doctorId`, `doctorName`, `visitDate`, `phone`, and
`patientEmail`.

`AppointmentRejected` data includes `appointmentId`, `patientId`, `patientName`,
`patientEmail`, `doctorName`, `date`, `time`, and `reason`.

`AppointmentRequested` data includes `appointmentId`, `patientId`, `patientName`,
`patientEmail`, `doctorId`, `doctorName`, `date`, `time`, and `reason`.

`SlotReserved` data includes `appointmentId`, `doctorId`, `doctorName`,
`patientId`, `patientName`, `patientEmail`, `date`, and `time`.

`SlotRejected` contains those same fields plus `reason`.

Appointment events use the fields required by their workflow, including
`patientId`, `patientName`, `patientEmail`, appointment/doctor details, and
date/time fields. Clinical fields are `diagnosis` and `prescription` on
`AppointmentCompleted`; it also includes `appointmentId`, `doctorId`,
`doctorName`, `date`, `time`, `visitDate`, and `notes`. Clinical fields must not
be included in notification text.
