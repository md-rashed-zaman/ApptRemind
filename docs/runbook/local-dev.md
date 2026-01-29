# Local Dev Runbook

## Start everything
```bash
cd deploy/compose
cp .env.example .env
docker compose up --build
```
Security notes (secrets, CORS, audit logging):
- `docs/security.md`
K8s skeleton (optional):
- `deploy/k8s/README.md`

## Troubleshooting Kafka
If Kafka fails to start with a `clusterId` mismatch or `/brokers/ids/1` error, reset Kafka state:
```bash
docker compose down
docker volume rm apptremind_kafka-data
docker compose up -d
```

## Health checks
- Gateway: `curl -sS localhost:8080/healthz`
- Booking: `docker exec -it apptremind-booking-service-1 wget -qO- http://localhost:8083/healthz`

Every request returns an `X-Request-Id` header; you can also provide your own:
```bash
curl -i -H 'X-Request-Id: demo-123' localhost:8080/healthz
```

## Gateway OpenAPI
```bash
curl -sS localhost:8080/openapi
```
Swagger UI (compose): `http://localhost:8088/docs`
If you see a CORS error, ensure `CORS_ALLOWED_ORIGINS` includes `http://localhost:8088` and restart the gateway.

## Tracing (OpenTelemetry + Jaeger)
- Jaeger UI: `http://localhost:16686`
- OTLP gRPC endpoint (compose): `jaeger:4317`

Env toggles (already set in compose):
- `OTEL_ENABLED` (set `false` to disable exporting)
- `OTEL_EXPORTER_OTLP_ENDPOINT` (default: `jaeger:4317`)
- `OTEL_SAMPLING_RATIO` (default: `1`)

Quick verify:
```bash
./scripts/smoke-booking-reminder.sh
```
Then in Jaeger search for service `gateway-service` (or `booking-service`) and you should see traces spanning HTTP + Kafka consumers.

## Auth + JWT (dev)
The auth-service issues HS256 JWTs using `JWT_SECRET` (shared with the gateway).
Use `/api/v1/auth/login` to get a token and call protected routes.
Example:
```bash
TOKEN="$(curl -sS -X POST localhost:8080/api/v1/auth/login -d '{"email":"demo@example.com","password":"demo"}' | jq -r .access_token)"
curl -sS -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/appointments
```
Check current user:
```bash
curl -sS -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/auth/me
```
Refresh access tokens (rotates refresh token):
```bash
REFRESH_TOKEN="$(curl -sS -X POST localhost:8080/api/v1/auth/login -d '{"email":"demo@example.com","password":"demo"}' | jq -r .refresh_token)"
curl -sS -X POST localhost:8080/api/v1/auth/refresh -d "{\"refresh_token\":\"$REFRESH_TOKEN\"}" | jq -r .access_token
```
Logout (revoke refresh token):
```bash
curl -sS -X POST localhost:8080/api/v1/auth/logout -d "{\"refresh_token\":\"$REFRESH_TOKEN\"}" -i
```
Auth smoke test (register -> refresh -> logout -> revoke check):
```bash
./scripts/smoke-auth.sh
```
Business/billing routes require role `owner` or `admin`.

## Gateway limits
Configured via env:
- `RATE_LIMIT_PER_MINUTE`
- `REQUEST_BODY_LIMIT_BYTES`
- `REQUEST_TIMEOUT_SECONDS`

## Gateway CORS
Configured via env (comma-separated):
- `CORS_ALLOWED_ORIGINS` (empty disables CORS)
- `CORS_ALLOWED_METHODS`
- `CORS_ALLOWED_HEADERS`
- `CORS_ALLOW_CREDENTIALS`
- `CORS_MAX_AGE_SECONDS`

## RS256 + JWKS (production-style)
Generate keys:
```bash
./scripts/gen-jwt-keys.sh
```
Then start compose with:
```bash
export JWT_PRIVATE_KEY_PEM="$(cat /tmp/apptremind-jwt/jwt_private.pem)"
export JWKS_URL="http://auth-service:8081/.well-known/jwks.json"
docker compose -f deploy/compose/docker-compose.yml up --build
```
Gateway will verify RS256 tokens via JWKS when `JWKS_URL` is set.
For key rotation, provide multiple keys:
```bash
export JWT_PRIVATE_KEYS_PEM="$(cat /tmp/apptremind-jwt/jwt_private.pem)$(cat /tmp/apptremind-jwt/jwt_private.pem)"
export JWT_ACTIVE_KID="your-active-kid"
export JWT_ROTATE_KEY="rotate-secret"
```
Rotate active key (auth-service):
```bash
curl -X POST localhost:8080/api/v1/auth/rotate \
  -H "X-Rotate-Key: rotate-secret" \
  -d '{"active_kid":"your-active-kid"}'
```
Or use the helper:
```bash
ROTATE_KEY=rotate-secret ./scripts/rotate-jwt-key.sh
```
Rotation events are recorded in `auth_db.audit_events` with event_type `jwt.rotate`.
Rotation events are also emitted to Kafka topic `auth.audit.v1` via the auth-service outbox.
List recent audit events:
```bash
curl -sS "localhost:8080/api/v1/auth/audit?limit=20" -H "X-Rotate-Key: rotate-secret"
```

