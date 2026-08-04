#!/usr/bin/env bash
# Reproducible CPU numerical / dataframe micro-benchmarks (wall time).
set -euo pipefail
cd "$(dirname "$0")/.."

WEFT_BIN="${WEFT_BIN:-}"
if [[ -z "$WEFT_BIN" ]]; then
  go build -o /tmp/weft-num-bench ./cmd/weft
  WEFT_BIN=/tmp/weft-num-bench
fi

OUT="${1:-/tmp/weft-numerical-bench.json}"
mkdir -p testdata/bench

# Ensure fixtures exist (idempotent write).
cat > testdata/bench/warp_matmul.weft <<'EOF'
use "../../packages/warp/lib.weft" as warp

fn main {
    n := 64
    let mut data = []
    let mut i = 0
    while i < n * n {
        data = push(data, (i % 7) + 0.0)
        i = i + 1
    }
    a := warp.array(data, [n, n])
    b := warp.array(data, [n, n])
    c := warp.matmul(a, b)
    say({"n": n, "storage": warp.storage_kind(a), "checksum": warp.sum(c)})
}
EOF

cat > testdata/bench/dataframe_groupby.weft <<'EOF'
use "../../packages/dataframe/lib.weft" as df

fn main {
    let mut rows = []
    let mut i = 0
    while i < 5000 {
        rows = push(rows, {
            "dept": if i % 3 == 0 { "eng" } else if i % 3 == 1 { "sales" } else { "ops" },
            "salary": 50000 + (i % 1000),
        })
        i = i + 1
    }
    t := df.from_rows(rows)
    g := df.group_by(t, "dept", {"salary_sum": {"col": "salary", "op": "sum"}, "count": {"col": "salary", "op": "count"}})
    say({"rows": 5000, "groups": df.nrows(g)})
}
EOF

bench_one() {
  local name=$1
  local file=$2
  local start end ms
  start=$(python3 - <<'PY'
import time; print(time.perf_counter())
PY
)
  "$WEFT_BIN" run "$file" >/tmp/weft-bench-"$name".out
  end=$(python3 - <<'PY'
import time; print(time.perf_counter())
PY
)
  ms=$(python3 - <<PY
start=float("$start"); end=float("$end"); print(int((end-start)*1000))
PY
)
  echo "$name $ms"
}

results_file=$(mktemp)
{
  bench_one warp_matmul testdata/bench/warp_matmul.weft
  bench_one dataframe_groupby testdata/bench/dataframe_groupby.weft
} >"$results_file"

python3 - "$OUT" "$results_file" <<'PY'
import json, sys, platform
out = sys.argv[1]
results_path = sys.argv[2]
items = []
with open(results_path, encoding="utf-8") as fh:
    for line in fh:
        line = line.strip()
        if not line:
            continue
        name, ms = line.split()
        items.append({"name": name, "wall_ms": int(ms)})
report = {
    "format": "weft.numerical.bench",
    "version": 1,
    "host": {"os": platform.system(), "arch": platform.machine()},
    "results": items,
}
with open(out, "w", encoding="utf-8") as fh:
    json.dump(report, fh, indent=2)
    fh.write("\n")
print(out)
for item in items:
    print(f"{item['name']}: {item['wall_ms']} ms")
PY
rm -f "$results_file"
