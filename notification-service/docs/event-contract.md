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
| PatientRegistered | `PatientRegistered` | Patient | Notification |
| AppointmentRequested | `AppointmentRequested` | Appointment | Doctor, Notification |
| SlotReserved | `SlotReserved` | Doctor | Appointment |
| SlotRejected | `SlotRejected` | Doctor | Appointment |
| AppointmentConfirmed | `AppointmentConfirmed` | Appointment | Notification |
| AppointmentRejected | `AppointmentRejected` | Appointment | Notification |
| AppointmentCompleted | `AppointmentCompleted` | Appointment | Patient, Notification |
| MedicalRecordCreated | `MedicalRecordCreated` | Patient | Notification |

Notification Service binds: `PatientRegistered`, `AppointmentRequested`,
`AppointmentConfirmed`, `AppointmentRejected`, `AppointmentCompleted`,
`MedicalRecordCreated`. It does not consume `SlotReserved` / `SlotRejected`.

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

`AppointmentRejected` is supported and contains `appointmentId`, `patientId`,
`patientName`, `patientEmail`, `doctorName`, `date`, `time`, and `reason`.
Patient events use `patientEmail` consistently; `email` is not an event field.