## Protobuf generation + tests
```bash
make proto
make test-proto
```

## Migrations (booking-service)
All migrate scripts now share a standard runner (`scripts/migrate-service.sh`).
```bash
make migrate-booking
```
All `make migrate-*` targets will run migrations inside the Postgres container when host `psql` isn't available.

## Migrations (auth-service)
```bash
make migrate-auth
```

## Migrations (business-service)
```bash
make migrate-business
```

## Business setup (profile/services/staff)
Register an owner and get the business id:
```bash
EMAIL="owner-$(date +%s)@example.com"
REG_JSON="$(curl -sS -X POST localhost:8080/api/v1/auth/register -H 'Content-Type: application/json' -d "{\"email\":\"$EMAIL\",\"password\":\"pass123\",\"business_name\":\"Demo Biz\"}")"
TOKEN="$(echo "$REG_JSON" | jq -r .access_token)"
BUSINESS_ID="$(curl -sS localhost:8080/api/v1/auth/me -H "Authorization: Bearer $TOKEN" | jq -r .business_id)"
```

Update business timezone (used for availability):
```bash
curl -sS -X PUT localhost:8080/api/v1/business/profile \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Demo Biz","timezone":"America/New_York","reminder_offsets_minutes":[1440,60]}' -i
```

Create a service (duration drives slot length):
```bash
SERVICE_ID="$(curl -sS -X POST localhost:8080/api/v1/business/services \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Consult","duration_minutes":25,"price":10,"description":"demo"}' | jq -r .id)"
```

Create a staff member (defaults to Mon-Fri 09:00-17:00, weekends closed):
```bash
STAFF_ID="$(curl -sS -X POST localhost:8080/api/v1/business/staff \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice"}' | jq -r .id)"
```

Update staff working hours (example: open Sunday 10:00-14:00; weekday uses Go numbering: 0=Sun..6=Sat):
```bash
curl -sS -X PUT "localhost:8080/api/v1/business/staff/working-hours?staff_id=$STAFF_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"weekday":0,"is_working":true,"start_minute":600,"end_minute":840}' -i
```

List staff working hours:
```bash
curl -sS "localhost:8080/api/v1/business/staff/working-hours?staff_id=$STAFF_ID" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Create staff time-off (blackout) (RFC3339 UTC timestamps):
```bash
TIMEOFF_ID="$(curl -sS -X POST "localhost:8080/api/v1/business/staff/time-off?staff_id=$STAFF_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"start_time":"2026-02-01T15:00:00Z","end_time":"2026-02-01T16:00:00Z","reason":"lunch"}' | jq -r .id)"
```

List staff time-off (overlapping range):
```bash
curl -sS "localhost:8080/api/v1/business/staff/time-off?staff_id=$STAFF_ID&from=2026-02-01T00:00:00Z&to=2026-02-02T00:00:00Z" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Delete staff time-off:
```bash
curl -sS -X DELETE "localhost:8080/api/v1/business/staff/time-off?id=$TIMEOFF_ID" \
  -H "Authorization: Bearer $TOKEN" -i
```

Smoke test (business setup + weekday/weekend slots):
```bash
./scripts/smoke-availability.sh
```

## Booking idempotency + cancellation
List available slots (public):
```bash
curl -sS "localhost:8080/api/v1/public/slots?business_id=$BUSINESS_ID&staff_id=$STAFF_ID&service_id=$SERVICE_ID&date=2026-01-28" | jq '.[0:10]'
```

Book with an idempotency key (retry-safe):
```bash
IDEMP_KEY="demo-book-1"
curl -sS -X POST localhost:8080/api/v1/public/book \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $IDEMP_KEY" \
  -d '{
    "business_id":"'"$BUSINESS_ID"'",
    "service_id":"'"$SERVICE_ID"'",
    "staff_id":"'"$STAFF_ID"'",
    "customer_name":"Demo Customer",
    "customer_email":"demo@example.com",
    "customer_phone":"+15550000000",
    "start_time":"2026-01-28T10:00:00Z",
    "end_time":"2026-01-28T10:30:00Z"
  }'
```
Repeat the same request with the same `Idempotency-Key` to get the same `appointment_id`.

