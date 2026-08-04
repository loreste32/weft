#!/usr/bin/env python3
"""Generate an honest Warp / DataFrame / ML capability matrix.

Scans package weft.json exports and a hand-maintained claim table so status
stays conservative: implemented / partial / unsupported. Writes
reports/capability-matrix.md (and optional JSON).

Usage:
  python3 scripts/capability-matrix.py
  python3 scripts/capability-matrix.py --json reports/capability-matrix.json
"""

from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def load_exports(pkg: str) -> list[str]:
    path = ROOT / "packages" / pkg / "weft.json"
    if not path.is_file():
        return []
    data = json.loads(path.read_text(encoding="utf-8"))
    exports = data.get("exports") or []
    return list(exports)


# Hand-maintained honesty table. Keys are claim labels (not necessarily export
# names). Status values: implemented | partial | unsupported.
#
# "implemented" = present, tested at smoke/semantics level for the claim.
# "partial"     = present but missing options, edge cases, scale, or parity.
# "unsupported" = not implemented / not claimed for replacement.
CLAIMS: list[dict] = [
    # ── Warp (NumPy-style arrays) ──────────────────────────────────────
    {"area": "warp", "claim": "array creation (array/zeros/ones/arange)", "status": "implemented", "notes": "Flat list + shape; typed constructors"},
    {"area": "warp", "claim": "fixed-width dtypes (bool/int/uint/float/object)", "status": "implemented", "notes": "Packed host dtypes with range validation; not full NumPy casting table or complex/structured dtypes"},
    {"area": "warp", "claim": "host packed tensor storage (_tid)", "status": "implemented", "notes": "Primary numeric storage via internal/tensor"},
    {"area": "warp", "claim": "elementwise arithmetic + broadcasting", "status": "implemented", "notes": "Trailing broadcast; shape mismatch → Err"},
    {"area": "warp", "claim": "reductions (sum/mean/min/max/axis)", "status": "implemented", "notes": "Axis opts partial (keepdims yes; where/initial/out no)"},
    {"area": "warp", "claim": "matmul / dot (1D/2D forms)", "status": "implemented", "notes": "CPU; LU inv/solve O(n³)"},
    {"area": "warp", "claim": "strided views / transpose_view / advanced index", "status": "partial", "notes": "Views + index API expanding; not full NumPy indexing"},
    {"area": "warp", "claim": "1D FFT / IFFT / fft_freq", "status": "partial", "notes": "1D only; power-of-2 Cooley–Tukey or naive; not multi-dim/sparse/masked"},
    {"area": "warp", "claim": "sparse / masked arrays", "status": "unsupported", "notes": "Not implemented"},
    {"area": "warp", "claim": "native accelerator dispatch (load/run/tensor)", "status": "implemented", "notes": "Explicit path + capability; no silent load"},
    {"area": "warp", "claim": "CUDA / ROCm / MLX automatic kernels", "status": "partial", "notes": "Vendor providers expose bounded float32 matmul + same-shape tensor_add; require explicit plugin path + hardware jobs"},
    {"area": "warp", "claim": "complete NumPy API replacement", "status": "unsupported", "notes": "Experimental surface; not binary-compatible NumPy"},
    # ── DataFrame (pandas-inspired) ────────────────────────────────────
    {"area": "dataframe", "claim": "from_rows / from_columns / CSV/JSON I/O", "status": "implemented", "notes": "Row-list storage; quoted CSV"},
    {"area": "dataframe", "claim": "filter / query / sort / head/tail/iloc", "status": "implemented", "notes": "iloc scalar/list; not full .loc label engine"},
    {"area": "dataframe", "claim": "groupby + aggregations + transform/size", "status": "partial", "notes": "group_by, group_by_transform, group_by_size; not full pandas groupby API"},
    {"area": "dataframe", "claim": "join / merge / concat", "status": "implemented", "notes": "Common how-modes; not full multi-key parity"},
    {"area": "dataframe", "claim": "DataFrame ↔ Warp numeric interchange", "status": "partial", "notes": "Tested 1D/2D copying path; rejects null/non-numeric values; zero-copy not claimed"},
    {"area": "dataframe", "claim": "Series + explicit index / MultiIndex foundation", "status": "partial", "notes": "Series helpers + multi-level foundation; not complete MultiIndex"},
    {"area": "dataframe", "claim": "pivot / melt / rolling / expanding", "status": "implemented", "notes": "Present; window ops limited vs pandas"},
    {"area": "dataframe", "claim": "nullable / categorical / datetime dtypes", "status": "unsupported", "notes": "Null-aware stats only; no extension dtypes"},
    {"area": "dataframe", "claim": "Arrow / Parquet / columnar backend", "status": "unsupported", "notes": "Row-list only"},
    {"area": "dataframe", "claim": "100k+ row ETL scale", "status": "partial", "notes": "Scale smoke exists; not memory-optimized for multi-GB"},
    {"area": "dataframe", "claim": "complete pandas API replacement", "status": "unsupported", "notes": "Experimental; deliberate subset"},
    # ── ML ─────────────────────────────────────────────────────────────
    {"area": "ml", "claim": "vectors (dot/cosine/norm/topk)", "status": "implemented", "notes": "Pure Weft"},
    {"area": "ml", "claim": "embeddings + local index (RAG helpers)", "status": "partial", "notes": "Provider-backed embed; needs network/keys"},
    {"area": "ml", "claim": "classical linear / logistic fit + score", "status": "implemented", "notes": "CPU minibatch; 100k-row train tested; accepts nested lists and packed Warp inputs"},
    {"area": "ml", "claim": "reverse-mode autodiff (scalars + warp)", "status": "implemented", "notes": "Tape ops; not full framework"},
    {"area": "ml", "claim": "SGD / Adam optimizers", "status": "implemented", "notes": "Scalar + Warp parameters"},
    {"area": "ml", "claim": "modules (linear/relu/sequential) + checkpoints", "status": "implemented", "notes": "Advisory; not PyTorch parity"},
    {"area": "ml", "claim": "higher-order grads (create_graph / gradcheck / hvp)", "status": "partial", "notes": "Scalar nested reverse-mode create_graph; array VJPs numeric; HVP finite-diff"},
    {"area": "ml", "claim": "device placement (cpu/cuda/rocm/mlx)", "status": "partial", "notes": "Advisory tags; non-CPU → fallback:true, compute stays CPU"},
    {"area": "ml", "claim": "GPU training without external provider", "status": "unsupported", "notes": "No fake GPU; needs native accelerator plugin"},
    {"area": "ml", "claim": "ONNX / Triton local inference", "status": "partial", "notes": "Via mlinfer HTTP sidecars, not in-process"},
]


