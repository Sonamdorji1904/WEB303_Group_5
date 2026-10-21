# Patient & Records Service — Yeshey

Go + PostgreSQL service owning patient demographics and visit records. Repository inspection found only a root README: no established stack, event schemas, RabbitMQ helper, or root Compose file. All implementation, examples, and dependencies are confined to this directory. Other services are untouched.

## Run locally

Requires Go 1.26.5+, PostgreSQL 17+, RabbitMQ 4 with quorum queues, and `psql`. The Compose example supplies PostgreSQL and RabbitMQ; Docker must be running.

From this directory:

```sh
cp .env.example .env
set -a
. ./.env
set +a
# Full standalone stack; migration completes before the service starts:
docker compose -f docker-compose.example.yml up --build
```

Or run dependencies in Docker and Go on the host:

```sh
docker compose -f docker-compose.example.yml up -d patient-db rabbitmq
docker compose -f docker-compose.example.yml run --rm patient-migrate
go run ./cmd/server
```

For existing dependencies:

```sh
# Set DATABASE_URL and RABBITMQ_URL to your own instances first.
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 --single-transaction -f migrations/001_init.sql
go run ./cmd/server
```

Migration 001 creates only this service's tables and indexes. It is repeatable, runs atomically with these commands, and records its version. Run migrations once as a deployment job before starting replicas, not concurrently. Future schema changes require new migration files; editing 001 does not upgrade existing tables. No cross-service database access is used.

| Variable | Default | Meaning |
| --- | --- | --- |
| `DATABASE_URL` | `postgres://patient:patient_local@localhost:55433/patient_db?sslmode=disable` | Patient-owned PostgreSQL database |
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` | Shared broker/vhost |
| `HTTP_ADDR` | `:8081` | HTTP listener |
| `CORS_ORIGIN` | `*` | Allowed browser origin, e.g. `http://localhost:3000` |
| `TEST_DATABASE_URL` | unset | Disposable test DB; tests create and drop isolated schemas |
| `TEST_RABBITMQ_URL` | unset | Dedicated disposable broker vhost; live tests purge patient queues |

`.env` is loaded by the shell, not automatically by Go. Compose uses its own container URLs; `PATIENT_DB_PASSWORD` and `PATIENT_RABBITMQ_URL` override Compose placeholders. Passwords embedded in URLs must be URL encoded. The example binds host ports to loopback. PostgreSQL uses host port `55433` to avoid a local PostgreSQL installation on `5432`; containers still connect to `patient-db:5432`. Set `PATIENT_DB_PORT` to override the host port and update the host-side `DATABASE_URL` accordingly. Authentication/authorization and TLS termination must be supplied by the team's gateway before exposing patient data; this service currently provides no identity layer. Broker credentials need configure/read/write access to the declared patient queues and exchanges. Use separate database credentials and restrict access to this database.

## REST contract and curl examples

Responses use camelCase. Patient IDs, record IDs, appointment IDs, and doctor IDs are canonical nonzero UUIDs. Birth dates use `YYYY-MM-DD`; visits and event timestamps use RFC3339 with an explicit offset. Outgoing event timestamps are UTC.

| Method | Path | Result |
| --- | --- | --- |
| POST | `/api/patients` | 201 patient, with `Location` header; registration event durably enqueued |
| GET | `/api/patients?limit=20&offset=0` | 200 `{data: [], limit, offset}` |
| GET | `/api/patients/{id}` | 200 patient |
| PUT | `/api/patients/{id}` | 200 patient; full demographic replacement |
| GET | `/api/patients/{id}/medical-records?limit=20&offset=0` | 200 paginated records |
| GET | `/api/medical-records/{id}` | 200 record |
| GET | `/health` | 200 when DB is reachable, otherwise 503 |

Pagination accepts limit 1–100 and nonnegative offset; default 20/0. Sort order is newest created patient first, newest visit first, with ID tie breakers. An unknown patient's record list returns 404; an existing patient without visits returns an empty array. Health checks DB availability; RabbitMQ may be down while REST remains usable and outbox entries accumulate.

