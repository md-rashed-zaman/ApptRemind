# ApptRemind

Appointment booking + reminders SaaS built as an event-driven microservices system.

## Purpose
Service businesses lose time and revenue when bookings are handled manually and customers forget appointments (no-shows).
ApptRemind solves this by providing:
- a public booking flow (availability, booking, cancellation)
- a durable scheduler for reminders
- notification delivery (email now; SMS via pluggable provider)
- subscription/billing entitlements to control usage
- analytics read models to track outcomes over time

This repo is also a production-practice learning project: it demonstrates common patterns for scalable backends (outbox/inbox, DLQ, tracing, rate limiting, etc.).
- Language: Go (services + shared libs)
- Infra: Postgres, Kafka, Redis, Mailpit, Jaeger
- Patterns: Outbox, Inbox (idempotency), Saga-style workflows, DLQ

This README is the main entry point; deeper docs live in `docs/`.

## Quick Start (Local, Docker Compose)
From the repo root:

```bash
cd deploy/compose
cp .env.example .env
# optional: STRIPE_WEBHOOK_SECRET=whsec_test
# optional: JWT_PRIVATE_KEY_PEM / JWKS_URL for RS256

docker compose up --build
```

If Kafka fails to start with a `clusterId` or `/brokers/ids/1` error, reset Kafka state:
```bash
docker compose down
docker volume rm apptremind_kafka-data
docker compose up -d
```

Health checks:
```bash
curl -sS localhost:8080/healthz
curl -sS localhost:8080/readyz
```

## Run Frontend Locally
Start the backend first (Docker Compose above), then run the admin UI:

```bash
pnpm -C apps/web dev
```

Open the admin UI:
- Admin UI: `http://localhost:3000`
- Public booking demo: `http://localhost:3000/public-demo`
- Status page: `http://localhost:3000/status`

If you prefer a production build of the UI:
```bash
pnpm -C apps/web build
pnpm -C apps/web start
```

Optional environment override:
```bash
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 pnpm -C apps/web dev
```

## End-to-End Tests (Playwright)
Make sure the backend stack is running, then run:
```bash
PLAYWRIGHT_NO_SANDBOX=1 pnpm -C apps/web exec playwright test
```

If you see a `.next/lock` error (stale build lock):
```bash
rm -f apps/web/.next/lock
```

## Key Endpoints
- Gateway OpenAPI: `http://localhost:8080/openapi`
- JWKS: `http://localhost:8080/.well-known/jwks.json`
- Swagger UI (compose): `http://localhost:8088/docs` (started automatically with `docker compose up --build`)
If you see a CORS error in Swagger UI, ensure `CORS_ALLOWED_ORIGINS` includes `http://localhost:8088` and restart:
```bash
docker compose -f deploy/compose/docker-compose.yml up -d --build gateway-service swagger-ui
```

## Services
- gateway-service: HTTP entry point, auth, RBAC, rate limits, CORS, OpenAPI
- auth-service: users, JWT issuance, refresh/rotation, audit events
- business-service: profile, services, staff, working hours, time-off; gRPC for booking
- booking-service: availability + booking + cancellations, idempotency, outbox events
- scheduler-service: durable delayed jobs for reminders, retries + DLQ
- notification-service: email (SMTP/Mailpit) + SMS (webhook) providers
- billing-service: Stripe checkout/webhooks, entitlements, subscription state
- analytics-service: read models + daily aggregates