def status_for_export(name: str, area: str) -> str:
    """Heuristic for unlisted exports: present in weft.json → implemented surface."""
    return "implemented"


def build_matrix() -> dict:
    exports = {
        "warp": load_exports("warp"),
        "dataframe": load_exports("dataframe"),
        "ml": load_exports("ml"),
    }
    claim_rows = []
    for row in CLAIMS:
        claim_rows.append(dict(row))

    # Surface audit: exports not mentioned in notes are still discoverable.
    covered_tokens = set()
    for row in CLAIMS:
        for part in row["claim"].replace("/", " ").replace("(", " ").replace(")", " ").split():
            covered_tokens.add(part.lower())

    export_summary = {}
    for area, names in exports.items():
        export_summary[area] = {
            "count": len(names),
            "exports": names,
            "package_path": f"packages/{area}/weft.json",
        }

    counts = {"implemented": 0, "partial": 0, "unsupported": 0}
    for row in claim_rows:
        counts[row["status"]] = counts.get(row["status"], 0) + 1

    return {
        "format": "weft.capability.matrix",
        "version": 1,
        "generated_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "honesty": (
            "Statuses are conservative. implemented ≠ complete NumPy/pandas/ML framework "
            "parity. See docs/COMPATIBILITY.md."
        ),
        "status_vocabulary": {
            "implemented": "Present and covered at smoke/semantics level for the claim",
            "partial": "Present but missing options, edge cases, scale, or backend parity",
            "unsupported": "Not implemented or deliberately out of scope for replacement claims",
        },
        "export_counts": {k: v["count"] for k, v in export_summary.items()},
        "exports": export_summary,
        "claims": claim_rows,
        "counts": counts,
    }


