#!/usr/bin/env bash
# Cross-check release readiness: build (full + slim), version, compat, quick run.
# Intended for release CI or maintainers before tagging. Not a full matrix (that needs GH Actions).
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== go version =="
go version

echo "== full build =="
go build -o /tmp/weft-release-full ./cmd/weft
FULL_SIZE=$(wc -c </tmp/weft-release-full | tr -d ' ')
echo "full binary: $FULL_SIZE bytes ($(ls -lah /tmp/weft-release-full | awk '{print $5}'))"

echo "== slim build (-tags slim) =="
go build -tags slim -o /tmp/weft-release-slim ./cmd/weft
SLIM_SIZE=$(wc -c </tmp/weft-release-slim | tr -d ' ')
echo "slim binary: $SLIM_SIZE bytes ($(ls -lah /tmp/weft-release-slim | awk '{print $5}'))"
if [ "$SLIM_SIZE" -ge "$FULL_SIZE" ]; then
  echo "WARN: slim binary is not smaller than full ($SLIM_SIZE >= $FULL_SIZE)" >&2
else
  saved=$((FULL_SIZE - SLIM_SIZE))
  echo "slim saves ~$saved bytes vs full"
fi

echo "== version =="
/tmp/weft-release-full version | grep -q "weft 0."
/tmp/weft-release-slim version | grep -q "weft 0."

echo "== compat corpus =="
go test ./pkg/weft/ -count=1 -run TestCompatCorpus

echo "== bytecode validate path (check) =="
/tmp/weft-release-full check --strict testdata/compat/*.weft
/tmp/weft-release-slim check --strict testdata/compat/*.weft

echo "== run goldens =="
for f in testdata/compat/*.weft; do
  base=$(basename "$f" .weft)
  out=$(/tmp/weft-release-full run "$f")
  want=$(cat "testdata/compat/${base}.out")
  # trim trailing whitespace for compare
  out_n=$(printf '%s\n' "$out")
  want_n=$(printf '%s\n' "$want")
  if [ "$out_n" != "$want_n" ]; then
    echo "FAIL $base" >&2
    echo "want: $want_n" >&2
    echo "got:  $out_n" >&2
    exit 1
  fi
  echo "  ok $base"
done

echo "== slim still runs agent-core (json/llm package present) =="
/tmp/weft-release-slim run examples/hello.weft | grep -q "hello"

echo "== slim stubs db =="
# importing/using db should error clearly, not crash
cat >/tmp/slim_db.weft <<'EOF'
fn main {
    # just resolve package method — will fail at call
    _ := db.open("sqlite", ":memory:")
}
EOF
if /tmp/weft-release-slim run /tmp/slim_db.weft 2>/tmp/slim_db.err; then
  # might return Err result without process error
  true
fi
if ! grep -qi 'slim\|not available' /tmp/slim_db.err /tmp/slim_db.out 2>/dev/null; then
  # also check stdout
  out=$(/tmp/weft-release-slim run /tmp/slim_db.weft 2>&1 || true)
  echo "$out" | grep -qi 'slim\|not available' || {
    echo "WARN: slim db stub message not found: $out" >&2
  }
fi

echo "== GOOS matrix (compile only) =="
for pair in "linux/amd64" "linux/arm64" "darwin/amd64" "darwin/arm64" "windows/amd64"; do
  goos=${pair%/*}
  goarch=${pair#*/}
  out=/tmp/weft-${goos}-${goarch}
  ext=""
  [ "$goos" = "windows" ] && ext=".exe"
  echo "  $goos/$goarch"
  GOOS=$goos GOARCH=$goarch go build -o "${out}${ext}" ./cmd/weft
  GOOS=$goos GOARCH=$goarch go build -tags slim -o "${out}-slim${ext}" ./cmd/weft
done

echo "OK release-smoke full=$FULL_SIZE slim=$SLIM_SIZE"
