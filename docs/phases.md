# Project Phases (Execution + Tracking)

This is the working checklist to build ApptRemind in production-style increments.
Each phase has deliverables, acceptance criteria, and the main commands to validate progress.

Conventions:
- "Done" means: code merged, CI green, docs updated, and basic local verification steps pass.
- Keep changes small: one phase can span multiple PRs, but keep PRs focused and testable.

## Progress Log (keep this updated)
- 2026-01-28: Added readiness checks for DB/Kafka and wired `/readyz` across services.
- 2026-01-28: Standardized migrations runner (`scripts/migrate-service.sh`) and refactored migrate scripts.
- 2026-01-28: Added SMS webhook provider for notifications and new auth/JWT/gateway authz tests; analytics rebuilds re-run.
- 2026-01-28: Added tagged Docker image build script and a minimal Kubernetes skeleton (gateway/auth) with docs.
- 2026-01-28: Re-ran full smoke suite (auth, availability, timeoff, booking reminder, notification pipeline, billing enforcement, Stripe webhook, gateway rate limit) with compose running.
- 2026-01-28: Added billing audit_events table and emit audit records for checkout, cancellations, and provider webhooks.
- 2026-01-28: Documented production security posture (secrets, CORS, audit logging) in docs/security.md.
- 2026-01-28: Added gateway CORS middleware with env-configured policy and docker-compose defaults.
- 2026-01-28: Added JSON schema validation for event payloads with CI + runbook wiring.
- 2026-01-28: Added contract checks (OpenAPI sync, proto clean, event catalog match) and a Kafka consumer lag script; CI updated to run checks.
- 2026-01-28: Added OpenAPI examples for auth + billing endpoints in both gateway specs.
- 2026-01-28: Added OpenAPI examples for business/booking endpoints (requests + responses) in both gateway specs.
- 2026-01-28: OpenAPI aligned with business/booking responses (service/staff schemas, id responses) and public slots fallback params documented.
- 2026-01-28: End-to-end smoke suite re-run; analytics rebuilds for appointments and notifications now return non-empty aggregates.
- 2026-01-28: Notification events now carry `business_id`, analytics stores it, and daily notification aggregates rebuild successfully.
- 2026-01-28: Full smoke suite executed (auth, availability, timeoff, booking reminder, notification pipeline, billing enforcement, Stripe webhook, gateway rate limit) and analytics rebuild verified with non-empty aggregates.
- 2026-01-28: Notification aggregates added (daily sent/failed by business + channel) with rebuild script.
- 2026-01-27: Analytics now tracks booking events and maintains daily appointment aggregates (`daily_appointment_metrics`) with a replay script (`scripts/rebuild-analytics-appointments.sh`).
- 2026-01-27: Added gateway rate limiting smoke test `./scripts/smoke-gateway-rate-limit.sh` (uses synthetic `X-Forwarded-For` to avoid interfering with your real IP bucket).
- 2026-01-27: Gateway rate limiting upgraded to optionally use Redis (distributed fixed-window) via `REDIS_ADDR`, keeping in-memory limiter as fallback.
- 2026-01-27: Verified billing smoke scripts locally: `./scripts/smoke-billing-enforcement.sh` and `./scripts/smoke-billing-stripe-webhook.sh` (after setting `STRIPE_WEBHOOK_SECRET=whsec_test` and recreating `billing-service` + `gateway-service`).
- 2026-01-27: Refactored subscription state transitions into `billing-service/internal/subscriptions` and added optional Stripe reconciliation loop (Postgres advisory lock) to self-heal if webhooks are missed.
- 2026-01-27: Implemented Stripe checkout session creation on billing-service (`POST /api/v1/billing/checkout`) with Stripe idempotency support and metadata wiring (`business_id`, `tier`).
- 2026-01-27: Added checkout return flow: gateway pages `/billing/success` + `/billing/cancel`, persisted Stripe checkout sessions in billing DB, and added public status endpoint `/api/v1/billing/checkout/session`.
- 2026-01-27: Hardened checkout return flow with per-session `state` token + public ack endpoint `/api/v1/billing/checkout/session/ack` and persisted `return_seen_at`.
- 2026-01-27: Added handling for `checkout.session.expired` to mark checkout sessions as `expired` in billing DB.
- 2026-01-27: Added Stripe webhook tolerance config (`STRIPE_WEBHOOK_TOLERANCE_SECONDS`) + structured audit logs for provider events (stripe/local).
- 2026-01-27: Added authenticated Stripe subscription cancellation endpoint (`POST /api/v1/billing/subscription/cancel`) with idempotency and event emission (`billing.subscription.canceled.v1`).
- 2026-01-27: Started Billing MVP: billing-service DB + outbox events, local webhook simulator, booking-service now enforces monthly appointment cap using cached entitlements.
- 2026-01-27: Added Stripe-style webhook endpoint (`/api/v1/billing/webhooks/stripe`) with signature verification + idempotent provider event recording (pre-checkout wiring).
- 2026-01-27: Added OpenTelemetry (`libs/otel`) + service HTTP/gRPC wiring + Kafka header propagation; outbox/scheduler now persist `traceparent`/`tracestate` to keep traces connected across async hops.
- 2026-01-27: Centralized Kafka header parsing + broker parsing in `libs/kafkax`; updated scheduler/notification/analytics consumers to use it.
- 2026-01-27: Added `libs/grpcx` (request-id propagation + dial helper) and started using it in business-service gRPC server and booking-service gRPC clients.
- 2026-01-27: Added DB-level guardrail for staff time-off: no overlapping blackouts per staff (btree_gist + exclusion constraint); added `scripts/smoke-timeoff-overlap.sh`.
- 2026-01-27: Booking-service now enforces availability on create (must fit within business-service availability windows); added `scripts/smoke-booking-outside-availability.sh`.
- 2026-01-27: Business reminder policy is now DB-backed (`reminder_offsets_minutes` on business profile) and served via business-service gRPC `GetBusinessProfile`.
- 2026-01-27: Added staff time-off (blackouts) to business-service + gRPC availability now supports multiple windows (`windows_utc`) with time-off subtracted.
- 2026-01-27: Business-service now supports staff working-hours CRUD (`/api/v1/business/staff/working-hours`), and availability smoke test validates weekend opening.
- 2026-01-27: Added end-to-end smoke test `scripts/smoke-booking-reminder.sh` (public booking -> reminder -> scheduler -> notification -> Mailpit).
- 2026-01-27: Gateway authz fixed for `/api/v1/business` and `/api/v1/billing` (auth now runs before role checks).
- 2026-01-27: Docker builds stabilized: Go builds use `vendor/` (no module downloads in Docker) and service Dockerfile copies `protos/`.
- 2026-01-27: All `scripts/migrate-*.sh` now fall back to running `psql` inside the Postgres container (no host `psql` required).
- 2026-01-27: Added `scripts/smoke-availability.sh` to validate business setup + slots end-to-end via gateway.
- 2026-01-27: Notification-service now sends real SMTP email (Mailpit in compose) and added `scripts/smoke-notification-pipeline.sh`.
- 2026-01-27: Business DB migrations now run without host `psql` (script falls back to `docker compose exec postgres psql`).
- 2026-01-27: Fixed Postgres init to enable `uuid-ossp` extension for `business_db`; business-service schema created successfully.
- 2026-01-27: Business-service CRUD scaffolding added (profile, services, staff + default working hours).
- 2026-01-27: Business-service gRPC added for availability config (`GetAvailabilityConfig`) and booking-service wired to use it for `/public/slots` when available.
- 2026-01-27: Public slots endpoint implemented (`GET /api/v1/public/slots`) with simple availability computation.
- 2026-01-27: Booking list endpoint (`GET /api/v1/appointments`) implemented and validated via gateway.
- 2026-01-27: Booking idempotency + cancellation verified end-to-end; cancellation event observed on Kafka.
- 2026-01-27: Booking idempotency keys + cancellation endpoint implemented; migrations applied locally.
- 2026-01-27: Auth register now writes `auth.user.created.v1` via outbox in the same DB transaction.
- 2026-01-27: Kafka tooling stabilized (`kafka-tools` now stays up) and `auth.user.created.v1` verified via kcat.
- 2026-01-27: Auth refresh/logout flow verified end-to-end via gateway; added `scripts/smoke-auth.sh`.
- 2026-01-27: Compose stack boots locally with Postgres + Kafka + services after Dockerfile/compose fixes.
- 2026-01-27: Postgres init script fixed to create roles/DBs reliably.

