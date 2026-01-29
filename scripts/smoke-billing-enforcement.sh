#!/usr/bin/env bash
set -euo pipefail

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required"
  exit 1
fi

BASE_URL="${BASE_URL:-http://localhost:8080}"

echo "Running billing -> booking entitlements enforcement smoke test"

wait_for() {
  local name="$1"
  local seconds="$2"
  shift 2
  for _ in $(seq 1 "$seconds"); do
    if "$@" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $name" >&2
  return 1
}

wait_for "gateway /healthz" 30 curl -fsS "$BASE_URL/healthz"

echo "Ensuring migrations..."
make migrate-auth >/dev/null
make migrate-business >/dev/null
make migrate-booking >/dev/null
make migrate-billing >/dev/null

EMAIL="owner-billing-$(date +%s)@example.com"
REG_JSON="$(
  curl -sS -X POST "$BASE_URL/api/v1/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$EMAIL\",\"password\":\"pass123\",\"business_name\":\"Billing Smoke Biz\"}"
)"
TOKEN="$(echo "$REG_JSON" | jq -r .access_token)"
BUSINESS_ID="$(curl -sS "$BASE_URL/api/v1/auth/me" -H "Authorization: Bearer $TOKEN" | jq -r .business_id)"

echo "business_id=$BUSINESS_ID"

curl -sS -X PUT "$BASE_URL/api/v1/business/profile" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"timezone":"UTC","reminder_offsets_minutes":[1440]}' >/dev/null

SERVICE_ID="$(
  curl -sS -X POST "$BASE_URL/api/v1/business/services" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"name":"Consult","duration_minutes":30,"price":10,"description":"smoke"}' | jq -r .id
)"

STAFF_ID="$(
  curl -sS -X POST "$BASE_URL/api/v1/business/staff" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"name":"Alice"}' | jq -r .id
)"

# Open all day today (UTC).
WEEKDAY="$(date -u +%w)" # 0=Sun..6=Sat
curl -sS -X PUT "$BASE_URL/api/v1/business/staff/working-hours?staff_id=$STAFF_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"weekday\":$WEEKDAY,\"is_working\":true,\"start_minute\":0,\"end_minute\":1440}" >/dev/null

# Activate starter tier (max 1 monthly appointment) through local webhook.
EVENT_ID="$(uuidgen 2>/dev/null || cat /proc/sys/kernel/random/uuid)"
OCCURRED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

curl -sS -X POST "$BASE_URL/api/v1/billing/webhooks/local" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"event_id\":\"$EVENT_ID\",
    \"type\":\"subscription.activated\",
    \"business_id\":\"$BUSINESS_ID\",
    \"tier\":\"starter\",
    \"occurred_at\":\"$OCCURRED_AT\"
  }" >/dev/null

subscription_is_starter() {
  curl -fsS "$BASE_URL/api/v1/billing/subscription?business_id=$BUSINESS_ID" -H "Authorization: Bearer $TOKEN" | jq -e '.tier == "starter"' >/dev/null
}
wait_for "billing subscription visible" 15 subscription_is_starter

# Allow time for billing outbox -> Kafka -> booking consumer to update the local entitlements cache.
sleep 5

RECIPIENT="customer-billing-$(date +%s)@example.com"

START_1="$(date -u -d '+70 seconds' +%Y-%m-%dT%H:%M:%SZ)"
END_1="$(date -u -d '+2500 seconds' +%Y-%m-%dT%H:%M:%SZ)"
START_2="$(date -u -d '+4000 seconds' +%Y-%m-%dT%H:%M:%SZ)"
END_2="$(date -u -d '+5800 seconds' +%Y-%m-%dT%H:%M:%SZ)"

code1="$(
  curl -sS -o /tmp/apptremind-book1.json -w '%{http_code}' -X POST "$BASE_URL/api/v1/public/book" \
    -H "Content-Type: application/json" \
    -d "{
      \"business_id\":\"$BUSINESS_ID\",
      \"service_id\":\"$SERVICE_ID\",
      \"staff_id\":\"$STAFF_ID\",
      \"customer_name\":\"Smoke Customer\",
      \"customer_email\":\"$RECIPIENT\",
      \"customer_phone\":\"\",
      \"start_time\":\"$START_1\",
      \"end_time\":\"$END_1\"
    }"
)"
if [[ "$code1" != "201" ]]; then
  echo "expected first booking to succeed (201), got $code1"
  cat /tmp/apptremind-book1.json
  exit 1
fi

code2="$(
  curl -sS -o /tmp/apptremind-book2.json -w '%{http_code}' -X POST "$BASE_URL/api/v1/public/book" \
    -H "Content-Type: application/json" \
    -d "{
      \"business_id\":\"$BUSINESS_ID\",
      \"service_id\":\"$SERVICE_ID\",
      \"staff_id\":\"$STAFF_ID\",
      \"customer_name\":\"Smoke Customer 2\",
      \"customer_email\":\"$RECIPIENT\",
      \"customer_phone\":\"\",
      \"start_time\":\"$START_2\",
      \"end_time\":\"$END_2\"
    }"
)"
if [[ "$code2" != "402" ]]; then
  echo "expected second booking to be rejected with 402, got $code2"
  cat /tmp/apptremind-book2.json
  exit 1
fi

echo "OK: entitlements enforced (second booking rejected with 402)"
