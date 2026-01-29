#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! command -v protoc-gen-go >/dev/null 2>&1; then
  echo "Installing protoc-gen-go..."
  (cd "$ROOT_DIR" && go install google.golang.org/protobuf/cmd/protoc-gen-go@latest)
fi

if ! command -v protoc-gen-go-grpc >/dev/null 2>&1; then
  echo "Installing protoc-gen-go-grpc..."
  (cd "$ROOT_DIR" && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest)
fi

PROTO_SRC="$ROOT_DIR/protos/internal"
OUT_DIR="$ROOT_DIR"
MODULE_PATH="github.com/md-rashed-zaman/apptremind"

mkdir -p "$OUT_DIR"

protoc \
  -I "$PROTO_SRC" \
  --go_out="$OUT_DIR" --go_opt=paths=import --go_opt=module="$MODULE_PATH" \
  --go-grpc_out="$OUT_DIR" --go-grpc_opt=paths=import --go-grpc_opt=module="$MODULE_PATH" \
  "$PROTO_SRC"/*.proto

echo "Generated Go protos in $OUT_DIR"