## Phase 0 - Baseline (Repo + Local Infra)
Status: DONE

Deliverables:
- Repo layout (`services/`, `libs/`, `deploy/compose/`, `docs/`)
- Local Docker Compose: Postgres (multi-DB), Kafka, Mailpit, Jaeger
- Service skeletons with `/healthz` and `/readyz`
- Initial public OpenAPI + internal proto stubs
- CI: generates protos and runs tests

Acceptance:
- `docker compose up --build` works from `deploy/compose/`
- `make proto` and `make test-proto` pass
- `docs/system-design.md` and ADRs exist and reflect the architecture decisions

Verify:
- `make proto`
- `make test-proto`

## Phase 1 - Shared Platform Libraries (Production Defaults)
Status: DONE

Goal: reduce duplication and enforce standards across services.

Deliverables:
- ✅ `libs/config`: env parsing helpers in use across services
- ✅ `libs/httpx`: request-id + access logs + timeouts/body limits/rate limits
- ✅ `libs/grpcx`: request-id interceptors + dial helper (used by booking-service + business-service)
- ✅ `libs/db`: pgx pool wrapper in use
- ✅ `libs/kafkax`: shared Kafka helpers (brokers parsing + event meta extraction)
- ✅ `libs/otel`: OpenTelemetry wiring (HTTP + gRPC + Kafka propagation; exports to local Jaeger via OTLP)

