#!/usr/bin/env bash
set -euo pipefail

TOPIC="${KAFKA_TEST_TOPIC:-billing.subscription.activated.v1}"
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
{"business_id":"${BUSINESS_ID:-$(python3 -c 'import uuid; print(uuid.uuid4())')}","tier":"pro","activated_at":"$NOW","subscription_id":"sub-demo-123"}
EOF

echo "Published."
