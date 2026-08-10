#!/usr/bin/env bash
# Check that package exports declared in weft.json match actual pub fn declarations.
# Fails if a declared export has no corresponding pub fn, or if a pub fn is not exported.
set -euo pipefail
cd "$(dirname "$0")/.."

failures=0

for manifest in packages/*/weft.json; do
  pkg_dir=$(dirname "$manifest")
  pkg_name=$(basename "$pkg_dir")

  # Skip packages without exports
  exports=$(python3 -c "import json; d=json.load(open('$manifest')); print('\n'.join(d.get('exports',[])))" 2>/dev/null)
  if [[ -z "$exports" ]]; then continue; fi

  # Collect all pub fn names from .weft files (skip test files and vendor)
  pub_fns=$(grep -h '^pub fn ' "$pkg_dir"/*.weft 2>/dev/null | grep -v '_test.weft' | sed 's/pub fn \([a-zA-Z_][a-zA-Z0-9_]*\).*/\1/' | sort -u)

  # Check each declared export exists as a pub fn
  while IFS= read -r export; do
    if ! echo "$pub_fns" | grep -qx "$export"; then
      echo "MISSING: $pkg_name declares export '$export' but no pub fn found" >&2
      failures=$((failures + 1))
    fi
  done <<< "$exports"
done

if [[ "$failures" -ne 0 ]]; then
  echo "export check failed ($failures missing)" >&2
  exit 1
fi
echo "export check ok"
