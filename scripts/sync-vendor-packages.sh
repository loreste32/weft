#!/usr/bin/env bash
# Copy canonical packages into example vendor trees.
set -euo pipefail
cd "$(dirname "$0")/.."

sync_one() {
  local pkg=$1
  local example=$2
  local src="packages/${pkg}"
  local dst="${example}/vendor/${pkg}"
  if [[ ! -d "$dst" ]]; then
    return 0
  fi
  mkdir -p "$dst"
  # Preserve example weft.json version pins if present; sync canonical source
  # files. Nested vendor trees are dependency-owned and are not copied into
  # the example's package vendor directory.
  rsync -a --delete \
    --exclude 'weft.json' \
    --exclude 'vendor/' \
    "${src}/" "${dst}/"
  # Keep weft.json name/exports in sync but preserve version if file exists.
  if [[ -f "${src}/weft.json" ]]; then
    cp "${src}/weft.json" "${dst}/weft.json"
  fi
  echo "synced ${src} -> ${dst}"
}

for example in examples/agent_stack examples/ml_demo; do
  for pkg in warp ml; do
    sync_one "$pkg" "$example"
  done
done
