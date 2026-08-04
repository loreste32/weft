#!/usr/bin/env bash
# Produce a machine-readable hardware/provider report.
# Safe without GPUs: reports unavailable providers instead of failing the build.
set -euo pipefail
cd "$(dirname "$0")/.."

OUT="${1:-/tmp/weft-accelerator-report.json}"
WEFT_BIN="${WEFT_BIN:-./weft}"
if [[ ! -x "$WEFT_BIN" ]]; then
  go build -o /tmp/weft-accel-report ./cmd/weft
  WEFT_BIN=/tmp/weft-accel-report
fi

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

# Portable CPU provider compile (optional).
cpu_status="unavailable"
cpu_detail="c compiler missing"
cpu_path=""
if command -v cc >/dev/null 2>&1; then
  if [[ "$(uname -s)" == "Darwin" ]]; then
    cpu_path="${tmpdir}/weft_accel_cpu.dylib"
    if cc -std=c11 -dynamiclib -fPIC -I native/accelerator \
      native/accelerator/example.c -o "$cpu_path" 2>"${tmpdir}/cpu_build.err"; then
      cpu_status="built"
      cpu_detail="ok"
    else
      cpu_status="build_failed"
      cpu_detail=$(tr '\n' ' ' <"${tmpdir}/cpu_build.err" | head -c 400)
    fi
  else
    cpu_path="${tmpdir}/weft_accel_cpu.so"
    if cc -std=c11 -shared -fPIC -I native/accelerator \
      native/accelerator/example.c -o "$cpu_path" 2>"${tmpdir}/cpu_build.err"; then
      cpu_status="built"
      cpu_detail="ok"
    else
      cpu_status="build_failed"
      cpu_detail=$(tr '\n' ' ' <"${tmpdir}/cpu_build.err" | head -c 400)
    fi
  fi
fi

cpu_test="skipped"
if [[ "$cpu_status" == "built" ]]; then
  if WEFT_ACCELERATOR_PLUGIN="$cpu_path" go test ./internal/accelerator -run '^TestExternalProvider(Tensor)?$' -count=1 -timeout=60s >"${tmpdir}/cpu_test.out" 2>&1; then
    cpu_test="passed"
  else
    cpu_test="failed"
    cpu_detail=$(tr '\n' ' ' <"${tmpdir}/cpu_test.out" | head -c 400)
  fi
fi

python3 - "$OUT" "$cpu_status" "$cpu_test" "$cpu_detail" <<'PY'
import json, os, platform, sys, shutil, subprocess
out, cpu_status, cpu_test, cpu_detail = sys.argv[1:5]

def which(name):
    return shutil.which(name)

def run(cmd):
    try:
        p = subprocess.run(cmd, check=False, capture_output=True, text=True, timeout=10)
        text = (p.stdout or p.stderr or "").strip()
        return text.splitlines()[0] if text else ""
    except Exception as exc:
        return f"error: {exc}"

report = {
    "format": "weft.accelerator.report",
    "version": 1,
    "host": {
        "os": platform.system(),
        "arch": platform.machine(),
        "python": platform.python_version(),
        "go": run(["go", "version"]),
    },
    "trust_model": {
        "disable_env": "WEFT_ACCELERATOR_DISABLE",
        "allowlist_env": "WEFT_ACCELERATOR_ALLOWLIST",
        "require_checksum_env": "WEFT_ACCELERATOR_REQUIRE_CHECKSUM",
        "checksum_env": "WEFT_ACCELERATOR_CHECKSUM",
        "notes": [
            "Native providers are trusted host code and bypass the language sandbox.",
            "Registry packages cannot silently activate plugins; loads require an explicit path.",
            "Prefer allowlist + checksum verification in production.",
        ],
    },
    "providers": {
        "cpu_reference": {
            "status": cpu_status,
            "conformance": cpu_test,
            "detail": cpu_detail,
            "tool": which("cc"),
        },
        "cuda": {
            "status": "available" if which("nvcc") else "unavailable",
            "tool": which("nvcc"),
            "version": run(["nvcc", "--version"]) if which("nvcc") else "",
            "conformance": "not_run",
            "note": "Requires self-hosted CUDA runner (.github/workflows/native-accelerators.yml)",
        },
        "rocm": {
            "status": "available" if which("hipcc") else "unavailable",
            "tool": which("hipcc"),
            "version": run(["hipcc", "--version"]) if which("hipcc") else "",
            "conformance": "not_run",
            "note": "Requires self-hosted ROCm runner",
        },
        "mlx": {
            "status": "configured" if os.environ.get("MLX_C_PREFIX") else "unavailable",
            "tool": os.environ.get("MLX_C_PREFIX", ""),
            "conformance": "not_run",
            "note": "Requires self-hosted Apple Silicon runner with mlx-c",
        },
    },
    "benchmarks": {
        "status": "see scripts/bench-numerical.sh",
        "note": "CPU numerical benchmarks are separate from provider hardware jobs",
    },
}
with open(out, "w", encoding="utf-8") as fh:
    json.dump(report, fh, indent=2)
    fh.write("\n")
print(out)
PY

echo "wrote $OUT"
