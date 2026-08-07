#!/usr/bin/env bash
# Scale-budget smoke for warp + dataframe fixtures.
# Soft budgets warn by default; hard-fail only when WEFT_SCALE_STRICT=1.
#
#   bash scripts/bench-scale.sh
#   bash scripts/bench-scale.sh reports/scale-bench.json
#   WEFT_BIN=./weft WEFT_SCALE_STRICT=1 bash scripts/bench-scale.sh
#
# Budgets (soft unless WEFT_SCALE_STRICT=1): see testdata/bench/README.md
set -euo pipefail
cd "$(dirname "$0")/.."

WEFT_BIN="${WEFT_BIN:-}"
if [[ -z "$WEFT_BIN" ]]; then
  if [[ -x ./weft ]]; then
    WEFT_BIN=./weft
  elif [[ -x /Users/loreste/weft/weft ]]; then
    WEFT_BIN=/Users/loreste/weft/weft
  else
    go build -o /tmp/weft-scale-bench ./cmd/weft
    WEFT_BIN=/tmp/weft-scale-bench
  fi
fi

TIMEOUT_SEC="${TIMEOUT_SEC:-300}"
OUT="${1:-reports/scale-bench.json}"
mkdir -p "$(dirname "$OUT")"
STRICT="${WEFT_SCALE_STRICT:-0}"
PYTHON="${PYTHON:-python3}"

# Soft budgets (ms). Exceed → warn; with WEFT_SCALE_STRICT=1 → fail.
# peak_rss_kb budgets are advisory when measurable (macOS /usr/bin/time -l).
BUDGET_WARP_MATMUL_MS="${BUDGET_WARP_MATMUL_MS:-120000}"
BUDGET_WARP_ELEM_MS="${BUDGET_WARP_ELEM_MS:-60000}"
BUDGET_DF_100K_MS="${BUDGET_DF_100K_MS:-120000}"
BUDGET_DF_250K_MS="${BUDGET_DF_250K_MS:-300000}"
BUDGET_DF_WIDE_MS="${BUDGET_DF_WIDE_MS:-120000}"
BUDGET_DF_JOIN_MS="${BUDGET_DF_JOIN_MS:-120000}"
BUDGET_DF_1M_MS="${BUDGET_DF_1M_MS:-1200000}"
BUDGET_PEAK_RSS_KB="${BUDGET_PEAK_RSS_KB:-4194304}" # 4 GiB advisory

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
RESULTS_JSONL="${tmpdir}/results.jsonl"
: >"$RESULTS_JSONL"

now_s() {
  if command -v "$PYTHON" >/dev/null 2>&1; then
    "$PYTHON" -c 'import time; print(time.perf_counter())'
  else
    date +%s
  fi
}

run_with_timeout() {
  if command -v timeout >/dev/null 2>&1; then
    timeout "${TIMEOUT_SEC}s" "$@"
  elif command -v gtimeout >/dev/null 2>&1; then
    gtimeout "${TIMEOUT_SEC}s" "$@"
  else
    "$@"
  fi
}

# Run one fixture; sets globals: last_ms last_peak_kb last_payload last_status
run_fixture() {
  local fixture=$1
  local run_out=$2
  local time_err=$3
  local peak_kb=""
  local start end status

  start=$(now_s)
  set +e
  if [[ -x /usr/bin/time ]]; then
    if /usr/bin/time -l true >/dev/null 2>&1; then
      run_with_timeout /usr/bin/time -l "$WEFT_BIN" run "$fixture" >"$run_out" 2>"$time_err"
      status=$?
      peak_kb=$(awk '/maximum resident set size/ { print int($1/1024); exit }' "$time_err" 2>/dev/null || true)
      if [[ -z "$peak_kb" ]]; then
        peak_kb=$(awk '/maximum resident set size/ { print $1; exit }' "$time_err" 2>/dev/null || true)
      fi
    elif /usr/bin/time -v true >/dev/null 2>&1; then
      run_with_timeout /usr/bin/time -v "$WEFT_BIN" run "$fixture" >"$run_out" 2>"$time_err"
      status=$?
      peak_kb=$(awk -F: '/Maximum resident set size/ { gsub(/ /,"",$2); print $2; exit }' "$time_err" 2>/dev/null || true)
    else
      run_with_timeout "$WEFT_BIN" run "$fixture" >"$run_out" 2>"$time_err"
      status=$?
    fi
  else
    run_with_timeout "$WEFT_BIN" run "$fixture" >"$run_out" 2>"$time_err"
    status=$?
  fi
  set -e
  end=$(now_s)

  if command -v "$PYTHON" >/dev/null 2>&1; then
    last_ms=$("$PYTHON" -c "print(int((float('$end')-float('$start'))*1000))")
  else
    last_ms=$(( (end - start) * 1000 ))
  fi
  last_peak_kb="$peak_kb"
  last_payload=$(cat "$run_out" 2>/dev/null || echo "")
  last_status=$status
}

