#!/usr/bin/env bash
# Generate and publish accelerator capability reports under reports/.
# Safe without GPUs: unavailable providers are recorded, not treated as failures.
#
# Usage:
#   scripts/publish-accelerator-report.sh
#
# Writes:
#   reports/accelerator-report.json
#   reports/accelerator-report.md
#
# Env (forwarded to accelerator-report.sh):
#   WEFT_BIN, WEFT_ACCEL_RUN_BENCH, WEFT_NUMERICAL_BENCH
set -euo pipefail
cd "$(dirname "$0")/.."

mkdir -p reports

JSON_OUT="reports/accelerator-report.json"
MD_OUT="reports/accelerator-report.md"
TMP_JSON=$(mktemp)
trap 'rm -f "$TMP_JSON"' EXIT

bash scripts/accelerator-report.sh "$TMP_JSON"
cp "$TMP_JSON" "$JSON_OUT"

python3 - "$JSON_OUT" "$MD_OUT" <<'PY'
import json, sys
from pathlib import Path

json_path, md_path = sys.argv[1], sys.argv[2]
with open(json_path, encoding="utf-8") as fh:
    report = json.load(fh)

host = report.get("host") or {}
providers = report.get("providers") or {}
bench = report.get("benchmarks") or {}
fallback = report.get("fallback_policy") or {}
trust = report.get("trust_model") or {}
fields = report.get("publish_fields") or {}

lines = []
lines.append("# Weft accelerator capability report")
lines.append("")
lines.append(f"- **Generated:** {report.get('generated_at', 'unknown')}")
lines.append(f"- **Format:** `{report.get('format', '')}` v{report.get('version', '?')}")
lines.append(
    f"- **Host:** {host.get('os', '?')} / {host.get('arch', '?')} "
    f"(Go: {host.get('go', 'n/a')}; Python: {host.get('python', 'n/a')})"
)
lines.append("")
lines.append("## Fallback policy")
lines.append("")
lines.append(
    f"- Silent fallback allowed: **{fallback.get('silent_fallback_allowed', False)}**"
)
for rule in fallback.get("rules") or []:
    lines.append(f"- {rule}")
lines.append("")
vocab = fallback.get("status_vocabulary") or {}
if vocab:
    lines.append("### Status vocabulary")
    lines.append("")
    lines.append("| Status | Meaning |")
    lines.append("|--------|---------|")
    for k, v in vocab.items():
        lines.append(f"| `{k}` | {v} |")
    lines.append("")

lines.append("## Providers")
lines.append("")
lines.append("| Provider | Status | Conformance | Tool / toolkit | Detail |")
lines.append("|----------|--------|-------------|----------------|--------|")
for name, info in providers.items():
    if not isinstance(info, dict):
        continue
    status = info.get("status", "")
    conf = info.get("conformance", "")
    tool = info.get("tool") or ""
    toolkit = info.get("driver_toolkit") or info.get("version") or ""
    tool_cell = f"{tool}"
    if toolkit:
        tool_cell = f"{tool} · {toolkit}" if tool else toolkit
    detail = info.get("detail") or info.get("note") or ""
    # collapse whitespace for table cells
    detail = " ".join(str(detail).split())
    if len(detail) > 120:
        detail = detail[:117] + "..."
    lines.append(
        f"| `{name}` | `{status}` | `{conf}` | {tool_cell or '—'} | {detail or '—'} |"
    )
lines.append("")

lines.append("## Trust model (summary)")
lines.append("")
lines.append("| Control | Environment variable |")
lines.append("|---------|----------------------|")
lines.append(f"| Hard disable | `{trust.get('disable_env', 'WEFT_ACCELERATOR_DISABLE')}` |")
lines.append(f"| Path allowlist | `{trust.get('allowlist_env', 'WEFT_ACCELERATOR_ALLOWLIST')}` |")
lines.append(
    f"| Require checksum | `{trust.get('require_checksum_env', 'WEFT_ACCELERATOR_REQUIRE_CHECKSUM')}` |"
)
lines.append(f"| Expected SHA-256 | `{trust.get('checksum_env', 'WEFT_ACCELERATOR_CHECKSUM')}` |")
lines.append("")
for note in trust.get("notes") or []:
    lines.append(f"- {note}")
lines.append("")

lines.append("## Benchmarks")
lines.append("")
lines.append(f"- **Status:** `{bench.get('status', 'not_run')}`")
if bench.get("note"):
    lines.append(f"- **Note:** {bench['note']}")
if bench.get("script"):
    lines.append(f"- **Script:** `{bench['script']}`")
numerical = bench.get("numerical")
if isinstance(numerical, dict):
    results = numerical.get("results") or []
    if results:
        lines.append("")
        lines.append("| Benchmark | Wall time (ms) |")
        lines.append("|-----------|----------------|")
        for item in results:
            lines.append(
                f"| `{item.get('name', '?')}` | {item.get('wall_ms', '?')} |"
            )
    nh = numerical.get("host") or {}
    if nh:
        lines.append("")
        lines.append(
            f"Numerical bench host: {nh.get('os', '?')} / {nh.get('arch', '?')}"
        )
lines.append("")

req = fields.get("required_for_release_claim") or []
if req:
    lines.append("## Required fields for per-release hardware claims")
    lines.append("")
    lines.append(
        "When publishing vendor hardware results for a release, record every field below "
        "(see [docs/ACCELERATORS.md](../docs/ACCELERATORS.md)):"
    )
    lines.append("")
    for f in req:
        lines.append(f"- `{f}`")
    lines.append("")

lines.append("## Artifacts")
lines.append("")
lines.append(f"- Machine-readable: `{json_path}`")
lines.append(f"- This summary: `{md_path}`")
lines.append("")
lines.append(
    "Hosts without GPUs report `unavailable` for vendor providers; that is expected and is not a CI failure."
)
lines.append("")

Path(md_path).write_text("\n".join(lines), encoding="utf-8")
print(md_path)
PY

echo "wrote $JSON_OUT"
echo "wrote $MD_OUT"
