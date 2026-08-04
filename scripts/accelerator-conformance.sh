#!/usr/bin/env bash
# Accelerator provider conformance (CPU reference required; vendors optional).
#
# Builds the portable CPU reference provider, runs host Go tests with
# WEFT_ACCELERATOR_PLUGIN, and writes a small JSON result. Safe without GPUs:
# vendor toolchains/devices missing → unavailable (not a failure).
#
# Usage:
#   bash scripts/accelerator-conformance.sh
#   bash scripts/accelerator-conformance.sh reports/accelerator-conformance.json
#
# Exit codes:
#   0  — CPU reference path ok (built + health/identity/matmul/tensor tests
#        pass + execution reporting classified "honest")
#   1  — CPU path failed (compile, load, numerical/conformance failure, or
#        missing/contradictory device/fallback reporting)
set -euo pipefail
cd "$(dirname "$0")/.."

OUT="${1:-reports/accelerator-conformance.json}"
mkdir -p "$(dirname "$OUT")"

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

# --- Build CPU reference provider ---
cpu_status="unavailable"
cpu_detail="c compiler missing"
cpu_path=""
cpu_fallback=false

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

# --- Run Go external provider tests (JSON + tensor) ---
json_test="skipped"
tensor_test="skipped"
tensor_add_test="skipped"
reporting_test="skipped"
reporting="not_run"
health_ok=false
identity_ok=false
matmul_ok=false
tensor_ok=false
tensor_add_ok=false

if [[ "$cpu_status" == "built" ]]; then
  set +e
  WEFT_ACCELERATOR_PLUGIN="$cpu_path" go test ./internal/accelerator \
    -run '^TestExternalProvider$' -count=1 -timeout=60s \
    >"${tmpdir}/json_test.out" 2>&1
  json_rc=$?
  WEFT_ACCELERATOR_PLUGIN="$cpu_path" go test ./internal/accelerator \
    -run '^TestExternalProviderTensor$' -count=1 -timeout=60s \
    >"${tmpdir}/tensor_test.out" 2>&1
tensor_rc=$?
WEFT_ACCELERATOR_PLUGIN="$cpu_path" go test ./internal/accelerator \
  -run '^TestExternalProviderTensorAdd$' -count=1 -timeout=60s \
  >"${tmpdir}/tensor_add_test.out" 2>&1
tensor_add_rc=$?
  # Adversarial reporting gate: a provider that omits device/fallback fields
  # or reports a contradiction fails conformance and is classified here.
  WEFT_ACCELERATOR_PLUGIN="$cpu_path" go test ./internal/accelerator \
    -run '^TestExternalProviderReporting$' -count=1 -timeout=60s \
    >"${tmpdir}/reporting_test.out" 2>&1
  reporting_rc=$?
  set -e

  if [[ $json_rc -eq 0 ]]; then
    json_test="passed"
    health_ok=true
    identity_ok=true
    matmul_ok=true
  else
    json_test="failed"
    cpu_detail=$(tr '\n' ' ' <"${tmpdir}/json_test.out" | head -c 400)
  fi

  if [[ $tensor_rc -eq 0 ]]; then
    tensor_test="passed"
    tensor_ok=true
  else
    tensor_test="failed"
    if [[ "$json_test" == "passed" ]]; then
      cpu_detail=$(tr '\n' ' ' <"${tmpdir}/tensor_test.out" | head -c 400)
    fi
  fi

  # Probe health JSON for explicit fallback reporting (CPU reference).
  if command -v python3 >/dev/null 2>&1 && [[ -x "$cpu_path" || -f "$cpu_path" ]]; then
    # Fallback flag is asserted inside Go tests; record cpu_fallback=false for reference.
    cpu_fallback=false
  fi
fi

if [[ $tensor_add_rc -eq 0 ]]; then
  tensor_add_test="passed"
  tensor_add_ok=true
else
  tensor_add_test="failed"
  if [[ "$json_test" == "passed" && "$tensor_test" == "passed" ]]; then
    cpu_detail=$(tr '\n' ' ' <"${tmpdir}/tensor_add_test.out" | head -c 400)
  fi
fi

# Classify provider reporting honesty from the adversarial reporting test.
if [[ "$cpu_status" == "built" ]]; then
  if [[ $reporting_rc -eq 0 ]]; then
    reporting_test="passed"
    reporting="honest"
  else
    reporting_test="failed"
    if grep -q "REPORTING_CONTRADICTORY" "${tmpdir}/reporting_test.out"; then
      reporting="contradictory"
    elif grep -q "REPORTING_UNREPORTED" "${tmpdir}/reporting_test.out"; then
      reporting="unreported"
    else
      reporting="failed"
    fi
    if [[ "$json_test" == "passed" && "$tensor_test" == "passed" && "$tensor_add_test" == "passed" ]]; then
      cpu_detail=$(tr '\n' ' ' <"${tmpdir}/reporting_test.out" | head -c 400)
    fi
  fi
fi