overall_ok=1
budget_warnings=0
budget_failures=0

append_result() {
  local name=$1
  local fixture=$2
  local budget_ms=$3
  local ok=$4
  local warn=$5
  local note=$6
  if ! command -v "$PYTHON" >/dev/null 2>&1; then
    echo "{\"name\":\"$name\",\"wall_ms\":$last_ms,\"ok\":$ok}" >>"$RESULTS_JSONL"
    return
  fi
  "$PYTHON" - "$RESULTS_JSONL" "$name" "$fixture" "$last_ms" "$last_peak_kb" \
    "$budget_ms" "$ok" "$warn" "$note" "$last_payload" <<'PY'
import json, sys
path, name, fixture, ms, peak_kb, budget_ms, ok, warn, note, payload = sys.argv[1:11]
try:
    body = json.loads(payload) if payload.strip() else {}
except Exception:
    body = {"raw": payload.strip()}
peak = None
if peak_kb not in ("", "None"):
    try:
        peak = int(float(peak_kb))
    except Exception:
        peak = None
row = {
    "name": name,
    "fixture": fixture,
    "wall_ms": int(ms),
    "peak_rss_kb": peak,
    "budget_ms": int(budget_ms),
    "within_budget": ok == "1",
    "budget_warning": warn == "1",
    "note": note,
    "result": body,
}
with open(path, "a", encoding="utf-8") as fh:
    fh.write(json.dumps(row, separators=(",", ":")) + "\n")
print(f"{name}: {ms} ms (budget {budget_ms})" + (f"  peak_rss_kb={peak}" if peak is not None else ""))
PY
}

run_named() {
  local name=$1
  local fixture=$2
  local budget_ms=$3
  local note=$4
  local run_out="${tmpdir}/${name}.out"
  local time_err="${tmpdir}/${name}.time"

  if [[ ! -f "$fixture" ]]; then
    echo "missing fixture: $fixture" >&2
    overall_ok=0
    append_result "$name" "$fixture" "$budget_ms" 0 0 "missing fixture"
    return
  fi

  echo "→ $name ($fixture)"
  run_fixture "$fixture" "$run_out" "$time_err"

  if [[ $last_status -ne 0 ]]; then
    echo "  FAILED exit=$last_status" >&2
    cat "$run_out" >&2 || true
    cat "$time_err" >&2 || true
    overall_ok=0
    append_result "$name" "$fixture" "$budget_ms" 0 0 "run failed exit=$last_status"
    return
  fi

  local within=1
  local warn=0
  if [[ "$last_ms" -gt "$budget_ms" ]]; then
    within=0
    warn=1
    budget_warnings=$((budget_warnings + 1))
    echo "  WARN: wall_ms=$last_ms exceeded soft budget ${budget_ms}ms" >&2
    if [[ "$STRICT" == "1" ]]; then
      budget_failures=$((budget_failures + 1))
      overall_ok=0
      echo "  STRICT: treating budget miss as failure" >&2
    fi
  fi
  if [[ -n "${last_peak_kb:-}" && "$last_peak_kb" != "" ]]; then
    if [[ "$last_peak_kb" -gt "$BUDGET_PEAK_RSS_KB" ]] 2>/dev/null; then
      warn=1
      budget_warnings=$((budget_warnings + 1))
      echo "  WARN: peak_rss_kb=$last_peak_kb exceeded advisory ${BUDGET_PEAK_RSS_KB}" >&2
      if [[ "$STRICT" == "1" ]]; then
        budget_failures=$((budget_failures + 1))
        overall_ok=0
      fi
    fi
  fi
  append_result "$name" "$fixture" "$budget_ms" "$within" "$warn" "$note"
}

# Fixtures: warp matmul (default 256), optional elementwise, dataframe 100k + 250k
run_named "warp_matmul" "testdata/bench/warp_scale.weft" "$BUDGET_WARP_MATMUL_MS" \
  "default WEFT_WARP_N=256 matmul"

