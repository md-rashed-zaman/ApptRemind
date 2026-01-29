#!/usr/bin/env bash
set -euo pipefail

echo "Bootstrapping Kafka topics via docker compose..."
docker compose -f deploy/compose/docker-compose.yml run --rm kafka-init