Patient input requires nonblank name (max 200 bytes), gender (free text, max 50), phone (max 50), valid birth date not in the future, and optional email (max 254). Surrounding whitespace is trimmed. Invalid JSON, unknown fields, trailing JSON, invalid IDs, and invalid pagination return 400. Email may be omitted or empty. PUT requires the same fields as POST and clears email if omitted. JSON bodies are capped at 64 KiB. There is no public medical record creation endpoint.

Errors have one format, e.g. `{"error":{"code":"invalid_input","message":"Check required fields, field lengths, email and dates"}}`. Missing resources return 404 `not_found`; unsupported methods on defined paths return 405 `method_not_allowed`; database failures return 503 `unavailable`. Internal errors and clinical data are never included in errors or logs.

```sh
curl -i -X POST http://localhost:8081/api/patients \
  -H 'Content-Type: application/json' \
  -d '{"name":"Example Patient","dateOfBirth":"2000-01-01","gender":"other","phone":"+97500000000","email":"example@example.com"}'

# Copy the returned id:
PATIENT_ID='replace-with-returned-uuid'
curl "http://localhost:8081/api/patients?limit=10&offset=0"
curl "http://localhost:8081/api/patients/$PATIENT_ID"
curl -X PUT "http://localhost:8081/api/patients/$PATIENT_ID" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Example Patient","dateOfBirth":"2000-01-01","gender":"other","phone":"+97500000001"}'
curl "http://localhost:8081/api/patients/$PATIENT_ID/medical-records"
RECORD_ID='replace-with-returned-record-uuid'
curl "http://localhost:8081/api/medical-records/$RECORD_ID"
curl http://localhost:8081/health
```

## Events and teammate integration

Declare/use durable topic exchange **`hospital.events`**. Routing keys equal the case-sensitive event type. All envelopes contain `eventId` (UUID), `eventType`, `timestamp` (RFC3339), and `data`. Publishers use persistent messages and publisher confirms. Event schemas are in [examples/](examples/).

| Producer → consumer | Routing key | Required data |
| --- | --- | --- |
| Patient → Notification/others | `PatientRegistered` | `patientId`, `patientName`, `phone`, `patientEmail` |
| Appointment → Patient | `AppointmentCompleted` | `appointmentId`, `patientId`, `patientName`, `doctorId`, `doctorName`, `visitDate`, `diagnosis`, `prescription` |
| Patient → Notification/others | `MedicalRecordCreated` | `recordId`, `appointmentId`, `patientId`, `patientName`, `doctorId`, `doctorName`, `visitDate`, `phone`, `patientEmail` |

All routing keys equal the PascalCase `eventType`: `PatientRegistered`,
`AppointmentRequested`, `SlotReserved`, `SlotRejected`, `AppointmentConfirmed`,
`AppointmentRejected`, `AppointmentCompleted`, and `MedicalRecordCreated`.

The Appointment team must include **all** fields above, including both names and clinical fields. A blank prescription is invalid; use an explicit value such as `None` for a visit without medication. Clinical strings must be nonblank and at most 10,000 bytes each. Unknown event fields are tolerated for forward compatibility. Incoming timestamps and visits may be at most five minutes in the future; absent/invalid timestamps are rejected. Incoming payload limit is 1 MiB.

Patients must already be registered here. No callbacks to the Appointment or Doctor service are made. Notification contacts and the patient's name come from this service's current patient row; doctor name, diagnosis, and prescription come from the completed appointment. Outgoing notification events omit diagnosis and prescription. Email is an empty string when unavailable; Notification must select its supported channel accordingly.

Each downstream service must declare its **own durable queue and binding before events are sent**, and deduplicate by `eventId`. Confirmed but unroutable messages are treated as failed publishes and retained in the outbox. Patient service never creates or owns Notification's queue. Coordinate outgoing bindings before enabling traffic; otherwise entries eventually park after ten attempts.

There is no shared helper to import. [internal/service/broker.go](internal/service/broker.go) isolates all RabbitMQ code. Expected future shared interface: connect/reconnect with context cancellation; declare durable topic exchanges and quorum queue bindings; consume with manual ack/reject; publish persistent JSON envelopes with mandatory routing and confirms/returns; expose bounded delayed retries, DLQ routing, and close. Swapping this adapter must retain atomic DB/outbox handling; broker helpers cannot replace a database transaction.

