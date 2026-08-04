#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

python3 - <<'PY'
import json
from pathlib import Path

root = Path("packages")
with (root / "index.json").open(encoding="utf-8") as handle:
    catalog = json.load(handle)

entries = catalog.get("packages")
if not isinstance(entries, list):
    raise SystemExit("catalog packages must be a JSON array")

by_name = {}
for entry in entries:
    name = entry.get("name")
    if not isinstance(name, str) or not name:
        raise SystemExit("catalog entry is missing a name")
    if name in by_name:
        raise SystemExit(f"duplicate catalog entry: {name}")
    by_name[name] = entry

failures = []
for entry in entries:
    name = entry["name"]
    manifest_path = root / name / "weft.json"
    if not manifest_path.is_file():
        failures.append(f"{name}: missing manifest {manifest_path}")
        continue
    with manifest_path.open(encoding="utf-8") as handle:
        manifest = json.load(handle)
    catalog_path = Path(entry.get("path", ""))
    if catalog_path != Path(f"./{name}"):
        failures.append(f"{name}: catalog path {catalog_path} != ./{name}")
    if entry.get("version") != manifest.get("version"):
        failures.append(
            f"{name}: catalog {entry.get('version')} != manifest {manifest.get('version')}"
        )

for manifest_path in sorted(root.glob("*/weft.json")):
    name = manifest_path.parent.name
    if name not in by_name:
        failures.append(f"{name}: top-level manifest is missing from catalog")

if failures:
    print("catalog sync failed:", flush=True)
    for failure in failures:
        print(f"- {failure}")
    raise SystemExit(1)

print(f"catalog sync ok ({len(entries)} packages)")
PY
