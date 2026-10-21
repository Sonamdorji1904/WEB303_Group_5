# Event contract: hospital.events

Shared by all four services. The Notification Service is written against exactly this
contract, so please review it and flag anything that differs from what you publish.

## Broker conventions

| Item | Value |
|---|---|
| Broker | RabbitMQ |
| Exchange | `hospital.events`, type `topic`, **durable** |
| Queue per consumer | `<service>.queue` (durable): `notification-service.queue`, `doctor-service.queue`, ... |
| Retry queue | `<service>.retry` (durable, per-message TTL, dead-letters back into `<service>.queue`) |
| Dead-letter queue | `<service>.dlq` (durable) |
| Messages | `delivery_mode = 2` (persistent), `content_type = application/json` |
| Acks | manual; ack only after processing succeeded |

Publishers should publish **after** their own database commit, and must not rely on the
consumers being online (durable queues hold the message).

## Envelope (every event)

```json
{
  "eventId": "3f1c2a52-6c43-4a8e-9a6b-0b5f2f0f6c11",
  "eventType": "AppointmentConfirmed",
  "timestamp": "2026-10-20T10:15:30Z",
  "data": { }
}
```

- `eventId`: new UUID per event. Re-publishing the same event reuses the same `eventId`
  (this is what the idempotency check relies on).
- `eventType`: PascalCase name from the table below. The Notification Service dispatches
  on this field.
- `timestamp`: RFC 3339, UTC.
- IDs (`patientId`, `doctorId`, `appointmentId`, `recordId`) may be strings or numbers.
- `date` is `YYYY-MM-DD`, `time` is `HH:MM` (24 h).

## Events and routing keys

| eventType | Routing key | Publisher | Consumers |
|---|---|---|---|
| PatientRegistered | `patient.registered` | Patient | Notification |
| AppointmentRequested | `appointment.requested` | Appointment | Doctor, Notification |
| SlotReserved | `slot.reserved` | Doctor | Appointment |
| SlotRejected | `slot.rejected` | Doctor | Appointment |
| AppointmentConfirmed | `appointment.confirmed` | Appointment | Notification |
| **AppointmentRejected** (proposed) | `appointment.rejected` | Appointment | Notification |
| AppointmentCompleted | `appointment.completed` | Appointment | Patient, Notification |
| MedicalRecordCreated | `medical_record.created` | Patient | Notification |

Notification Service binds: `patient.registered`, `appointment.requested`,
`appointment.confirmed`, `appointment.rejected`, `appointment.completed`,
`medical_record.created`. It does not consume `slot.reserved` / `slot.rejected`.

## `data` fields the Notification Service needs

`R` = required (event is dead-lettered if missing), `O` = optional.

| Field | PatientRegistered | AppointmentRequested | AppointmentConfirmed | AppointmentRejected | AppointmentCompleted | MedicalRecordCreated |
|---|:-:|:-:|:-:|:-:|:-:|:-:|
| patientId | R | R | R | R | R | R |
| patientName | R | R | R | R | R | R |
| patientEmail | O | O | O | O | O | O |
| appointmentId | | R | R | R | R | O |
| doctorId | | | R | O | O | |
| doctorName | | R | R | O | R | O |
| doctorEmail | | | O | | | |
| date | | R | R | R | R | |
| time | | R | R | R | O | |
| reason | | | | O | | |
| recordId | | | | | | O |
| diagnosis / prescription | | | | | O (not used in text) | O (not used in text) |

Notification text never includes diagnosis or prescription; medical details stay in the
Patient & Records Service.

## Decision needed from the team

1. **Rejected appointments.** The plan stops at `SlotRejected -> status REJECTED`, so the
   patient never hears about it. Proposal: Appointment Service also publishes
   `AppointmentRejected` (fields above) after it sets REJECTED. The Notification Service
   already handles it.
2. **Routing keys** above are my proposal (the plan only named event types). If you use
   different keys, tell me; it is a one-line change in `internal/events/events.go`.
3. **Doctor email** is optional; if Appointment has it, include `doctorEmail` in
   `AppointmentConfirmed`.
