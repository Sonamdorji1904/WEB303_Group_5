# Notification Service

Part of the WEB303 Lab 6 Hospital Management System. It is a pure **event consumer**: it
listens to events from the other services, turns them into notifications (patient and
doctor), stores them in its own database, and exposes them over REST for the frontend.
It never calls another service and no other service reads its database.

| | |
|---|---|
| Language | Go 1.22+ (standard library `net/http`) |
| Broker | RabbitMQ, exchange `hospital.events`, queue `notification-service.queue` |
| Database | PostgreSQL (`notifications`, `processed_events`) |
| Port | 8084 |

## Events consumed

| Event | Notification created |
|---|---|
| PatientRegistered | Patient: welcome message |
| AppointmentRequested | Patient: "request received" |
| AppointmentConfirmed | Patient **and** doctor: appointment confirmed |
| AppointmentRejected (proposed) | Patient: slot unavailable |
| AppointmentCompleted | Patient: visit complete |
| MedicalRecordCreated | Patient: record updated |

Full payload definitions are in [docs/event-contract.md](docs/event-contract.md).

## Reliability

- **Durable** exchange/queues and persistent messages: events published while the service is down are kept and processed when it restarts.
- **Manual acks**: a message is only acknowledged after the notification is saved.
- **Retry then DLQ**: transient failures (database or sender down) go to `notification-service.retry`, wait `RETRY_DELAY_SECONDS`, and come back, up to `MAX_RETRIES`; after that they are parked in `notification-service.dlq` with the error in the `x-last-error` header. Permanent failures (bad JSON, unknown event type, missing fields) go straight to the DLQ.
- **Idempotency**: the `eventId` is recorded in `processed_events` in the same transaction as the notifications; a repeated `eventId` is acknowledged and ignored, and nothing is sent twice. A unique constraint on `(event_id, recipient_type, recipient_id)` is a second safety net.
- **Auto-reconnect** to RabbitMQ with backoff, and database connection retry at startup. `/health` reports `DEGRADED` while either is down.
- **Logging**: every event processed, ignored duplicate, retry, and dead-letter is logged with `eventId` and `eventType`.

## REST API

| Method | Path | Description |
|---|---|---|
| GET | `/health` | Service, database and RabbitMQ status |
| GET | `/api/notifications` | List, newest first. Query: `recipientType` (PATIENT/DOCTOR), `recipientId`, `eventType`, `unread=true`, `limit` |
| GET | `/api/notifications/{id}` | One notification |
| PATCH | `/api/notifications/{id}/read` | Mark as read |
| GET | `/api/processed-events` | Event IDs already processed (evidence for the duplicate test) |

Example: `curl "http://localhost:8084/api/notifications?recipientType=PATIENT&recipientId=p-1"`

## Run it

Standalone (own RabbitMQ and database):

```bash
docker compose -f docker-compose.dev.yml up --build
./scripts/demo.sh happy      # publish a full sample workflow
./scripts/demo.sh show       # see the results
```

Inside the full system: copy the services from `docker-compose.snippet.yml` into the root `docker-compose.yml`.

Locally with Go: start Postgres and RabbitMQ (e.g. `docker compose -f docker-compose.dev.yml up -d rabbitmq notification-db`), then `cp .env.example .env`, export the variables, and `go run ./cmd/server`.

Tests: `go test ./... -v`

## Configuration

| Variable | Default |
|---|---|
| `PORT` | `8084` |
| `DATABASE_URL` | `postgres://notification:notification@localhost:5435/notification_db?sslmode=disable` |
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` |
| `RABBITMQ_EXCHANGE` | `hospital.events` |
| `RABBITMQ_QUEUE` | `notification-service.queue` |
| `MAX_RETRIES` | `3` |
| `RETRY_DELAY_SECONDS` | `5` |
| `PREFETCH` | `5` |
| `CORS_ORIGIN` | `*` |

## Layout

```
cmd/server/            entry point
internal/events/       envelope, event types, routing keys
internal/messaging/    RabbitMQ consumer: topology, acks, retry, DLQ, reconnect
internal/service/      event -> notification logic and message templates
internal/store/        Store interface, Postgres implementation, in-memory test store
internal/sender/       delivery (logs the message; swap in SMTP if wanted)
internal/api/          REST handlers
docs/                  event contract, test plan
scripts/demo.sh        publish sample events
```

## Note for the shared RabbitMQ helper

`internal/messaging` is self-contained so this service works on its own. If the shared
helper (retry, DLQ, idempotency) lands in the repo, `consumer.go` can be replaced by it;
the handler signature `func(ctx, body []byte) error` and the `ErrPermanent` convention
are all the rest of the service depends on.
