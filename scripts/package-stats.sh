#!/usr/bin/env bash
# Generate machine-readable package statistics from weft.json manifests.
# Output: JSON with stdlib count, registry module count, maturity breakdown,
# platform support, and provider status.
set -euo pipefail
cd "$(dirname "$0")/.."

# Count stdlib packages
STDLIB_COUNT=$(go build -o /tmp/weft-stats ./cmd/weft 2>/dev/null && /tmp/weft-stats stdlib 2>/dev/null | wc -l | tr -d ' ')

# Count registry modules
REGISTRY_COUNT=$(python3 -c "import json; d=json.load(open('packages/index.json')); print(len(d['packages']))")

# Count Go tests
# Go test count (slow; skip with SKIP_GO_TESTS=1)
if [[ "${SKIP_GO_TESTS:-}" == "1" ]]; then
  GO_TESTS=0
else
  GO_TESTS=$(go test ./... -count=1 -timeout 300s 2>&1 | grep -oE '[0-9]+ passed' | head -1 | grep -oE '^[0-9]+' || echo "0")
fi

# Maturity breakdown from weft.json files
EXPERIMENTAL=0
BETA=0
STABLE=0
for manifest in packages/*/weft.json; do
  maturity=$(python3 -c "import json; d=json.load(open('$manifest')); print(d.get('maturity','experimental'))" 2>/dev/null)
  case "$maturity" in
    stable) STABLE=$((STABLE + 1)) ;;
    beta) BETA=$((BETA + 1)) ;;
    *) EXPERIMENTAL=$((EXPERIMENTAL + 1)) ;;
  esac
done

# Platform support
PLATFORMS="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64"

cat << STATS
{
  "version": "$(grep 'const Version' pkg/weft/weft.go | sed 's/.*"\(.*\)".*/\1/')",
  "stdlib_packages": $STDLIB_COUNT,
  "registry_modules": $REGISTRY_COUNT,
  "go_tests": ${GO_TESTS:-0},
  "maturity": {
    "experimental": $EXPERIMENTAL,
    "beta": $BETA,
    "stable": $STABLE
  },
  "platforms": "$(echo $PLATFORMS | tr ' ' ',')",
  "accelerator_providers": {
    "cpu_reference": "tested",
    "cuda": "validated (NVIDIA A2)",
    "rocm": "compile-verified",
    "mlx": "validated (Apple M4)"
  }
}
STATS
