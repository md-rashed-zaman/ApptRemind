# Roadmap (production-oriented)

## Phase 0 - Repo + engineering baseline
- Standardize Go version, linting, CI, service template (health/metrics/logging).
- Docker Compose: Postgres, Kafka, optional Redis, Mailpit, observability stack.
- Shared libs: config, logging, http/gRPC middleware, db/migrations, kafka wrappers, otel.
- Event envelope + conventions (naming, versioning, partitioning, DLQ).

## Phase 1 - Auth + gateway
- auth-service: register/login, password hashing, JWT issuance, org membership.
- gateway-service: JWT validation, routing, request IDs, CORS, rate limits.
- Define RBAC model (owner/staff/admin) and tenant scoping (`business_id` claim).

## Phase 2 - Business master data
- business-service: business profile, staff, working hours, service catalog, reminder policy.
- Publish master-data events (optional early) or expose read APIs for booking-service.

## Phase 3 - Booking MVP (correctness first)
- booking-service: availability, book, cancel, list.
- DB-enforced no-overlap constraint + idempotent public booking endpoint.
- Outbox publisher emits `booking.appointment.booked.v1` / `booking.appointment.cancelled.v1`.
- Emit `booking.reminder.requested.v1` for each reminder time.

## Phase 4 - Scheduler + notifications (reusable services)
- scheduler-service: durable delayed-job table + polling + retries; emits `scheduler.reminder.due.v1`.
- notification-service: provider interface; start with Mailpit/email + mock SMS; emits status events.
- Inbox pattern in scheduler/notification; DLQ wiring.

## Phase 5 - Billing
- billing-service: Stripe checkout + webhooks (idempotent).
- Emit entitlement events (`billing.subscription.activated.v1`, `billing.subscription.canceled.v1`).
- Gateway enforces tier limits; services can also enforce locally.

## Phase 6 - Analytics/read models
- analytics-service consumes booking + notification events.
- Aggregates: daily appointments, cancellations, reminder success rate, no-show proxy metrics.

## Phase 7 - Hardening + scale
- Contract tests for events + APIs; integration tests with containers.
- Load testing (booking concurrency) + consumer lag monitoring.
- K8s manifests/Helm (optional) + migration strategy.
- Data retention policies + GDPR delete workflow (saga).