## Architecture (System Overview)
```mermaid
flowchart TB
  %% A more organized view: Edge -> Sync services -> Async/eventing -> Data/Infra.

  subgraph Edge["Edge"]
    Client[Client / Frontend]
    GW[gateway-service<br>HTTP API Gateway]
    Client -->|HTTP| GW
  end

  subgraph Sync["Synchronous Services (HTTP/gRPC)"]
    AUTH[auth-service<br>JWT + users]
    BUS[business-service<br>master data]
    BOOK[booking-service<br>booking + availability]
    BILL[billing-service<br>billing + entitlements]

    GW -->|HTTP| AUTH
    GW -->|HTTP| BUS
    GW -->|HTTP| BOOK
    GW -->|HTTP| BILL

    BOOK -->|gRPC| BUS
  end

  subgraph Async["Asynchronous Services (Kafka)"]
    KAFKA[(Kafka)]
    SCH[scheduler-service<br>delayed jobs]
    NOTIF[notification-service<br>email/sms]
    AN[analytics-service<br>read models]

    BOOK -->|publish| KAFKA
    BILL -->|publish| KAFKA
    AUTH -->|publish| KAFKA

    KAFKA -->|consume| SCH
    SCH -->|publish| KAFKA
    KAFKA -->|consume| NOTIF
    NOTIF -->|publish| KAFKA
    KAFKA -->|consume| AN
  end

  subgraph DataInfra["Data & Infra"]
    AUTHDB[(auth_db)]
    BUSDB[(business_db)]
    BOOKDB[(booking_db)]
    BILLDB[(billing_db)]
    SCHDB[(scheduler_db)]
    NOTIFDB[(notification_db)]
    ANDB[(analytics_db)]
    REDIS[(Redis)]
    MAILPIT[(Mailpit)]
    JAEGER[(Jaeger)]

    AUTH --> AUTHDB
    BUS --> BUSDB
    BOOK --> BOOKDB
    BILL --> BILLDB
    SCH --> SCHDB
    NOTIF --> NOTIFDB
    AN --> ANDB

    GW -->|rate limit| REDIS
    NOTIF -->|SMTP| MAILPIT
  end

  %% Observability (OTel traces)
  GW -.->|OTel| JAEGER
  AUTH -.->|OTel| JAEGER
  BUS -.->|OTel| JAEGER
  BOOK -.->|OTel| JAEGER
  BILL -.->|OTel| JAEGER
  SCH -.->|OTel| JAEGER
  NOTIF -.->|OTel| JAEGER
  AN -.->|OTel| JAEGER
```

## Data Flow (Booking → Reminder → Notification)
```mermaid
sequenceDiagram
  autonumber
  participant Client
  participant GW as Gateway
  participant BOOK as Booking
  participant K as Kafka
  participant SCH as Scheduler
  participant NOTIF as Notification
  participant MP as Mailpit

  Client->>GW: POST /api/v1/public/book
  GW->>BOOK: forward request
  BOOK->>BOOK: store appointment + outbox event
  BOOK->>K: booking.reminder.requested.v1
  K->>SCH: consume reminder request
  SCH->>SCH: store job + wait until remind_at
  SCH->>K: scheduler.reminder.due.v1
  K->>NOTIF: consume reminder due
  NOTIF->>MP: send email
  NOTIF->>K: notification.sent.v1
```

## Data Flow (Billing → Entitlements)
```mermaid
sequenceDiagram
  autonumber
  participant Owner
  participant GW as Gateway
  participant BILL as Billing
  participant Stripe
  participant K as Kafka
  participant BOOK as Booking

  Owner->>GW: POST /api/v1/billing/checkout
  GW->>BILL: forward request
  BILL->>Stripe: create checkout session
  Stripe-->>BILL: webhook checkout.session.completed
  BILL->>K: billing.subscription.activated.v1
  K->>BOOK: update entitlements cache
  BOOK-->>Owner: enforce monthly cap (402 if exceeded)
```

## ER Diagrams (Core Tables)

### Booking DB
```mermaid
erDiagram
  appointments {
    uuid id PK
    uuid business_id
    uuid staff_id
    uuid service_id
    timestamptz start_time
    timestamptz end_time
    text status
  }
  outbox_events {
    uuid event_id PK
    varchar event_type
    jsonb payload
    timestamptz created_at
    timestamptz published_at
  }
  inbox_events {
    uuid event_id PK
    varchar event_type
    timestamptz received_at
  }
  business_entitlements {
    uuid business_id PK
    varchar tier
    int max_monthly_appointments
    timestamptz updated_at
  }

  appointments ||--o{ outbox_events : emits
  inbox_events }o--|| appointments : dedupe
  business_entitlements ||--o{ appointments : limits
```

