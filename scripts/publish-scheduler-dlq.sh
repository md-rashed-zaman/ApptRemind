#!/usr/bin/env bash
set -euo pipefail

TOPIC="${KAFKA_TEST_TOPIC:-scheduler.reminder.dlq.v1}"
BROKER="${KAFKA_BROKER:-kafka:9092}"
EVENT_ID="${EVENT_ID:-$(python3 -c 'import uuid; print(uuid.uuid4())')}"
NOW="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

echo "Publishing to $TOPIC via $BROKER (event_id=$EVENT_ID)"

CONTAINER_NAME="${KAFKA_TOOLS_CONTAINER:-apptremind-kafka-tools-1}"

docker exec -i "$CONTAINER_NAME" kcat -b "$BROKER" -t "$TOPIC" \
  -H "event_id=$EVENT_ID" \
  -H "event_type=$TOPIC" \
  -k "demo-key" \
  -P <<EOF
{"appointment_id":"${APPOINTMENT_ID:-$(python3 -c 'import uuid; print(uuid.uuid4())')}","business_id":"${BUSINESS_ID:-$(python3 -c 'import uuid; print(uuid.uuid4())')}","channel":"${CHANNEL:-email}","recipient":"${RECIPIENT:-demo@example.com}","remind_at":"$NOW","template_data":{"business_name":"Demo Salon","service":"Haircut","time":"$NOW"},"error_reason":"max attempts reached","failed_at":"$NOW"}
EOF

echo "Published."
