#!/usr/bin/env bash
# Generate a dependency SBOM (roadmap N6) from the pinned module graph.
# Output: JSON with module path, version, and go.sum hashes for every
# dependency of the main module, plus the Go toolchain version.
#
# Usage: bash scripts/sbom.sh [output.json]   (default: stdout)
set -euo pipefail
cd "$(dirname "$0")/.."

out="${1:-}"
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

{
  echo '{'
  echo '  "format": "weft.sbom",'
  echo '  "version": 1,'
  printf '  "toolchain": "%s",\n' "$(go version | awk '{print $3}')"
  printf '  "module": "%s",\n' "$(go list -m)"
  echo '  "packages": ['

  first=1
  # Every module in the build list except the main module itself.
  # -mod=mod: a local (untracked) vendor/ dir must not change the result.
  go list -mod=mod -m all | tail -n +2 | while read -r path version _rest; do
    # go.sum lines: "<path> <version>/go.mod h1:...=" and "<path> <version> h1:...="
    modhash=$(awk -v p="$path" -v v="$version" '$1 == p && $2 == v {print $3; exit}' go.sum)
    modfilehash=$(awk -v p="$path" -v v="$version/go.mod" '$1 == p && $2 == v {print $3; exit}' go.sum)
    sep=','
    [ "$first" = 1 ] && { sep=''; first=0; }
    printf '    %s{"path": "%s", "version": "%s", "hash": "%s", "go_mod_hash": "%s"}\n' \
      "$sep" "$path" "$version" "${modhash:-}" "${modfilehash:-}"
  done

  echo '  ]'
  echo '}'
} > "$tmp"

# Validate JSON before emitting.
python3 -c "import json,sys; json.load(open(sys.argv[1]))" "$tmp"

if [ -n "$out" ]; then
  cp "$tmp" "$out"
  echo "sbom: $out"
else
  cat "$tmp"
fi
