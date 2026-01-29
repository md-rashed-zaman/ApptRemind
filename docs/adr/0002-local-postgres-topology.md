# ADR 0002: Local Postgres Topology

## Status
Accepted

## Context
We want local development to match production practices (DB-per-service) while keeping ops simple.

## Decision
Run a single Postgres container locally, but create **one database per service** and a **dedicated DB user per service** with least-privilege grants.

Example (names only):
- `auth_db` + `auth_user`
- `business_db` + `business_user`
- `booking_db` + `booking_user`
- `billing_db` + `billing_user`
- `scheduler_db` + `scheduler_user`
- `notification_db` (optional, for inbox/outbox/audit) + `notification_user`
- `analytics_db` + `analytics_user`

## Consequences
- Matches production data ownership boundaries without multiple Postgres instances.
- Clear migration ownership per service.
- Easier to move to managed Postgres later (same logical shape, different hosting).

