# System Design - ApptRemind

This document describes a production-oriented, cloud-portable architecture for ApptRemind.

## Goals
- Multi-tenant B2B SaaS: many businesses, isolated data and quotas.
- Event-driven services with clear boundaries and reusable platform components.
- Local-first dev (Docker Compose), portable to paid clouds later (AWS/GCP/etc).
- Production patterns from day 1: outbox/inbox, idempotency, retries/DLQ, observability.

## Architecture (high level)
- Edge: reverse proxy (Traefik/Nginx) for routing + TLS termination.
- Public API: HTTP/REST via gateway/BFF.
- Internal: prefer async events; use gRPC only for "must-be-sync" reads/commands.
- Event backbone: Kafka topics per domain, versioned schemas.
- Data: Postgres, one logical database per service (separate DB + credentials), no shared tables.

### Service map
- gateway-service (HTTP): auth middleware, routing, rate limits, public booking endpoints.
- auth-service: users, sessions/JWT, org membership.
- business-service: businesses, staff, working hours, services catalog, reminder policy defaults.
- booking-service: availability + appointment lifecycle; emits appointment events.
- billing-service: subscriptions, Stripe integration; emits entitlement events.
- scheduler-service (generic, reusable): durable delayed-job engine -> emits due events.
- notification-service (reusable): consumes reminder due events; sends email/SMS; emits status events.
- analytics-service (optional early): consumes events; builds read models/aggregates.

## Production patterns used
### Outbox pattern (per service)
Each service writes domain changes and an outbox row in the same DB transaction.
An outbox publisher publishes to Kafka and marks the outbox row as published.

Why: prevents "DB commit succeeded but event publish failed" and enables retries.

### Inbox pattern (per consumer)
Consumers record processed message IDs (or idempotency keys) before/with side effects.

Why: Kafka is at-least-once; inbox prevents duplicates from causing double side effects.

### Sagas (for cross-service workflows)
Use events for choreography by default; use an orchestrator only when needed.

Common sagas:
- Business signup: auth-service creates user -> emits `auth.user.created.v1`; business-service creates business -> emits `business.created.v1`; gateway can rely on eventual consistency or block on a sync call to business-service for immediate UX.
- Subscription upgrade: billing-service handles Stripe webhook -> emits `billing.subscription.activated.v1`; other services update entitlements/read models.
- Paid booking (future): booking-service "reserve slot" -> billing-service "authorize payment" -> booking-service "confirm appointment" or "release reservation".

### Idempotency (public endpoints + webhooks)
- Public booking endpoint requires an idempotency key to avoid duplicate bookings.
- Stripe webhooks handled idempotently by event ID.

### Retries + DLQ
- Consumers retry transient failures with backoff; poison messages go to `*.dlq` topics.
- Notification sending retries with per-provider policies.

## Sync vs async communication (REST vs gRPC)
Default: async events for state changes.

Use gRPC for:
- Read queries needed in-request (e.g., booking checks entitlements from billing).
- Internal service-to-service commands that must return immediately.

Use REST for:
- External/public APIs (browser/mobile/partners) via gateway with OpenAPI.

Recommendation:
- Public: REST (OpenAPI).
- Internal: gRPC for the few synchronous reads/commands; keep it minimal to avoid tight coupling.

## Kafka topic strategy
- Domain topics, versioned: `booking.appointment.booked.v1`, `scheduler.job.requested.v1`, etc.
- Partition key: `business_id` (preserves per-tenant ordering, enables horizontal scale).
- Consumer groups: one per service instance set.
- DLQ topics: `booking.appointment.booked.v1.dlq` (or `dlq.<topic>`).

Event envelope (CloudEvents-ish fields):
- `id`, `type`, `source`, `time`, `specversion`, `subject`, `datacontenttype`, `trace_id`, `data`, `version`.

## Data & tenancy
Best practice for microservices:
- One logical database per service (separate DB + credentials + migrations).
- Physical deployment can still be one Postgres cluster for cost; isolation is logical + permissions.

Local dev decision:
- Use a single Postgres container hosting multiple per-service databases, each with its own DB role and least-privilege grants.

Multi-tenancy:
- Every table includes `business_id` where applicable.
- Composite indexes start with `business_id`.
- Later: Postgres RLS can be added per service if needed.

Booking correctness (double booking prevention):
- Use a DB-level constraint to prevent overlapping appointments per staff.
  For example, an exclusion constraint on `tstzrange(start_time, end_time)`.

## Scheduler design (generic, reusable)
Scheduler owns a durable jobs table and emits due events.

Flow for reminders:
1. booking-service emits `booking.reminder.requested.v1` with (business_id, appointment_id, remind_at, channel, recipient, template_data).
2. scheduler-service consumes and persists a job with an idempotency key (e.g., `appointment_id|remind_at|channel`).
3. scheduler-service polls due jobs using `FOR UPDATE SKIP LOCKED`, marks as running, emits `scheduler.reminder.due.v1`, marks job as completed (or schedules retry).
4. notification-service consumes `scheduler.reminder.due.v1`, sends, emits `notification.sent.v1` / `notification.failed.v1`.

## Observability (production defaults)
- Structured logs with request/message IDs.
- OpenTelemetry tracing across HTTP/gRPC + Kafka (propagate trace context in headers).
- Metrics: Prometheus (latency, error rates, consumer lag, retries, DLQ count).
- Health: `/healthz` (liveness), `/readyz` (readiness), plus DB/Kafka checks on readiness.

## Security baseline
- JWT validation at gateway; services validate authz for sensitive operations.
- Secrets via env in dev; later swap to Vault/Infisical/AWS Secrets Manager.
- Rate limiting at edge/gateway; basic abuse protection on public booking endpoints.
