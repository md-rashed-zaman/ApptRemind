#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-deploy/compose/docker-compose.yml}"
BROKER="${BROKER:-kafka:9092}"

docker compose -f "$COMPOSE_FILE" exec -T kafka \
  kafka-consumer-groups --bootstrap-server "$BROKER" --describe --all-groups