Acceptance:
- Every service uses shared logging + config patterns (no hand-rolled env parsing)
- `/readyz` checks dependencies (DB/Kafka) where applicable
- Example service can publish/consume a test Kafka message locally

Verify:
- `make test`
- Optional: run compose and hit each `/healthz` + `/readyz`

## Phase 2 - Data Layer Patterns (Migrations + Outbox/Inbox)
Status: DONE

Goal: implement production-grade reliability primitives before business logic.

Deliverables:
- Per-service migrations framework (Go migrate/Goose/Atlas; pick one and standardize)
- Standard tables (per service where relevant):
  - `outbox_events` (publisher reads and publishes to Kafka)
  - `inbox_events` (consumer dedupe/idempotency)
- Outbox publisher worker template (reused by services that publish domain events)
- Consumer template with DLQ routing + idempotency

Acceptance:
- Booking-service can write a DB row + outbox event in one tx
- Publisher publishes to Kafka and marks outbox row as published
- Consumer processes at-least-once safely (duplicate event doesn't duplicate side effects)
- Status notes:
  - ✅ Outbox/inbox tables + workers exist across booking/scheduler/notification/analytics/auth
  - ✅ Local Kafka topics bootstrap via compose `kafka-init`
  - ✅ Standardized migrations runner in `scripts/migrate-service.sh` used by all migrate scripts

Verify:
- Integration test using docker compose (or testcontainers) proves outbox->kafka->consumer flow
- Practical local checks:
  - `docker compose -f deploy/compose/docker-compose.yml up -d`
  - `./scripts/publish-reminder-requested.sh`
  - `docker compose -f deploy/compose/docker-compose.yml logs -f scheduler-service notification-service analytics-service`

## Phase 3 - Auth Service (Tenant + Identity Foundation)
Status: DONE

Deliverables:
- ✅ `auth-service`: register/login/me, password hashing, JWT issuance
- ✅ JWT claims include: `sub`, `business_id`, `role`, `exp`, `iat`
- ✅ JWT validation enforced at gateway for protected routes + basic RBAC for business/billing
- ✅ API: auth endpoints documented in `openapi/gateway.v1.yaml`
- ✅ Refresh/logout flow with rotation + revocation
- ✅ `auth.user.created.v1` event emitted via outbox on register

Acceptance:
- Can register an owner -> creates user and emits `auth.user.created.v1` (via outbox)
- Can login -> returns JWT
- Tests cover: hashing, JWT signing/validation, authz checks
- Status notes:
  - ✅ Register/login/me/refresh/logout all work through gateway locally
  - ✅ JWKS + rotation endpoint exists
  - ✅ Focused auth tests added (hashing + JWT + gateway authz)

Verify:
- `docker compose up --build`
- Register and login via curl/postman (document commands in `docs/runbook/local-dev.md`)
- Smoke test:
  - `./scripts/smoke-auth.sh`

## Phase 4 - Business Service (Master Data)
Status: DONE

Deliverables:
- `business-service`: business profile, services catalog, staff + working hours
- Internal gRPC read API for booking-service to fetch business timezone + reminder policy
- Optional events: `business.updated.v1`, `business.service.created.v1`, etc.
- Status notes:
  - ✅ gRPC reminder policy stub exists behind `protogen` build tag
  - ✅ Reminder policy is DB-backed on business profile (`reminder_offsets_minutes`) and returned via gRPC
  - ✅ Master data CRUD exists (profile/services/staff), with default working hours on staff creation
  - ✅ Working-hours CRUD exists; time-off (blackout) CRUD exists
  - ✅ Availability gRPC supports multiple windows per day (`windows_utc`) with time-off subtracted
  - ✅ OpenAPI + runbook examples aligned with final endpoint shapes

Acceptance:
- CRUD works with tenant scoping (`business_id`)
- gRPC `BusinessService.GetBusinessProfile` returns timezone + reminder offsets

Verify:
- Unit tests for validation and tenancy checks
- One integration test for gRPC method

## Phase 5 - Booking Service (Correctness First)
Status: DONE

Deliverables:
- Availability computation + booking creation + cancellation
- DB-level prevention of double booking (exclusion constraint)
- Public booking endpoint idempotency (Idempotency-Key header or request field)
- Events:
  - `booking.appointment.booked.v1`
  - `booking.appointment.cancelled.v1`
  - `booking.reminder.requested.v1` (one per reminder time)
- Status notes:
  - ✅ Exclusion constraint + outbox events for booked/reminder requested
  - ✅ Public booking idempotency via `Idempotency-Key` + cancellation endpoint/outbox event
  - ✅ Basic list/read path via `GET /api/v1/appointments`
  - ✅ Public slots availability via `GET /api/v1/public/slots` (simple workday-based calculation)
  - ✅ Slots now support business-driven config via business-service gRPC (service duration + business timezone + staff working hours)

Acceptance:
- Concurrent booking attempts do not double-book
- Duplicate public booking request (same idempotency key) returns same result

Verify:
- Load-ish test that fires concurrent booking attempts
- Outbox publishes booked + reminder requested events

## Phase 6 - Scheduler Service (Generic, Reusable)
Status: DONE

Deliverables:
- `scheduler-service`: durable delayed jobs table + polling (`FOR UPDATE SKIP LOCKED`)
- Consumes `booking.reminder.requested.v1`, stores job with idempotency key
- Emits `scheduler.reminder.due.v1` when due; retry policy + DLQ

Acceptance:
- Scheduler emits due reminders reliably across restarts
- Duplicate reminder requested event does not create duplicate jobs
- Status notes:
  - ✅ Reminder requested -> job -> due event pipeline implemented with retries + DLQ

Verify:
- Integration test: insert reminder request -> observe due event published

## Phase 7 - Notification Service (Email/SMS Providers)
Status: DONE

Deliverables:
- `notification-service`: consumes `scheduler.reminder.due.v1`
- Email provider interface (Mailpit for local); SMS provider interface (mock)
- Emits `notification.sent.v1` / `notification.failed.v1`
- Retries + DLQ; inbox dedupe

Acceptance:
- Email shows up in Mailpit for a due reminder
- Failure triggers retry and then DLQ after max attempts
- Status notes:
  - ✅ `notification.sent.v1` and `notification.failed.v1` emission implemented
  - ✅ SMS webhook provider added (email still uses SMTP/Mailpit)

Verify:
- Compose run -> book appointment -> observe reminder -> Mailpit receives email

## Phase 8 - Billing Service (Entitlements)
Status: DONE

Deliverables:
- Stripe checkout + webhook handling (idempotent by Stripe event id)
- Entitlements read model exposed via gRPC `EntitlementsService`
- Events: `billing.subscription.activated.v1`, `billing.subscription.canceled.v1`
- Status notes:
  - ✅ billing-service DB + outbox publisher + local webhook simulator (`/api/v1/billing/webhooks/local`)
  - ✅ Stripe webhook endpoint (`/api/v1/billing/webhooks/stripe`) validates `Stripe-Signature` using `STRIPE_WEBHOOK_SECRET` and is idempotent by Stripe event id
  - ✅ booking-service caches entitlements (`business_entitlements`) from billing events and enforces monthly cap (402 when exceeded)
  - ✅ Checkout session creation (`/api/v1/billing/checkout`) creates subscription-mode Checkout Session and sets metadata (`business_id`, `tier`)
  - ✅ Subscription cancel endpoint (`/api/v1/billing/subscription/cancel`) calls Stripe API and emits `billing.subscription.canceled.v1`
  - ✅ Optional reconciliation loop (env-gated) fetches Stripe subscription state periodically to self-heal if webhooks were missed

Acceptance:
- Webhook replay does not duplicate subscription state
- Booking limits can be enforced (initially at gateway or booking-service)

Verify:
- Local webhook simulation test (or stripe-cli if you want later)
- Smoke:
  - `./scripts/smoke-billing-enforcement.sh`
  - `./scripts/smoke-billing-stripe-webhook.sh` (requires `STRIPE_WEBHOOK_SECRET`)
  - Reconcile (optional): set `BILLING_STRIPE_RECONCILE_ENABLED=true` + `STRIPE_SECRET_KEY=sk_test_...` and restart billing-service

## Phase 9 - Gateway (BFF, Auth, Rate Limits)
Status: DONE

Deliverables:
- Routes public + private APIs to services
- JWT validation and request context propagation (trace/request id)
- Rate limiting + basic abuse controls for public booking endpoints
- OpenAPI served at `/openapi` (or static file) for developer UX
- Status notes:
  - ✅ Reverse proxy routes, JWT validation (HS256 or RS256 via JWKS), RBAC, rate limits, timeouts, `/openapi`
  - ✅ OpenTelemetry spans + trace propagation (HTTP + gRPC + Kafka)
  - ✅ Optional Redis-backed rate limiter for multi-instance deployments (`REDIS_ADDR`)

Acceptance:
- Only gateway is exposed publicly in compose; internal services are still reachable locally but treated as private
- Authenticated routes require valid JWT

Verify:
- End-to-end happy path works through gateway
- `./scripts/smoke-gateway-rate-limit.sh`

## Phase 10 - Analytics (Read Models + Dashboards)
Status: DONE

Deliverables:
- `analytics-service` consumes booking + notification events
- Aggregates for daily appointments, cancellations, reminder success rate
- Optional: Grafana dashboards (later)
- Status notes:
  - ✅ Consumers for notification + scheduler DLQ + auth audit exist
  - ✅ Booking events aggregated into `daily_appointment_metrics`
  - ✅ Replay script added for appointment aggregates
  - ✅ Notification aggregates (`daily_notification_metrics`) + rebuild script

Acceptance:
- Aggregates update correctly after event replay

Verify:
- Replay test: re-consume from earliest offset and recompute deterministically
 - `./scripts/rebuild-analytics-appointments.sh`
 - `./scripts/rebuild-analytics-notifications.sh`

## Phase 11 - Hardening (Production Readiness)
Status: DONE

Deliverables:
- Contract tests (OpenAPI + proto + event schema compatibility)
- Structured logs + full OTel traces across HTTP/gRPC/Kafka
- Consumer lag metrics; DLQ monitoring
- Security: secrets strategy, CORS, rate limiting tuned, audit logging for sensitive actions
- Deployment path: docker images tagged; optional k8s manifests/Helm chart

- Status notes:
  - ✅ Contract checks wired (OpenAPI/proto/event catalog + event schemas) and CI running them
  - ✅ Structured JSON logs + OTel traces across HTTP/gRPC/Kafka
  - ✅ Consumer lag helper + DLQ topics in compose
  - ✅ Security baseline documented (secrets, CORS, rate limits, audit logging for billing/auth)
  - ✅ Deployment path added (tagged images + minimal k8s skeleton)

Acceptance:
- CI includes unit + integration + contract checks
- Local runbook is complete and reproducible

Verify:
- `make test-proto`
- Compose: full end-to-end (register -> configure business -> book -> reminder -> notification)
 - `./scripts/check-openapi-sync.sh`
 - `./scripts/check-proto-clean.sh`
 - `./scripts/check-events-consistency.sh`
 - `./scripts/check-event-schemas.sh`
 - `./scripts/consumer-lag.sh`
 - `./scripts/smoke-auth.sh`
 - `./scripts/smoke-availability.sh`
 - `./scripts/smoke-timeoff-overlap.sh`
 - `./scripts/smoke-booking-outside-availability.sh`
 - `./scripts/smoke-booking-reminder.sh`
 - `./scripts/smoke-notification-pipeline.sh`
 - `./scripts/smoke-billing-enforcement.sh`
 - `./scripts/smoke-billing-stripe-webhook.sh`
  - `./scripts/smoke-gateway-rate-limit.sh`

## Phase 12 - Frontend (Next.js + Vite + shadcn/ui)
Status: PLANNED

Deliverables:
- Modern owner dashboard + public booking UI integrated with gateway
- Typed API client generated from OpenAPI
- Auth flows + protected routes + robust loading/error UX
- Playwright E2E for core flows

Plan:
- `docs/frontend/phases.md`
