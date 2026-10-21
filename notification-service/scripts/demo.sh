#!/usr/bin/env sh
# Publishes sample events to RabbitMQ (through its management API) so the
# Notification Service can be tested without the other services running.
#
#   ./scripts/demo.sh happy       full workflow: register -> request -> confirm -> complete -> record
#   ./scripts/demo.sh duplicate   publish the SAME eventId three times (idempotency test)
#   ./scripts/demo.sh poison      malformed + invalid events (dead-letter queue test)
#   ./scripts/demo.sh show        print notifications, processed events and queue depths
set -eu

RABBIT="${RABBIT_API:-http://localhost:15672}"
AUTH="${RABBIT_AUTH:-guest:guest}"
API="${NOTIFICATION_API:-http://localhost:8084}"
EXCHANGE="${EXCHANGE:-hospital.events}"

uuid() { uuidgen | tr 'A-Z' 'a-z'; }
now()  { date -u +%Y-%m-%dT%H:%M:%SZ; }

# publish <routing-key> <raw payload string>
publish() {
  escaped=$(printf '%s' "$2" | sed 's/\\/\\\\/g; s/"/\\"/g')
  body=$(printf '{"properties":{"delivery_mode":2,"content_type":"application/json"},"routing_key":"%s","payload":"%s","payload_encoding":"string"}' "$1" "$escaped")
  out=$(curl -s -u "$AUTH" -H 'content-type: application/json' -X POST "$RABBIT/api/exchanges/%2F/$EXCHANGE/publish" -d "$body")
  echo "  -> $1  $out"
}

# event <routing-key> <eventType> <data-json> [eventId]
event() {
  id="${4:-$(uuid)}"
  publish "$1" "{\"eventId\":\"$id\",\"eventType\":\"$2\",\"timestamp\":\"$(now)\",\"data\":$3}"
}

PATIENT='"patientId":"p-1","patientName":"Pema Dorji","patientEmail":"pema@example.com"'
APPT='"appointmentId":"a-1","doctorId":"d-1","doctorName":"Karma Wangchuk","doctorEmail":"karma@example.com","date":"2026-10-25","time":"10:30"'

case "${1:-}" in
  happy)
    echo "1. PatientRegistered";        event patient.registered PatientRegistered "{$PATIENT}"
    echo "2. AppointmentRequested";     event appointment.requested AppointmentRequested "{$PATIENT,$APPT}"
    echo "3. AppointmentConfirmed";     event appointment.confirmed AppointmentConfirmed "{$PATIENT,$APPT}"
    echo "4. AppointmentCompleted";     event appointment.completed AppointmentCompleted "{$PATIENT,$APPT,\"diagnosis\":\"Influenza\",\"prescription\":\"Rest and fluids\"}"
    echo "5. MedicalRecordCreated";     event medical_record.created MedicalRecordCreated "{\"recordId\":\"r-1\",$PATIENT,\"appointmentId\":\"a-1\",\"doctorName\":\"Karma Wangchuk\"}"
    ;;
  duplicate)
    id=$(uuid)
    echo "Publishing eventId $id three times"
    for _ in 1 2 3; do event patient.registered PatientRegistered "{$PATIENT}" "$id"; done
    echo "Expect exactly ONE welcome notification for this event."
    ;;
  poison)
    echo "Malformed JSON";   publish appointment.confirmed '{this is not json'
    echo "Unknown type";     event appointment.confirmed SomethingUnknown '{}'
    echo "Missing fields";   event patient.registered PatientRegistered '{"patientId":"p-9"}'
    echo "Expect all three in notification-service.dlq (management UI -> Queues)."
    ;;
  show)
    echo "== notifications"; curl -s "$API/api/notifications?limit=20"; echo
    echo "== processed events"; curl -s "$API/api/processed-events?limit=20"; echo
    echo "== queues"; curl -s -u "$AUTH" "$RABBIT/api/queues" | tr '{' '\n' | grep -o '"name":"notification-service[^"]*"\|"messages":[0-9]*' | paste -sd' ' - | sed 's/"name"/\n"name"/g'
    echo
    ;;
  *) echo "usage: $0 {happy|duplicate|poison|show}"; exit 1 ;;
esac
