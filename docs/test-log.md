# Shared-stack end-to-end test log

Run date: 2026-10-21

The root stack was started with:

```sh
docker compose up --build -d
docker compose ps
```

All five long-running containers were healthy/running. The migration job
`patient-migrate` exited successfully after applying `001_init.sql`.

| Scenario | Steps | Expected result | Actual result | Status |
|---|---|---|---|---|
| A. Direct publish | `cd patient-service && RABBITMQ_URL='amqp://guest:guest@localhost:5672/' go run ./cmd/publish-sample -file examples/patient-registered.json`; then `curl -sS http://localhost:8084/api/notifications` | PatientRegistered is published through the shared broker and stored as one notification | Publisher output: `event published eventId=3836f2ee-d95c-4bec-bb50-af167449a800 eventType=PatientRegistered` and `Event confirmed by RabbitMQ`. Notification response contained recipient `Example Patient`, email `example@example.com`, and event type `PatientRegistered`. | PASS |
| B. Real patient flow | `curl -X POST http://localhost:8081/api/patients ...`; then `curl -sS http://localhost:8084/api/notifications` | HTTP 201; outbox event reaches Notification; recipient name/email match the patient | Response was `HTTP/1.1 201 Created`, patient ID `742d4444-a331-4780-8d95-eda66b44b5f0`. After asynchronous outbox dispatch, notification contained `Test Flow Patient`, `flow@example.com`, and event ID `e9723842-b683-4d04-902b-2e3c5662171b`. | PASS |
| C. Duplicate event | Publish `examples/patient-registered.json` twice using the Test A command; count `3836f2ee-d95c-4bec-bb50-af167449a800` in the notifications response | Duplicate delivery is acknowledged but creates only one notification | Both publishes returned `Event confirmed by RabbitMQ`; `grep ... | wc -l` returned `1`. Notification logs reported `duplicate event ignored`. | PASS |
| D. Failure and recovery | `docker compose stop notification-service`; POST a patient; poll `rabbitmqctl list_queues name messages_ready messages_unacknowledged consumers`; `docker compose start notification-service`; query notifications | Event remains durable in the queue while the consumer is stopped and is processed after restart | For patient `Queue Proof Patient`, polling changed from `0 0 0` to `1 0 0` for `notification-service.queue` and remained queued through poll 10. After restart, the notification count for `Queue Proof Patient` was `2` across the two recovery runs. | PASS |

## Commands used

### Test A

```sh
cd patient-service
RABBITMQ_URL='amqp://guest:guest@localhost:5672/' \
  go run ./cmd/publish-sample -file examples/patient-registered.json
curl -sS http://localhost:8084/api/notifications
```

## Doctor and appointment service verification

The expanded stack was rebuilt with `docker compose up --build -d`. All
application health endpoints returned `200`; PostgreSQL and RabbitMQ containers
were healthy. During the first workflow run, the appointment event was found to
omit `appointmentId`; this was fixed in the appointment outbox payload and the
workflow was rerun successfully.

| Scenario | Steps | Expected result | Actual result | Status |
|---|---|---|---|---|
| E. Full appointment chain | Register patient, create doctor and slot, POST appointment, poll status, complete it, GET `/api/patients/{id}/medical-records` and notifications | PENDING becomes CONFIRMED, completion becomes COMPLETED, one medical record and five notification event types exist | Patient `82d5f344-f7c8-4d01-ae35-10768904d2d1`; appointment `9f7a782a-8163-4d4a-9df1-9a777a476912`; status `CONFIRMED` before completion; medical record returned with diagnosis `Stable`; notification types were `PatientRegistered`, `AppointmentRequested`, `AppointmentConfirmed`, `AppointmentCompleted`, `MedicalRecordCreated`. | PASS |
| F. Reserved-slot rejection | Book the already-reserved slot again and GET the appointment and notifications | Appointment becomes REJECTED and AppointmentRejected is notified | Appointment `e2d3e9e3-e342-4303-902d-c86ab532945d` returned `REJECTED` with reason `requested slot is unavailable`; notification types included `AppointmentRejected`. | PASS |
| G. Doctor failure and recovery | Stop doctor service, create an appointment, poll RabbitMQ management API, start doctor service, poll appointment | Appointment remains PENDING, message waits in doctor queue, then becomes CONFIRMED | Queue reported `messages_ready=1`; appointment `9e021e65-ae49-435c-99d0-8584d5f8d53d` was `PENDING` before restart and `CONFIRMED` after restart. | PASS |
| H. Duplicate delivery | Publish the same AppointmentRequested envelope twice with `appointment-service/cmd/publish-sample -count 2`; inspect reserved slots | One reservation and one SlotReserved result | Publisher reported `published 11111111-1111-4111-8111-111111111111 2 time(s)`; the doctor slot had one `RESERVED` row with one appointment ID. | PASS |
| I. Dead letter | Publish an envelope with valid metadata but invalid AppointmentRequested data; poll the doctor DLQ and main queue | Invalid event reaches DLQ after retries and does not block valid traffic | Doctor logs showed retries 1, 2, 3 followed by dead-lettering for event `44444444-4444-4444-8444-444444444444`; DLQ reached 2 messages and the main queue remained at `messages_ready=0`. | PASS |

