#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

IMAGE_TAG="${IMAGE_TAG:-dev}"
IMAGE_REGISTRY="${IMAGE_REGISTRY:-}"

if [[ -n "$IMAGE_REGISTRY" && "$IMAGE_REGISTRY" != */ ]]; then
  IMAGE_REGISTRY="${IMAGE_REGISTRY}/"
fi

services=(
  auth-service
  business-service
  booking-service
  billing-service
  scheduler-service
  notification-service
  analytics-service
  gateway-service
)

for svc in "${services[@]}"; do
  image="${IMAGE_REGISTRY}apptremind-${svc}:${IMAGE_TAG}"
  echo "Building ${image}"
  docker build \
    -f build/Dockerfile.go-service \
    --build-arg SERVICE="${svc}" \
    -t "${image}" \
    .
  echo "Built ${image}"
  echo
 done