Cancel through the authenticated route:
```bash
TOKEN="$(curl -sS -X POST localhost:8080/api/v1/auth/register -d '{"email":"cancel-owner@example.com","password":"pass123","business_name":"Cancel Owner"}' | jq -r .access_token)"
curl -sS -X POST localhost:8080/api/v1/appointments/cancel \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"business_id":"00000000-0000-0000-0000-000000000001","appointment_id":"PUT_APPOINTMENT_ID_HERE","reason":"customer_request"}'
```
List appointments for the authenticated business:
```bash
curl -sS localhost:8080/api/v1/appointments -H "Authorization: Bearer $TOKEN" | jq '.[0:5]'
```

## Outbox publisher
The booking-service outbox publisher uses Kafka brokers from `KAFKA_BROKERS`.
By default in compose this resolves to `kafka:9092`.
Reminder offsets are configured by `REMINDER_OFFSETS_MINUTES` (comma-separated).
When building with `-tags protogen`, booking-service will fetch reminder offsets per business via gRPC from business-service (`BUSINESS_GRPC_ADDR`).

## Inbox consumer (real contract stub)
Booking-service runs consumers for:
- `billing.subscription.activated.v1`
- `billing.subscription.canceled.v1` (when configured)

It records dedupe entries in `inbox_events` and maintains a local entitlements cache in `business_entitlements`, used to enforce the monthly booking cap.

## Publish a test event
```bash
./scripts/publish-test-event.sh
```

## Publish a reminder event (notification-service)
```bash
./scripts/publish-reminder-event.sh
```

## SMS provider (webhook)
Notification-service can send SMS via a generic webhook:
```bash
export SMS_PROVIDER=webhook
export SMS_WEBHOOK_URL="https://example-webhook.local/sms"
export SMS_WEBHOOK_TOKEN="optional-token"
docker compose -f deploy/compose/docker-compose.yml up -d --build notification-service
```
When a reminder event uses channel `sms`, the webhook receives `{"to":"...","body":"..."}`.

## Publish a reminder request (scheduler-service)
```bash
./scripts/publish-reminder-requested.sh
```

Smoke test (scheduler -> notification -> Mailpit + analytics metrics):
```bash
make migrate-scheduler
make migrate-notification
make migrate-analytics
./scripts/smoke-notification-pipeline.sh
```

Smoke test (full flow: booking -> reminder requested -> scheduler -> notification -> Mailpit):
```bash
./scripts/smoke-booking-reminder.sh
```

Smoke test (guardrail: booking outside availability is rejected with 422):
```bash
./scripts/smoke-booking-outside-availability.sh
```

Smoke test (guardrail: overlapping time-off is rejected with 409):
```bash
./scripts/smoke-timeoff-overlap.sh
```

Smoke test (billing entitlements: second booking rejected with 402):
```bash
make migrate-billing
./scripts/smoke-billing-enforcement.sh
```

Smoke test (gateway rate limit: expect 429s after enough requests):
```bash
./scripts/smoke-gateway-rate-limit.sh
```

