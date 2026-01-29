#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export ROOT_DIR

python3 - <<'PY'
import json
import os
import re
from pathlib import Path

root = Path(os.environ.get("ROOT_DIR", ".")).resolve()
docs = root / "docs" / "events.md"
schemas_dir = root / "docs" / "event-schemas"

if not docs.exists():
    raise SystemExit("docs/events.md not found")
if not schemas_dir.exists():
    raise SystemExit("docs/event-schemas directory not found")

doc_events = set(re.findall(r"^\s*- event:\s+([\w\.\-]+)$", docs.read_text(encoding="utf-8"), flags=re.MULTILINE))
schema_files = sorted(schemas_dir.glob("*.json"))
schema_events = {p.stem for p in schema_files}

missing = sorted(doc_events - schema_events)
extra = sorted(schema_events - doc_events)

if missing:
    print("Missing schemas for events:")
    for e in missing:
        print(f" - {e}")
    raise SystemExit(1)

if extra:
    print("Extra schemas not listed in docs/events.md:")
    for e in extra:
        print(f" - {e}")
    raise SystemExit(1)

def validate_schema(path: Path):
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except Exception as exc:
        raise SystemExit(f"{path.name}: invalid JSON ({exc})")

    if data.get("type") != "object":
        raise SystemExit(f"{path.name}: top-level type must be object")
    props = data.get("properties")
    if not isinstance(props, dict) or not props:
        raise SystemExit(f"{path.name}: properties must be a non-empty object")
    required = data.get("required", [])
    if not isinstance(required, list):
        raise SystemExit(f"{path.name}: required must be a list if present")
    missing_props = [r for r in required if r not in props]
    if missing_props:
        raise SystemExit(f"{path.name}: required fields missing from properties: {missing_props}")

for path in schema_files:
    validate_schema(path)

print("Event schemas validated.")
PY
