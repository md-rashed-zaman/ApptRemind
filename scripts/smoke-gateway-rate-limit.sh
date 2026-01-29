#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
LIMIT="${LIMIT:-60}"
N="${N:-65}"

# Use a synthetic X-Forwarded-For so the test doesn't interfere with your real IP's limiter bucket.
RAND="${RAND:-$RANDOM}"
IP="203.0.113.$((RAND % 250 + 1))"

echo "Running gateway rate limit smoke test"
echo "BASE_URL=$BASE_URL"
echo "IP=$IP"
echo "N=$N (limit expected around $LIMIT/min)"

ok=0
too_many=0
other=0

for _ in $(seq 1 "$N"); do
  code="$(curl -sS -o /dev/null -w "%{http_code}" \
    -H "X-Forwarded-For: $IP" \
    "$BASE_URL/openapi" || true)"
  if [[ "$code" == "200" ]]; then
    ok=$((ok + 1))
  elif [[ "$code" == "429" ]]; then
    too_many=$((too_many + 1))
  else
    other=$((other + 1))
  fi
done

echo "200=$ok 429=$too_many other=$other"

if [[ "$too_many" -lt 1 ]]; then
  echo "FAILED: expected at least one 429 response after $N requests"
  exit 1
fi

echo "OK: rate limiting enforced (got $too_many x 429)"

