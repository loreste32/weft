#!/usr/bin/env bash
# Comparative glue microbenches: Weft vs Python3 (optional Lua/LuaJIT).
# Not part of every-PR CI (wall times vary). Run locally or on a quiet machine:
#   make bench-glue
#   bash scripts/bench-glue.sh
set -euo pipefail
cd "$(dirname "$0")/.."

WEFT_BIN="${WEFT_BIN:-}"
if [[ -z "$WEFT_BIN" ]]; then
  if [[ -x ./weft ]]; then
    WEFT_BIN=./weft
  else
    go build -o /tmp/weft-bench ./cmd/weft
    WEFT_BIN=/tmp/weft-bench
  fi
fi

PYTHON="${PYTHON:-python3}"
if ! command -v "$PYTHON" >/dev/null 2>&1; then
  echo "python3 not found; install Python for comparative benches" >&2
  exit 1
fi

# Prefer gdate (coreutils) on macOS for high-res timing; fall back to python.
now_ns() {
  if command -v gdate >/dev/null 2>&1; then
    gdate +%s%N
  else
    "$PYTHON" -c 'import time; print(int(time.perf_counter_ns()))'
  fi
}

run_timed() {
  # args: label command...
  local label=$1
  shift
  local start end ms
  start=$(now_ns)
  "$@" >/tmp/weft-bench-out.txt
  end=$(now_ns)
  ms=$("$PYTHON" -c "print(f'{(${end}-${start})/1e6:.1f}')")
  printf "%-22s %8s ms\n" "$label" "$ms"
}

verify_pair() {
  local weft_src=$1
  local py_src=$2
  local name
  name=$(basename "$weft_src" .weft)
  "$WEFT_BIN" run "$weft_src" > /tmp/weft-bench-w.out
  "$PYTHON" "$py_src" > /tmp/weft-bench-p.out
  if ! diff -u /tmp/weft-bench-w.out /tmp/weft-bench-p.out >/dev/null; then
    echo "output mismatch for $name (Weft vs Python) — refusing to time" >&2
    diff -u /tmp/weft-bench-w.out /tmp/weft-bench-p.out >&2 || true
    exit 1
  fi
}

echo "== glue bench (Weft $($WEFT_BIN version 2>/dev/null | head -1) vs $PYTHON) =="
echo "host: $(uname -srm)  go: $(go env GOOS)/$(go env GOARCH)"
echo

pairs=(
  "testdata/bench/json_roundtrip.weft testdata/bench/json_roundtrip.py"
  "testdata/bench/seq_map.weft testdata/bench/seq_map.py"
  "testdata/bench/str_split_join.weft testdata/bench/str_split_join.py"
  "testdata/bench/fib.weft testdata/bench/fib.py"
)

for pair in "${pairs[@]}"; do
  # shellcheck disable=SC2086
  set -- $pair
  w=$1
  p=$2
  name=$(basename "$w" .weft)
  echo "--- $name ---"
  verify_pair "$w" "$p"
  # Warm once (disk/cache)
  "$WEFT_BIN" run "$w" >/dev/null
  "$PYTHON" "$p" >/dev/null
  run_timed "weft" "$WEFT_BIN" run "$w"
  run_timed "python3" "$PYTHON" "$p"
  if command -v luajit >/dev/null 2>&1 && [[ -f "testdata/bench/${name}.lua" ]]; then
    run_timed "luajit" luajit "testdata/bench/${name}.lua"
  fi
  echo
done

echo "Notes:"
echo "  - Wall times; not statistical (use hyperfine for CI-quality). Suggested:"
echo "      hyperfine -w 2 -r 5 './weft run testdata/bench/json_roundtrip.weft' 'python3 testdata/bench/json_roundtrip.py'"
echo "  - fib is pure recursion (not glue). Expect Weft slower than CPython here."
echo "  - JSON/string loops are closer to agent/ops workloads."
echo "  - Go unit benches: make bench"