## Stripe webhook (production-style)
This repo exposes a Stripe webhook endpoint that is *not* JWT-protected (Stripe can't send a JWT); it is protected by signature verification:
- Endpoint: `POST /api/v1/billing/webhooks/stripe`
- Env required on `billing-service`: `STRIPE_WEBHOOK_SECRET`
- Optional: `STRIPE_WEBHOOK_TOLERANCE_SECONDS` (default: 300)

Local dev with Stripe CLI (free):
```bash
# In one terminal
stripe listen --forward-to localhost:8080/api/v1/billing/webhooks/stripe
```
Stripe CLI prints a signing secret like `whsec_...`. Export it and restart compose:
```bash
export STRIPE_WEBHOOK_SECRET="whsec_..."
docker compose -f deploy/compose/docker-compose.yml up -d --build billing-service gateway-service
```
Then trigger a test event:
```bash
stripe trigger checkout.session.completed
```
Note: for our MVP, the webhook expects `business_id` and `tier` in Stripe object metadata (we'll wire checkout to set these next).

## Stripe checkout (production-style)
To enable real checkout session creation (Stripe test mode is free):
- `STRIPE_SECRET_KEY` (sk_test_...)
- `STRIPE_PRICE_STARTER` and/or `STRIPE_PRICE_PRO` (Price IDs from Stripe dashboard)
- Either send `success_url`/`cancel_url` in the request body, or set defaults:
  - `CHECKOUT_SUCCESS_URL`, `CHECKOUT_CANCEL_URL`
In compose, defaults point to:
- `http://localhost:8080/billing/success?session_id={CHECKOUT_SESSION_ID}`
- `http://localhost:8080/billing/cancel?session_id={CHECKOUT_SESSION_ID}`

Example (requires owner/admin JWT via gateway):
```bash
curl -sS -X POST localhost:8080/api/v1/billing/checkout \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: checkout-1" \
  -d '{"tier":"starter","success_url":"http://localhost:8080/healthz","cancel_url":"http://localhost:8080/healthz"}' | jq .
```
After Stripe redirects back, the gateway serves:
- `/billing/success?session_id=...`
- `/billing/cancel?session_id=...`
These pages poll `/api/v1/billing/checkout/session?session_id=...` (public) to show status.
They also POST an acknowledgement to `/api/v1/billing/checkout/session/ack` using a per-session `state` token embedded into the return URL (prevents tampering).

Cancel subscription (owner/admin):
```bash
curl -sS -X POST localhost:8080/api/v1/billing/subscription/cancel \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: cancel-1" \
  -d '{}' | jq .
```

Billing audit events:
```bash
psql "postgres://billing_user:billing_password@localhost:5432/billing_db?sslmode=disable" \
  -c "SELECT event_type, actor_type, actor_id, business_id, created_at FROM audit_events ORDER BY id DESC LIMIT 20;"
```

Smoke (signed webhook via `stripe-go` helper):
```bash
export STRIPE_WEBHOOK_SECRET="whsec_test"
docker compose -f deploy/compose/docker-compose.yml up -d --build billing-service gateway-service
./scripts/smoke-billing-stripe-webhook.sh
```

## Billing - Stripe reconciliation (optional hardening)
If webhooks are missed (deploy downtime, network issues), billing-service can periodically fetch Stripe subscription state and self-heal local DB + emit entitlement events when the effective tier/status changes.

Enable in local dev (requires a real Stripe test API key):
```bash
export STRIPE_SECRET_KEY="sk_test_..."
export BILLING_STRIPE_RECONCILE_ENABLED="true"
docker compose -f deploy/compose/docker-compose.yml up -d --build billing-service
```

## Publish a scheduler DLQ event
```bash
./scripts/publish-scheduler-dlq.sh
```

## Observe notification metrics
After publishing a reminder event, you can verify analytics ingestion:
```bash
psql "postgres://analytics_user:analytics_password@localhost:5432/analytics_db?sslmode=disable" -c "SELECT * FROM notification_metrics ORDER BY id DESC LIMIT 5;"
```

## Kafka topics
```bash
make bootstrap-topics
```

## Kafka consumer lag (basic)
```bash
./scripts/consumer-lag.sh
```

## Contract checks (local)
```bash
./scripts/check-openapi-sync.sh
./scripts/check-proto-clean.sh
./scripts/check-events-consistency.sh
./scripts/check-event-schemas.sh
```

## Migrations (notification-service)
```bash
make migrate-notification
```

## Migrations (analytics-service)
```bash
make migrate-analytics
```

## Analytics rebuild (appointments)
Recompute daily appointment aggregates from stored booking events:
```bash
./scripts/rebuild-analytics-appointments.sh
```
Quick sanity query:
```bash
psql "postgres://analytics_user:analytics_password@localhost:5432/analytics_db?sslmode=disable" \
  -c "SELECT business_id, day, booked_count, canceled_count FROM daily_appointment_metrics ORDER BY day DESC LIMIT 10;"
```

## Analytics rebuild (notifications)
Recompute daily notification aggregates from stored notification metrics:
```bash
./scripts/rebuild-analytics-notifications.sh
```
Quick sanity query:
```bash
psql "postgres://analytics_user:analytics_password@localhost:5432/analytics_db?sslmode=disable" \
  -c "SELECT business_id, day, channel, sent_count, failed_count FROM daily_notification_metrics ORDER BY day DESC LIMIT 10;"
```

## Migrations (scheduler-service)
```bash
make migrate-scheduler
```

## Notification consumer
Notification-service consumes `scheduler.reminder.due.v1` and stores rows in `notifications`.
It also writes an outbox event `notification.sent.v1` for each received reminder.

## Scheduler retry/backoff
Scheduler retries failed enqueue operations with `SCHEDULER_BACKOFF_SECONDS`. After max attempts it emits `scheduler.reminder.dlq.v1`.
Set `NOTIFICATION_FAIL_SUFFIX` (e.g. `@fail.local`) to simulate failures and emit `notification.failed.v1`.

## Analytics consumer
Analytics-service consumes `notification.sent.v1` and `notification.failed.v1`, and writes to `notification_metrics` with `status=sent|failed`.
It also consumes `scheduler.reminder.dlq.v1` and writes to `scheduler_dlq_events`.
It consumes `auth.audit.v1` and writes to `security_audit_events`.
Inspect security audit events:
```bash
./scripts/query-security-audit.sh 10
```