## Reliability and failure policy

* Registration commits the patient and `PatientRegistered` outbox event together.
* Appointment processing validates the envelope, verifies the patient locally, and atomically inserts the record, processed event ID, and `MedicalRecordCreated` outbox event. Manual ACK occurs only after commit.
* An advisory transaction lock serializes the same event ID across replicas. `processed_events.event_id` and unique `medical_records.appointment_id` prevent duplicates. A different event ID for an existing appointment is recorded as processed without changing the original record or publishing another event. The first committed appointment completion wins; corrections require a separately agreed workflow.
* Outbox dispatch uses `FOR UPDATE SKIP LOCKED` and publisher confirms. Failure delays are 2, 4, 8, 16, 32, 64, 128, 256, 256 seconds, then the tenth failed attempt parks the row (`failed_at`). Workers reconnect every three seconds after connection/publish errors. Connection outages before a publish attempt do not consume attempts. Parked events need operator replay after fixing the cause.
* A crash between broker confirm and `published_at` commit can deliver the event again with the **same event ID**. The transport is at least once; consumers must deduplicate. Logical record and outgoing event creation remain idempotent.
* Invalid events go immediately to DLQ. Missing patients and database failures retry at five-second intervals, at most **five delayed retries after the original attempt**, then DLQ. Failed processing never commits a processed-event marker. A retry publication is confirmed before the original is acknowledged. If this handoff fails, the connection closes and RabbitMQ redelivers the original after reconnect.
* Main and retry queues use quorum queues, reject-publish overflow, and at-least-once dead lettering so broker transfer failures do not silently discard a committed handoff. RabbitMQ 4 quorum delivery limits are an additional bound for repeated unacknowledged disconnects. Persistent storage is required for broker restart recovery; a single-node example provides no node-failure redundancy.
* Shutdown stops workers, cancels active transactions, drains HTTP requests, and closes broker connections. Unacknowledged deliveries return to RabbitMQ. No diagnoses, prescriptions, names, contact details, event bodies, or database query errors are logged by runtime handlers/workers.

| Owned topology | Name |
| --- | --- |
| Main durable quorum queue | `patient-service.appointment-completed` |
| Delayed durable quorum queue | `patient-service.appointment-completed.retry` |
| Dead-letter exchange (direct) | `patient-service.dead` |
| Durable quorum DLQ | `patient-service.appointment-completed.dlq` (key `failed`) |

Existing queues with incompatible arguments require a coordinated migration before deployment. This service does not delete or redeclare other teams' topology.

## Publish a sample completion

Register a patient, then replace `REPLACE_WITH_REGISTERED_PATIENT_UUID` in `examples/appointment-completed.json`. Set visit and envelope timestamps to appropriate past/current values if needed.

```sh
go run ./cmd/publish-sample -file examples/appointment-completed.json
curl "http://localhost:8081/api/patients/$PATIENT_ID/medical-records"
# Replay unchanged: no additional record or logical outgoing event.
go run ./cmd/publish-sample -file examples/appointment-completed.json
```

The local publisher declares only patient-owned topology, validates the sample, and waits for a persistent mandatory publish confirmation. For a new visit, change both `appointmentId` and `eventId`; for a duplicate-appointment check change only `eventId`.

## Tests

```sh
go test -count=1 -v ./...
# All PostgreSQL cases; use a disposable DB, with CREATE schema privilege:
TEST_DATABASE_URL="$DATABASE_URL" go test -race -count=1 -v ./...
go vet ./...
go build ./...
```

Without `TEST_DATABASE_URL`, database tests explicitly skip. With it, tests apply migrations to isolated random schemas, then remove them. They cover successful registration, invalid input, patient/record HTTP lookups and update, concurrent duplicate event IDs, different IDs for the same appointment, invalid/missing patient failures, an injected outbox database failure rolling back both record processing and registration, stable outgoing event IDs, ten-attempt parking, and successful operator replay.

For live broker tests, create a **separate disposable vhost**; never point this test at a team/deployed vhost (it purges the three patient queues). With the example broker:

