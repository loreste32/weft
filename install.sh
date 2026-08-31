#!/bin/sh
# Weft installer — https://weftproject.dev/install.sh
# Usage: curl -fsSL https://weftproject.dev/install.sh | sh
#        VERSION=0.3.1 sh install.sh   # pin a version
set -eu

REPO="loreste32/weft"
GITHUB="https://github.com/${REPO}"
API="https://api.github.com/repos/${REPO}"

err() { echo "install.sh: $*" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || err "curl is required"

# --- Detect OS / arch --------------------------------------------------------
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
	linux|darwin) ;;
	*) err "unsupported OS: $os (supported: linux, darwin)" ;;
esac

arch=$(uname -m)
case "$arch" in
	x86_64|amd64) arch="amd64" ;;
	arm64|aarch64) arch="arm64" ;;
	*) err "unsupported architecture: $(uname -m) (supported: amd64, arm64)" ;;
esac

# --- Resolve version ---------------------------------------------------------
if [ "${VERSION:-}" != "" ]; then
	tag="${VERSION#v}" # allow VERSION=v0.3.1 or 0.3.1
else
	echo "Fetching latest release..."
	resp=$(curl -fsSL "$API/releases/latest") || err "could not query latest release"
	if command -v jq >/dev/null 2>&1; then
		tag=$(printf '%s' "$resp" | jq -r '.tag_name')
	else
		tag=$(printf '%s' "$resp" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
	fi
	[ -n "$tag" ] || err "could not determine latest release tag"
	tag="${tag#v}"
fi
echo "Installing weft $tag ($os/$arch)..."

asset="weft-${tag}-${os}-${arch}.tar.gz"
base="$GITHUB/releases/download/v${tag}"

# --- Download ----------------------------------------------------------------
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

curl -fsSL -o "$tmpdir/$asset" "$base/$asset" || err "download failed: $base/$asset"
curl -fsSL -o "$tmpdir/SHA256SUMS" "$base/SHA256SUMS" || err "download failed: $base/SHA256SUMS"

# --- Verify SHA-256 ----------------------------------------------------------
expected=$(awk -v f="$asset" '$2 == f {print $1}' "$tmpdir/SHA256SUMS")
[ -n "$expected" ] || err "$asset not listed in SHA256SUMS"

if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "$tmpdir/$asset" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
	actual=$(shasum -a 256 "$tmpdir/$asset" | awk '{print $1}')
else
	err "neither sha256sum nor shasum found — cannot verify checksum"
fi

[ "$actual" = "$expected" ] || err "checksum mismatch for $asset (expected $expected, got $actual)"
echo "Checksum verified."

# --- Install -----------------------------------------------------------------
tar -xzf "$tmpdir/$asset" -C "$tmpdir"
[ -f "$tmpdir/weft" ] || err "archive did not contain a weft binary"

if [ -w /usr/local/bin ]; then
	dest="/usr/local/bin"
else
	dest="$HOME/.local/bin"
	mkdir -p "$dest"
fi

install -m 755 "$tmpdir/weft" "$dest/weft"
echo "Installed weft $tag to $dest/weft"

case ":$PATH:" in
	*":$dest:"*) ;;
	*) echo "warning: $dest is not on your PATH — add it to your shell profile" >&2 ;;
esac

"$dest/weft" version 2>/dev/null || true
