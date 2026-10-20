# Notification Service: test evidence (Lab 6, Part G)

Start the service alone: `docker compose -f docker-compose.dev.yml up --build -d`
RabbitMQ UI: http://localhost:15672 (guest / guest). Helper: `./scripts/demo.sh`.
Fill in **Actual result** and attach a screenshot for each row before the report is submitted.

| # | Scenario | Steps | Expected | Actual result |
|---|---|---|---|---|
| 1 | Event delivery | `./scripts/demo.sh happy`, then `./scripts/demo.sh show` | 6 notifications: welcome, request received, confirmed (patient), confirmed (doctor), visit complete, record updated; 5 processed events | |
| 2 | Multiple consumers (confirmed) | Same run | `AppointmentConfirmed` produces one PATIENT and one DOCTOR notification | |
| 3 | Duplicate event | `./scripts/demo.sh duplicate`, then `show` | Exactly one welcome notification for the repeated `eventId`; log shows "duplicate event ignored" twice; one row in `/api/processed-events` | |
| 4 | Consumer unavailable | `docker compose -f docker-compose.dev.yml stop notification-service`, run `happy` | Events wait in `notification-service.queue` (Ready count rises in RabbitMQ UI); nothing lost | |
| 5 | Consumer recovery | `docker compose -f docker-compose.dev.yml start notification-service` | Queue drains to 0; notifications appear in `/api/notifications` | |
| 6 | Transient failure then retry | Stop the DB (`stop notification-db`), run `happy`, start the DB within ~15 s | Logs show "scheduling retry"; messages land in `notification-service.retry`, then are processed after the DB returns | |
| 7 | Dead-letter queue | Keep the DB down past 3 retries, or run `./scripts/demo.sh poison` | Failed messages end up in `notification-service.dlq` with `x-last-error` header; permanent errors skip retries | |
| 8 | Full workflow (with other services) | Register patient, book, confirm, complete via the UI | Notifications appear for every step; REJECTED path notifies the patient (if `AppointmentRejected` is adopted) | |

Automated unit tests (no Docker needed): `go test ./... -v`