def render_md(matrix: dict) -> str:
    lines = [
        "# Weft capability matrix",
        "",
        f"- **Generated:** {matrix.get('generated_at', 'unknown')}",
        f"- **Format:** `{matrix.get('format')}` v{matrix.get('version')}",
        f"- **Honesty:** {matrix.get('honesty')}",
        "",
        "## Status vocabulary",
        "",
        "| Status | Meaning |",
        "|--------|---------|",
    ]
    for k, v in (matrix.get("status_vocabulary") or {}).items():
        lines.append(f"| `{k}` | {v} |")
    lines.append("")

    lines.append("## Export surface (from weft.json)")
    lines.append("")
    lines.append("| Package | Export count | Manifest |")
    lines.append("|---------|--------------|----------|")
    for area, info in (matrix.get("exports") or {}).items():
        lines.append(
            f"| `{area}` | {info.get('count', 0)} | `{info.get('package_path', '')}` |"
        )
    lines.append("")

    counts = matrix.get("counts") or {}
    lines.append("## Claim summary")
    lines.append("")
    lines.append(
        f"- implemented: **{counts.get('implemented', 0)}** · "
        f"partial: **{counts.get('partial', 0)}** · "
        f"unsupported: **{counts.get('unsupported', 0)}**"
    )
    lines.append("")

    by_area: dict[str, list] = {}
    for row in matrix.get("claims") or []:
        by_area.setdefault(row["area"], []).append(row)

    for area in ("warp", "dataframe", "ml"):
        rows = by_area.get(area, [])
        lines.append(f"## {area}")
        lines.append("")
        lines.append("| Claim | Status | Notes |")
        lines.append("|-------|--------|-------|")
        for row in rows:
            notes = (row.get("notes") or "").replace("|", "\\|")
            lines.append(
                f"| {row['claim']} | `{row['status']}` | {notes} |"
            )
        lines.append("")

    lines.append("## How to refresh")
    lines.append("")
    lines.append("```sh")
    lines.append("make capability-matrix")
    lines.append("# or: python3 scripts/capability-matrix.py")
    lines.append("```")
    lines.append("")
    lines.append(
        "Edit the hand-maintained `CLAIMS` table in `scripts/capability-matrix.py` "
        "when adding APIs or changing honesty status. Do not mark a claim "
        "`implemented` without tests or a documented smoke path."
    )
    lines.append("")
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "-o",
        "--output",
        default=str(ROOT / "reports" / "capability-matrix.md"),
        help="Markdown output path",
    )
    parser.add_argument(
        "--json",
        default="",
        help="Optional JSON output path",
    )
    args = parser.parse_args(argv)

    matrix = build_matrix()
    out = Path(args.output)
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(render_md(matrix), encoding="utf-8")
    print(out)

    if args.json:
        jpath = Path(args.json)
        jpath.parent.mkdir(parents=True, exist_ok=True)
        # Drop full export lists from default JSON for size; keep counts + claims.
        slim = dict(matrix)
        slim["exports"] = {
            k: {"count": v["count"], "package_path": v["package_path"]}
            for k, v in matrix["exports"].items()
        }
        jpath.write_text(json.dumps(slim, indent=2) + "\n", encoding="utf-8")
        print(jpath)

    counts = matrix["counts"]
    print(
        f"claims: implemented={counts.get('implemented', 0)} "
        f"partial={counts.get('partial', 0)} "
        f"unsupported={counts.get('unsupported', 0)}"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
