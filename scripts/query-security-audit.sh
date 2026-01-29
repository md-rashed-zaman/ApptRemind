#!/usr/bin/env bash
set -euo pipefail

LIMIT="${1:-10}"

psql "postgres://analytics_user:analytics_password@localhost:5432/analytics_db?sslmode=disable" \
  -c "SELECT id, event_type, actor_id, created_at FROM security_audit_events ORDER BY id DESC LIMIT ${LIMIT};"
