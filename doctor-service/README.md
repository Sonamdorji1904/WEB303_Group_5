# Doctor Service

The Doctor Service owns doctors, appointment slots, and approval requests. It
consumes `AppointmentRequested` from `hospital.events`, places an available
slot on hold, and waits for a doctor decision before publishing `SlotReserved`
or `SlotRejected`.

## Run

From the repository root:

```sh
docker compose up --build -d doctor-db doctor-migrate doctor-service
```

Local defaults are in `.env.example`. The service uses PostgreSQL on port 5434,
HTTP on port 8082, and the shared RabbitMQ broker on port 5672.

## REST endpoints

- `POST /api/doctors`
- `GET /api/doctors`
- `GET /api/doctors/{id}`
- `POST /api/doctors/{id}/slots`
- `GET /api/doctors/{id}/slots?status=AVAILABLE`
- `GET /api/requests` (pending requests by default; accepts `status=ACCEPTED|REJECTED`)
- `POST /api/requests/{id}/accept`
- `POST /api/requests/{id}/reject` with `{ "reason": "..." }`
- `GET /health`

## Events

- Consumes `AppointmentRequested`.
- A successful request changes the slot to `HELD` and creates a `PENDING`
  approval request without publishing a slot event.
- Accepting a request changes the slot to `RESERVED` and publishes `SlotReserved`.
- Rejecting a request releases the slot to `AVAILABLE` and publishes
  `SlotRejected` with the doctor's reason.

Events are published through a transactional outbox. The consumer uses manual
acknowledgements, three retries, a dead-letter queue, and `processed_events`
idempotency.