### Commands used for E-I

```sh
docker compose ps
for p in 8081 8082 8083 8084; do
  curl -sS -o /dev/null -w "$p %{http_code}\n" "http://localhost:$p/health"
done

curl -sS -X POST http://localhost:8081/api/patients \
  -H 'Content-Type: application/json' \
  -d '{"name":"Medical Record Patient","email":"record.patient@example.com","dateOfBirth":"1990-01-02","gender":"M","phone":"555-0102"}'
curl -sS -X POST http://localhost:8082/api/doctors \
  -H 'Content-Type: application/json' \
  -d '{"name":"Record Doctor","specialization":"General Practice","email":"record.doctor@example.com","phone":"555-0202"}'
curl -sS -X POST http://localhost:8082/api/doctors/{doctorId}/slots \
  -H 'Content-Type: application/json' \
  -d '{"date":"2026-10-21","startTime":"11:26"}'
curl -sS -X POST http://localhost:8083/api/appointments \
  -H 'Content-Type: application/json' \
  -d '{"patientId":"{patientId}","patientName":"Medical Record Patient","patientEmail":"record.patient@example.com","doctorId":"{doctorId}","doctorName":"Record Doctor","date":"2026-10-21","time":"11:26","reason":"Checkup"}'
curl -sS -X PUT http://localhost:8083/api/appointments/{appointmentId}/complete \
  -H 'Content-Type: application/json' \
  -d '{"diagnosis":"Stable","prescription":"Rest","notes":"No concerns"}'
curl -sS http://localhost:8081/api/patients/{patientId}/medical-records
curl -sS http://localhost:8084/api/notifications

docker compose stop doctor-service
curl -sS -u guest:guest \
  http://localhost:15672/api/queues/%2F/doctor-service.queue \
  | jq '{messages_ready,messages_unacknowledged}'
docker compose start doctor-service

(cd appointment-service && go run ./cmd/publish-sample \
  -file /tmp/duplicate-appointment-requested.json -count 2)
(cd appointment-service && go run ./cmd/publish-sample \
  -file /tmp/malformed-event.json)
curl -sS -u guest:guest \
  http://localhost:15672/api/queues/%2F/doctor-service.dlq \
  | jq '{messages,messages_ready}'
```

## UI testing

The Next.js dashboard was built with Next.js 16.4.2 and served from the
Compose frontend container on port 3000.

| Scenario | Expected | Actual | Status |
|---|---|---|---|
| Frontend build and lint | Production build and lint complete without type errors | `npm run lint` completed with exit code 0; `npm run build` generated routes `/`, `/patients`, `/doctors`, `/appointments`, and `/notifications` successfully. | PASS |
| Compose dashboard startup | Frontend and all APIs are reachable | `docker compose ps` showed frontend on `0.0.0.0:3000->3000`, all four APIs running, all databases healthy, and RabbitMQ healthy. `curl http://localhost:3000/` returned `200`; all API health endpoints returned `200`. | PASS |
| Browser registration and scheduling | Register patient, add doctor and slot, book appointment, and observe event-driven confirmation | Playwright registered `UI Flow Patient`, added `UI Flow Doctor`, added slot `2026-12-20 · 14:00`, booked `UI verification visit`; the UI first showed `PENDING` and then `CONFIRMED` after polling. | PASS |
| Browser complete visit | Complete a confirmed visit and show completion feedback | The modal accepted diagnosis `UI verified`, prescription `None`, and notes; toast displayed `Visit completed. Medical record creation is now in progress.` and the row changed to `COMPLETED`. | PASS |
| Doctor outage and recovery | Red service dot and pending appointment while Doctor is stopped; green dot and confirmation after restart | After `docker compose stop doctor-service`, the dashboard header contained the red Doctor dot and the appointments page showed `Doctor outage proof` as `PENDING`. After `docker compose start doctor-service`, the same row changed to `CONFIRMED` through polling. | PASS |
| Patient CORS | Browser requests from `http://localhost:3000` must reach every API | Patient service initially blocked browser requests because it had no CORS middleware. Added configurable `CORS_ORIGIN` support and set `http://localhost:3000` in Compose; browser requests then succeeded. | PASS |

