# Appointment Service

The Appointment Service owns appointment state. It publishes
`AppointmentRequested`, consumes `SlotReserved` and `SlotRejected`, publishes
`AppointmentConfirmed`/`AppointmentRejected`, and publishes
`AppointmentCompleted` after a confirmed visit is completed.

## Run

From the repository root:

```sh
docker compose up --build -d appointment-db appointment-migrate appointment-service
```

The service uses PostgreSQL on port 5436, HTTP on port 8083, and the shared
RabbitMQ broker on port 5672.

## REST endpoints

- `POST /api/appointments`
- `GET /api/appointments?status=&patientId=`
- `GET /api/appointments/{id}`
- `PUT /api/appointments/{id}/complete`
- `GET /health`

Appointment creation requires patient and doctor IDs plus the display names and
email supplied by the UI. The service never reads another service's database.

## Events

All messages use the durable `hospital.events` topic exchange and PascalCase
routing keys. The service has its own durable queue, retry queue, DLQ,
transactional outbox, and `processed_events` idempotency table.
