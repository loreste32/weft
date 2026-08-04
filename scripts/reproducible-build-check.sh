#!/usr/bin/env bash
# Reproducible-build gate (roadmap N6): the same commit, built at two
# different paths, offline (GOPROXY=off after `go mod download`), with
# -trimpath -buildvcs=false, must produce byte-identical binaries.
#
# Usage: bash scripts/reproducible-build-check.sh [ref]
#   ref defaults to HEAD. Requires a clean-enough git state for `git archive`.
set -euo pipefail
cd "$(dirname "$0")/.."

ref="${1:-HEAD}"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "== reproducible build check ($ref) =="
echo "== offline dependency install (go mod download + verify) =="
go mod download
go mod verify

build_at() {
  local name="$1"
  local dir="$tmp/$name"
  mkdir -p "$dir"
  git archive "$ref" | tar -x -C "$dir"
  (
    cd "$dir"
    GOPROXY=off go build -mod=mod -trimpath -buildvcs=false -o "$tmp/$name.weft" ./cmd/weft
  )
  sha256sum "$tmp/$name.weft" | awk '{print $1}'
}

echo "== build A/B at different paths =="
ha=$(build_at a)
hb=$(build_at b)
echo "  a: $ha"
echo "  b: $hb"
if [ "$ha" != "$hb" ]; then
  echo "FAIL: builds are not byte-reproducible" >&2
  exit 1
fi
echo "OK reproducible: $ha"