### UI commands

```sh
cd frontend
npm install
npm run lint
npm run build

cd ..
docker compose up --build -d
curl -sS -o /dev/null -w '%{http_code}\n' http://localhost:3000/
curl -sS -o /dev/null -w '%{http_code}\n' http://localhost:8081/health
curl -sS -o /dev/null -w '%{http_code}\n' http://localhost:8082/health
curl -sS -o /dev/null -w '%{http_code}\n' http://localhost:8083/health
curl -sS -o /dev/null -w '%{http_code}\n' http://localhost:8084/health

docker compose stop doctor-service
docker compose start doctor-service
```

## Doctor approval flow

| Scenario | Steps | Expected result | Actual result | Status |
|---|---|---|---|---|
| J. Hold and approve | Created appointment `3af9ed24-c5a2-424c-85b3-3b158a042306`, listed requests, then accepted request `1427c72f-203d-467b-9000-5b2bd88477d1` | Appointment stays PENDING while the slot is HELD; accepting changes the slot to RESERVED and appointment to CONFIRMED | Slot response showed `HELD` before acceptance; accept returned `ACCEPTED`; appointment returned `CONFIRMED`; slot returned `RESERVED` | PASS |
| K. Reject with reason | Created appointment `93e1fd5b-a113-47ab-9314-9645c9a575a9`, then rejected request `6b75e4e3-2573-4f15-a986-25317815c725` with `{ "reason": "Doctor unavailable for this clinic session" }` | Slot returns to AVAILABLE, appointment becomes REJECTED, and the UI shows the reason | Reject returned `REJECTED` with the supplied reason; appointment returned `REJECTED` with `rejectionReason` set to `Doctor unavailable for this clinic session`; the released slot was confirmed AVAILABLE by date/time lookup | PASS |

The approval endpoints and frontend page are covered by the Doctor service
store logic and frontend production build.

Commands used:

```sh
curl -sS http://localhost:8082/api/requests
curl -sS -X POST http://localhost:8082/api/requests/{requestId}/accept
curl -sS -X POST http://localhost:8082/api/requests/{requestId}/reject \
  -H 'Content-Type: application/json' \
  -d '{"reason":"Doctor unavailable for this clinic session"}'
curl -sS http://localhost:8083/api/appointments/{appointmentId}
curl -sS http://localhost:8082/api/doctors/{doctorId}/slots
```

### Test B

```sh
curl -sS -i -X POST http://localhost:8081/api/patients \
  -H 'Content-Type: application/json' \
  -d '{"name":"Test Flow Patient","dateOfBirth":"1990-05-10","gender":"other","phone":"+97517123456","email":"flow@example.com"}'
sleep 4
curl -sS http://localhost:8084/api/notifications
```

### Test C

```sh
cd patient-service
RABBITMQ_URL='amqp://guest:guest@localhost:5672/' \
  go run ./cmd/publish-sample -file examples/patient-registered.json
RABBITMQ_URL='amqp://guest:guest@localhost:5672/' \
  go run ./cmd/publish-sample -file examples/patient-registered.json
curl -sS http://localhost:8084/api/notifications \
  | grep -o '3836f2ee-d95c-4bec-bb50-af167449a800' | wc -l
```

### Test D

```sh
docker compose stop notification-service
sleep 5
curl -sS -X POST http://localhost:8081/api/patients \
  -H 'Content-Type: application/json' \
  -d '{"name":"Queue Proof Patient","dateOfBirth":"1987-04-22","gender":"other","phone":"+97517444444","email":"queueproof@example.com"}'
docker compose exec -T rabbitmq rabbitmqctl \
  list_queues name messages_ready messages_unacknowledged consumers
docker compose start notification-service
sleep 8
curl -sS http://localhost:8084/api/notifications
```
