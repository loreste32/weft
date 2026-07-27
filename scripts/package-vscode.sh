#!/usr/bin/env bash
# Build a VS Code VSIX for Weft (requires network for npm the first time).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
EXT="$ROOT/editors/vscode"
cd "$EXT"

if ! command -v npm >/dev/null 2>&1; then
  echo "npm required" >&2
  exit 1
fi

echo "== npm install =="
npm install --silent

echo "== vsce package =="
# --allow-missing-repository if private; keep repo field in package.json for public
npx --yes @vscode/vsce package --no-update-package-json

ls -la weft-*.vsix
echo "Install: code --install-extension $EXT/weft-*.vsix --force"