### Scheduler DB
```mermaid
erDiagram
  reminder_jobs {
    bigint id PK
    text idempotency_key
    uuid appointment_id
    uuid business_id
    text channel
    text recipient
    timestamptz remind_at
    int attempts
    timestamptz next_attempt_at
  }
  outbox_events {
    uuid event_id PK
    varchar event_type
    jsonb payload
    timestamptz created_at
    timestamptz published_at
  }
  inbox_events {
    uuid event_id PK
    varchar event_type
    timestamptz received_at
  }

  reminder_jobs ||--o{ outbox_events : emits
  inbox_events }o--|| reminder_jobs : dedupe
```

### Notification DB
```mermaid
erDiagram
  notifications {
    bigint id PK
    uuid appointment_id
    uuid business_id
    text channel
    text recipient
    text status
    timestamptz created_at
  }
  outbox_events {
    uuid event_id PK
    varchar event_type
    jsonb payload
    timestamptz created_at
    timestamptz published_at
  }
  inbox_events {
    uuid event_id PK
    varchar event_type
    timestamptz received_at
  }

  notifications ||--o{ outbox_events : emits
  inbox_events }o--|| notifications : dedupe
```

## Contract Docs
- Event catalog: `docs/events.md`
- Event schemas: `docs/event-schemas/*.json`
- OpenAPI: `openapi/gateway.v1.yaml`
- Protos: `protos/`

## Running Tests
```bash
make test
make test-proto
```

Contract checks:
```bash
./scripts/check-openapi-sync.sh
./scripts/check-proto-clean.sh
./scripts/check-events-consistency.sh
./scripts/check-event-schemas.sh
```

Smoke suite (end-to-end):
```bash
./scripts/smoke-auth.sh
./scripts/smoke-availability.sh
./scripts/smoke-timeoff-overlap.sh
./scripts/smoke-booking-outside-availability.sh
./scripts/smoke-booking-reminder.sh
./scripts/smoke-notification-pipeline.sh
./scripts/smoke-billing-enforcement.sh
STRIPE_WEBHOOK_SECRET=whsec_test ./scripts/smoke-billing-stripe-webhook.sh
./scripts/smoke-gateway-rate-limit.sh
```

## Migrations
All migrate scripts share a standard runner (`scripts/migrate-service.sh`).

```bash
make migrate-auth
make migrate-business
make migrate-booking
make migrate-billing
make migrate-notification
make migrate-analytics
make migrate-scheduler
```

## Kafka
Bootstrap topics:
```bash
make bootstrap-topics
```

Publish test events:
```bash
./scripts/publish-test-event.sh
./scripts/publish-reminder-event.sh
./scripts/publish-reminder-requested.sh
./scripts/publish-scheduler-dlq.sh
```

## Observability
- Logs: JSON (slog)
- Tracing: OpenTelemetry → Jaeger (`http://localhost:16686`)

## Production Notes
- Security baseline: `docs/security.md`
- Local runbook: `docs/runbook/local-dev.md`
- Phased delivery tracker: `docs/phases.md`
- K8s skeleton (optional): `deploy/k8s/README.md`

## Frontend (Next.js + TypeScript)
Frontend plan:
- `docs/frontend/phases.md`

Run the Next.js app:
```bash
corepack enable
pnpm install
pnpm dev
```

Run the UI playground (Vite):
```bash
pnpm ui:dev
```

Generate TypeScript types from OpenAPI:
```bash
pnpm api:gen
```

## What’s Next
Backend is done and fully tested. Frontend execution plan:
- `docs/frontend/phases.md`