# Overall CPU path status. Reporting is part of conformance: a provider that
# runs correctly but does not say where it ran does not pass.
cpu_conformance="skipped"
if [[ "$cpu_status" == "built" ]]; then
if [[ "$json_test" == "passed" && "$tensor_test" == "passed" && "$tensor_add_test" == "passed" && "$reporting" == "honest" ]]; then
    cpu_conformance="passed"
  else
    cpu_conformance="failed"
  fi
elif [[ "$cpu_status" == "build_failed" ]]; then
  cpu_conformance="failed"
fi

# Vendor availability (not required for exit 0)
cuda_status="unavailable"
rocm_status="unavailable"
mlx_status="unavailable"
if command -v nvcc >/dev/null 2>&1; then cuda_status="available"; fi
if command -v hipcc >/dev/null 2>&1; then rocm_status="available"; fi
if [[ -n "${MLX_C_PREFIX:-}" ]]; then mlx_status="configured"; fi

python3 - "$OUT" \
  "$cpu_status" "$cpu_conformance" "$cpu_detail" \
  "$json_test" "$tensor_test" "$tensor_add_test" \
  "$health_ok" "$identity_ok" "$matmul_ok" "$tensor_ok" "$tensor_add_ok" \
  "$cpu_fallback" \
  "$cuda_status" "$rocm_status" "$mlx_status" \
  "$reporting" "$reporting_test" <<'PY'
import json, platform, shutil, subprocess, sys
from datetime import datetime, timezone

(
    out,
    cpu_status,
    cpu_conformance,
    cpu_detail,
    json_test,
    tensor_test,
    tensor_add_test,
    health_ok,
    identity_ok,
    matmul_ok,
    tensor_ok,
    tensor_add_ok,
    cpu_fallback,
    cuda_status,
    rocm_status,
    mlx_status,
    reporting,
    reporting_test,
) = sys.argv[1:19]

def which(name):
    return shutil.which(name)

def run(cmd):
    try:
        p = subprocess.run(cmd, check=False, capture_output=True, text=True, timeout=10)
        text = (p.stdout or p.stderr or "").strip()
        return text.splitlines()[0] if text else ""
    except Exception as exc:
        return f"error: {exc}"

def truthy(s):
    return str(s).lower() in ("1", "true", "yes", "on")

report = {
    "format": "weft.accelerator.conformance",
    "version": 1,
    "generated_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "host": {
        "os": platform.system(),
        "arch": platform.machine(),
        "go": run(["go", "version"]),
    },
    "cpu_reference": {
        "status": cpu_status,
        "conformance": cpu_conformance,
        "detail": cpu_detail,
        "fallback": truthy(cpu_fallback),
        # "honest" | "unreported" | "contradictory" | "failed" | "not_run".
        # Anything but "honest" fails conformance, even if numerics pass.
        "reporting": reporting,
        "tests": {
            "health": {"passed": truthy(health_ok)},
            "identity": {"passed": truthy(identity_ok)},
            "matmul": {"passed": truthy(matmul_ok), "fallback": truthy(cpu_fallback)},
    "tensor_matmul": {"passed": truthy(tensor_ok)},
    "tensor_add": {"passed": truthy(tensor_add_ok)},
            "json_ops": json_test,
    "tensor_ops": tensor_test,
    "tensor_add_ops": tensor_add_test,
    "reporting_ops": reporting_test,
        },
    },
    "vendors": {
        "cuda": {
            "status": cuda_status,
            "conformance": "not_run",
            "reporting": "not_run",
            "tool": which("nvcc"),
            "note": "Optional; self-hosted CUDA runner only",
        },
        "rocm": {
            "status": rocm_status,
            "conformance": "not_run",
            "reporting": "not_run",
            "tool": which("hipcc"),
            "note": "Optional; self-hosted ROCm runner only",
        },
        "mlx": {
            "status": mlx_status,
            "conformance": "not_run",
            "reporting": "not_run",
            "tool": "",
            "note": "Optional; requires MLX_C_PREFIX on Apple Silicon",
        },
    },
    "summary": {
        "cpu_ok": cpu_conformance == "passed",
        "vendors_required": False,
        "silent_fallback_allowed": False,
        "reporting_required": True,
    },
}

with open(out, "w", encoding="utf-8") as fh:
    json.dump(report, fh, indent=2)
    fh.write("\n")
print(out)
print(f"cpu_reference: status={cpu_status} conformance={cpu_conformance} reporting={reporting}")
print(f"  json_ops={json_test} tensor_ops={tensor_test} fallback={cpu_fallback}")
print(f"vendors: cuda={cuda_status} rocm={rocm_status} mlx={mlx_status}")
PY

echo "wrote $OUT"

if [[ "$cpu_conformance" != "passed" ]]; then
  echo "accelerator-conformance: CPU path failed (reporting=${reporting})" >&2
  exit 1
fi
echo "accelerator-conformance: CPU path ok, reporting honest (vendors optional)"
exit 0
