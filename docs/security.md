# Security Notes (Production Practices)

This document captures the baseline security posture for ApptRemind and the production-style defaults used in the project.

## Secrets management
- Local dev uses environment variables (compose `.env`) for all sensitive values.
- Production should use a secrets manager (e.g. Vault, Doppler, AWS/GCP secrets) and inject values as env vars at runtime.
- Never commit secrets to the repo. Rotate any leaked values immediately.
- Use least-privilege credentials (separate DB users per service are already used).

### Sensitive environment variables
- Auth:
  - `JWT_SECRET` (HS256 dev only)
  - `JWT_PRIVATE_KEY_PEM`, `JWT_PRIVATE_KEYS_PEM`, `JWT_ACTIVE_KID` (RS256)
  - `JWT_ROTATE_KEY` (protects `/api/v1/auth/rotate` + `/api/v1/auth/audit`)
- Billing:
  - `STRIPE_API_KEY`
  - `STRIPE_WEBHOOK_SECRET`
- Postgres:
  - `DATABASE_URL` (service-specific)
- Redis (gateway rate limit):
  - `REDIS_PASSWORD`
- SMTP (notification-service):
  - `SMTP_USER`, `SMTP_PASSWORD`

## CORS
CORS is enforced at the gateway only and is configured via env vars. Keep the allowed origins list tight in production.

## Rate limits
Rate limiting is enforced at the gateway. Use Redis-backed limits (`REDIS_ADDR`) in multi-instance deployments.

## Audit logging (sensitive actions)
- JWT key rotations are recorded in `auth_db.audit_events` and emitted to Kafka (`auth.audit.v1`).
- Billing provider events are persisted for traceability, and billing-service records sensitive actions in `billing_db.audit_events`.

## Logging + tracing
- All services emit structured JSON logs (slog) and export OTel traces to Jaeger/OTLP when enabled.