# Elementwise sum path (same fixture, different mode)
export WEFT_WARP_MODE=elementwise
run_named "warp_elementwise" "testdata/bench/warp_scale.weft" "$BUDGET_WARP_ELEM_MS" \
  "default WEFT_WARP_ELEMS=100000 sum"
unset WEFT_WARP_MODE

run_named "dataframe_100k" "testdata/bench/dataframe_scale.weft" "$BUDGET_DF_100K_MS" \
  "fixed ~100k groupby+sort"

# 250k default for dataframe_scale_1m (1M optional via WEFT_DF_ROWS)
run_named "dataframe_250k" "testdata/bench/dataframe_scale_1m.weft" "$BUDGET_DF_250K_MS" \
  "default 250k; WEFT_DF_ROWS=1000000 for full million"

run_named "dataframe_wide_100k" "testdata/bench/dataframe_scale_wide.weft" "$BUDGET_DF_WIDE_MS" \
  "100k rows x 13 cols, derivations + groupby"

run_named "dataframe_join_100k" "testdata/bench/dataframe_scale_join.weft" "$BUDGET_DF_JOIN_MS" \
  "100k x 20k inner join + groupby"

# Opt-in heavy tier (WEFT_SCALE_BIG=1): full 1M-row run. 10M rows is
# aspirational pending columnar storage (row-list frames grow linearly in
# memory) — see testdata/bench/README.md.
if [[ "${WEFT_SCALE_BIG:-0}" == "1" ]]; then
  export WEFT_DF_ROWS=1000000
  run_named "dataframe_1m" "testdata/bench/dataframe_scale_1m.weft" "$BUDGET_DF_1M_MS" \
    "opt-in WEFT_SCALE_BIG=1 full million rows"
  unset WEFT_DF_ROWS
fi

# Write final report
if command -v "$PYTHON" >/dev/null 2>&1; then
  "$PYTHON" - "$OUT" "$RESULTS_JSONL" "$STRICT" "$overall_ok" "$budget_warnings" "$budget_failures" \
    "$BUDGET_WARP_MATMUL_MS" "$BUDGET_WARP_ELEM_MS" "$BUDGET_DF_100K_MS" "$BUDGET_DF_250K_MS" "$BUDGET_PEAK_RSS_KB" \
    "$BUDGET_DF_WIDE_MS" "$BUDGET_DF_JOIN_MS" <<'PY'
import json, platform, sys
from datetime import datetime, timezone

(
    out, results_path, strict, overall_ok, warnings, failures,
    b_warp_m, b_warp_e, b_df100, b_df250, b_rss, b_dfwide, b_dfjoin,
) = sys.argv[1:14]

rows = []
with open(results_path, encoding="utf-8") as fh:
    for line in fh:
        line = line.strip()
        if line:
            rows.append(json.loads(line))

report = {
    "format": "weft.scale.bench",
    "version": 2,
    "generated_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "host": {"os": platform.system(), "arch": platform.machine()},
    "strict": strict == "1",
    "budgets_ms": {
        "warp_matmul": int(b_warp_m),
        "warp_elementwise": int(b_warp_e),
        "dataframe_100k": int(b_df100),
        "dataframe_250k": int(b_df250),
        "dataframe_wide_100k": int(b_dfwide),
        "dataframe_join_100k": int(b_dfjoin),
    },
    "budgets_peak_rss_kb": int(b_rss),
    "budget_policy": {
        "soft_by_default": True,
        "strict_env": "WEFT_SCALE_STRICT",
        "note": "Soft budgets warn; only WEFT_SCALE_STRICT=1 fails on budget miss. Run failures always fail.",
    },
    "summary": {
        "ok": overall_ok == "1",
        "budget_warnings": int(warnings),
        "budget_failures": int(failures),
        "fixtures": len(rows),
    },
    "results": rows,
}
with open(out, "w", encoding="utf-8") as fh:
    json.dump(report, fh, indent=2)
    fh.write("\n")
print(out)
print(json.dumps(report["summary"], separators=(",", ":")))
PY
else
  echo "{\"format\":\"weft.scale.bench\",\"version\":2,\"results_raw\":true}" >"$OUT"
fi

echo "wrote $OUT"
if [[ "$overall_ok" -ne 1 ]]; then
  echo "bench-scale: FAILED (run error or strict budget miss)" >&2
  exit 1
fi
if [[ "$budget_warnings" -gt 0 ]]; then
  echo "bench-scale: ok with $budget_warnings budget warning(s) (soft)"
else
  echo "bench-scale: ok"
fi
exit 0
