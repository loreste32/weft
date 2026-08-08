#!/usr/bin/env bash
# Fail when example vendor copies drift from canonical packages/.
set -euo pipefail
cd "$(dirname "$0")/.."

packages=(warp ml)
examples=(examples/agent_stack examples/ml_demo)
# packages/ml also vendors warp as a nested dependency
nested_vendors=("warp:packages/ml")
failures=0

for pkg in "${packages[@]}"; do
  canonical="packages/${pkg}"
  if [[ ! -d "$canonical" ]]; then
    echo "missing canonical package: $canonical" >&2
    failures=$((failures + 1))
    continue
  fi
  for example in "${examples[@]}"; do
    vendor="${example}/vendor/${pkg}"
    if [[ ! -d "$vendor" ]]; then
      # Not every example vendors every package.
      continue
    fi
    # Compare canonical package source files only. A package may contain a
    # nested vendor tree for an embedded dependency; that tree is owned by
    # its dependency, not by the example's top-level package copy.
    while IFS= read -r -d '' file; do
      rel="${file#${canonical}/}"
      case "$rel" in
        weft.json) continue ;;
      esac
      other="${vendor}/${rel}"
      if [[ ! -f "$other" ]]; then
        echo "vendor missing ${other} (from ${file})" >&2
        failures=$((failures + 1))
        continue
      fi
      if ! diff -u "$file" "$other" >/tmp/weft-vendor-diff.out 2>&1; then
        echo "vendor drift: ${file} != ${other}" >&2
        head -n 40 /tmp/weft-vendor-diff.out >&2 || true
        failures=$((failures + 1))
      fi
    done < <(find "$canonical" -type d -name vendor -prune -o -type f \( -name '*.weft' -o -name 'README.md' \) -print0)
  done
done

# Check nested vendor copies (e.g. packages/ml/vendor/warp)
for entry in "${nested_vendors[@]}"; do
  pkg="${entry%%:*}"
  parent="${entry##*:}"
  canonical="packages/${pkg}"
  vendor="${parent}/vendor/${pkg}"
  if [[ ! -d "$vendor" ]]; then
    continue
  fi
  while IFS= read -r -d '' file; do
    rel="${file#${canonical}/}"
    case "$rel" in
      weft.json) continue ;;
    esac
    other="${vendor}/${rel}"
    if [[ ! -f "$other" ]]; then
      echo "vendor missing ${other} (from ${file})" >&2
      failures=$((failures + 1))
      continue
    fi
    if ! diff -u "$file" "$other" >/tmp/weft-vendor-diff.out 2>&1; then
      echo "vendor drift: ${file} != ${other}" >&2
      head -n 40 /tmp/weft-vendor-diff.out >&2 || true
      failures=$((failures + 1))
    fi
  done < <(find "$canonical" -type d -name vendor -prune -o -type f \( -name '*.weft' -o -name 'README.md' \) -print0)
done

if [[ "$failures" -ne 0 ]]; then
  echo "vendor sync check failed (${failures} issue(s))" >&2
  echo "hint: rsync -a packages/warp/ examples/*/vendor/warp/ (and ml) or reinstall packages" >&2
  exit 1
fi

echo "vendor sync ok"