```sh
docker compose -f docker-compose.example.yml exec rabbitmq rabbitmqctl add_vhost patient-test
docker compose -f docker-compose.example.yml exec rabbitmq rabbitmqctl set_permissions -p patient-test guest '.*' '.*' '.*'
TEST_DATABASE_URL="$DATABASE_URL" \
TEST_RABBITMQ_URL='amqp://guest:guest@localhost:5672/patient-test' \
  go test -race -count=1 -v ./internal/service -run TestRabbitMQWorkflow -timeout 90s
```

The optional live test checks confirmed publication, mandatory returns, consumption, duplicate deliveries, outgoing dispatch, consumer stop/restart, immediate invalid-event DLQ, and missing-patient retry exhaustion. It skips explicitly when `TEST_RABBITMQ_URL` is unset.

## Manual restart, retry and DLQ demonstration

Use synthetic data only. Notification (or a local test queue) must already bind the two outgoing event keys. For a local test queue, in RabbitMQ management at `http://localhost:15672` (local example `guest`/`guest`), declare a durable quorum queue `patient-local-observer`; add bindings from `hospital.events` with keys `PatientRegistered` and `MedicalRecordCreated`. This is a local observer, not a Notification implementation.

1. Stop RabbitMQ with `docker compose -f docker-compose.example.yml stop rabbitmq`. Register a new patient via curl; 201 still succeeds. Inspect the pending outbox with the SQL below.
2. Restart the patient process (`docker compose -f docker-compose.example.yml restart patient-service`), then the broker (`docker compose -f docker-compose.example.yml start rabbitmq`). Pending events publish after reconnect, keeping their original IDs. The management observer queue shows them; SQL shows `published_at` populated.
3. Stop patient service, publish a valid sample completion, restart patient service. The durable queue retains the event and the visit appears after startup. Replay the same JSON twice; there is still one record and one `MedicalRecordCreated` outbox row for that appointment.
4. For a missing-patient retry, change sample `eventId` and `appointmentId` to fresh UUIDs and `patientId` to a valid UUID that does not exist. Publish. Inspect the retry queue during the next 25–35 seconds; the message then reaches the DLQ without any record or processed marker. If the patient is legitimately available after remediation, replay from DLQ with the original event ID.
5. For invalid-event DLQ, publish raw JSON `{"bad":"event"}` to `hospital.events` with routing key `AppointmentCompleted` using management UI. The sample tool deliberately refuses invalid messages. The consumer rejects this immediately into DLQ.

Read-only counts through CLI:

```sh
docker compose -f docker-compose.example.yml exec rabbitmq rabbitmqctl list_queues name messages_ready messages_unacknowledged
psql "$DATABASE_URL" -c "SELECT event_id,event_type,attempts,published_at,failed_at,next_attempt_at FROM outbox ORDER BY created_at;"
psql "$DATABASE_URL" -c "SELECT appointment_id,count(*) FROM medical_records GROUP BY appointment_id;"
psql "$DATABASE_URL" -c "SELECT event_id,event_type FROM processed_events;"
```

Inspect the DLQ in management UI: Queues → `patient-service.appointment-completed.dlq` → Get messages, with **requeue enabled**. Inspect `x-death` and `x-patient-retries`; avoid copying clinical payloads into logs. Do not purge failed records as a replay mechanism. After resolving the cause, republish the original envelope to `hospital.events` / `AppointmentCompleted` as a persistent confirmed message, clearing retry/death headers. Only remove its DLQ copy once republishing is confirmed. An unchanged event ID makes replay safe. Permanently invalid contracts need agreement and correction before replay.

After fixing outgoing routing/broker issues, re-enable a parked outbox row without replacing its event ID:

```sql
UPDATE outbox
SET attempts=0, failed_at=NULL, next_attempt_at=now()
WHERE event_id='REPLACE-WITH-OUTGOING-EVENT-UUID'
  AND published_at IS NULL;
```

## Verification in the implementation environment

Completed: Go build, vet, and race-enabled unit/PostgreSQL tests (see final delivery for actual run results). Database tests ran against a temporary local PostgreSQL cluster, including fault injection and concurrent deliveries. Live RabbitMQ workflow and Docker build/Compose startup were not run: Docker daemon was unavailable and there was no installed RabbitMQ server. The optional test and manual steps above are provided for a broker-enabled machine. No claim of a completed live restart/DLQ demonstration is made.
