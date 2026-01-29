# Contracts Overview

This directory contains the initial contracts for public REST APIs and internal gRPC APIs.

## Public APIs (REST/OpenAPI)
- Gateway exposes all public routes.
- Contracts are versioned (v1) and documented in `openapi/gateway.v1.yaml`.

## Internal APIs (gRPC)
Only use gRPC for synchronous calls that are required for correctness in the same request flow.

Initial internal gRPC contracts:
- `entitlements.v1.EntitlementsService` for checking subscription limits.
- `business.v1.BusinessService` for read-only business metadata (timezone, reminder policy).

To generate Go stubs:
```bash
./scripts/gen-proto.sh
```
Or:
```bash
make proto
```

To enable the sample gRPC wiring in billing-service, build with:
```bash
go build -tags protogen ./services/billing-service/cmd/billing-service
```

To enable gRPC business policy lookups (booking -> business), build with:
```bash
go build -tags protogen ./services/business-service/cmd/business-service
go build -tags protogen ./services/booking-service/cmd/booking-service
```

To run gRPC-related tests (requires generated protos):
```bash
make test-proto
```

## Event Contracts (Kafka)
Current event schemas:
- `auth.user.created.v1` (see `docs/contracts/auth.user.created.v1.json`)
- `auth.audit.v1` (see `docs/contracts/auth.audit.v1.json`)
- `billing.subscription.activated.v1` (see `docs/contracts/billing.subscription.activated.v1.json`)
- `booking.appointment.cancelled.v1` (see `docs/contracts/booking.appointment.cancelled.v1.json`)
- `booking.reminder.requested.v1` (see `docs/contracts/booking.reminder.requested.v1.json`)
- `scheduler.reminder.due.v1` (see `docs/contracts/scheduler.reminder.due.v1.json`)
- `scheduler.reminder.dlq.v1` (see `docs/contracts/scheduler.reminder.dlq.v1.json`)
- `notification.sent.v1` (see `docs/contracts/notification.sent.v1.json`)
- `notification.failed.v1` (see `docs/contracts/notification.failed.v1.json`)

Events remain the default async integration path.
