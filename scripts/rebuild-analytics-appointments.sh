#!/usr/bin/env bash
set -euo pipefail

DB_URL="${ANALYTICS_DATABASE_URL:-postgres://analytics_user:analytics_password@postgres:5432/analytics_db?sslmode=disable}"
COMPOSE_FILE="${COMPOSE_FILE:-deploy/compose/docker-compose.yml}"

run_psql() {
  if command -v psql >/dev/null 2>&1; then
    PGPASSWORD="analytics_password" psql "$DB_URL" "$@"
    return 0
  fi

  if command -v docker >/dev/null 2>&1; then
    docker compose -f "$COMPOSE_FILE" exec -T postgres \
      psql "postgres://analytics_user:analytics_password@localhost:5432/analytics_db?sslmode=disable" "$@"
    return 0
  fi

  echo "psql not found and docker not available" >&2
  return 1
}

run_psql <<'SQL'
BEGIN;
TRUNCATE TABLE daily_appointment_metrics;
INSERT INTO daily_appointment_metrics (business_id, day, booked_count, canceled_count, updated_at)
SELECT
  business_id,
  occurred_at::date AS day,
  SUM(CASE WHEN event_type = 'booking.appointment.booked.v1' THEN 1 ELSE 0 END) AS booked_count,
  SUM(CASE WHEN event_type = 'booking.appointment.cancelled.v1' THEN 1 ELSE 0 END) AS canceled_count,
  now()
FROM booking_events
GROUP BY business_id, day;
COMMIT;
SQL

echo "Rebuild complete."
