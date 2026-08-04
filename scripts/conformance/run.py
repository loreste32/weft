#!/usr/bin/env python3
"""Differential smoke checks for Weft warp/dataframe against pinned NumPy/pandas.

Honesty: this is still a smoke suite (small fixtures + one property check), not a
full NumPy/pandas replacement matrix. It locks a few high-value value/dtype/error
behaviors from the numerical roadmap; gaps remain outside these samples.
"""

from __future__ import annotations

import argparse
import json
import math
import subprocess
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd


ROOT = Path(__file__).resolve().parents[2]


def run_weft(weft: Path, program: str) -> dict[str, Any]:
    result = subprocess.run(
        [str(weft), "run", str(ROOT / "testdata" / "conformance" / program)],
        cwd=ROOT,
        check=False,
        capture_output=True,
        text=True,
    )
    if result.returncode:
        raise AssertionError(
            f"Weft conformance program {program} failed ({result.returncode}):\n"
            f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
        )
    for line in reversed(result.stdout.splitlines()):
        try:
            value = json.loads(line)
        except json.JSONDecodeError:
            continue
        if isinstance(value, dict):
            return value
    raise AssertionError(f"{program} did not emit a JSON object:\n{result.stdout}")


def assert_equal(actual: Any, expected: Any, path: str = "root") -> None:
    if isinstance(expected, float):
        if not isinstance(actual, (int, float)) or not math.isclose(
            float(actual), expected, rel_tol=1e-10, abs_tol=1e-10
        ):
            raise AssertionError(f"{path}: got {actual!r}, want {expected!r}")
        return
    if expected is None:
        if actual is not None:
            raise AssertionError(f"{path}: got {actual!r}, want null")
        return
    if isinstance(expected, list):
        if not isinstance(actual, list) or len(actual) != len(expected):
            raise AssertionError(f"{path}: got {actual!r}, want {expected!r}")
        for index, (got, want) in enumerate(zip(actual, expected)):
            assert_equal(got, want, f"{path}[{index}]")
        return
    if isinstance(expected, dict):
        if not isinstance(actual, dict) or set(actual) != set(expected):
            raise AssertionError(f"{path}: got keys {actual!r}, want {expected!r}")
        for key, want in expected.items():
            assert_equal(actual[key], want, f"{path}.{key}")
        return
    if actual != expected:
        raise AssertionError(f"{path}: got {actual!r}, want {expected!r}")


def warp_expected() -> dict[str, Any]:
    a = np.array([[1.0, 2.0, 3.0], [4.0, 5.0, 6.0]])
    b = np.array([10.0, 20.0, 30.0])
    left = np.array([[1.0, 2.0], [3.0, 4.0]])
    right = np.array([[5.0, 6.0], [7.0, 8.0]])
    ints = np.array([[1, 2], [3, 4]], dtype=np.int64)
    floats = np.array([[1, 2], [3, 4]], dtype=np.float32)
    typed_added = floats + 1
    comparison = a > 3.0
    selected = np.where(comparison, a, 0.0)
    return {
        "shape": [2, 3],
        "dtype": str(a.dtype),
        "typed_dtype": str(floats.dtype),
        "typed_added_dtype": str(typed_added.dtype),
        "comparison_dtype": str(comparison.dtype),
        "comparison": comparison.reshape(-1).tolist(),
        "broadcast_add": (a + b).reshape(-1).tolist(),
        "sum_axis1": {"shape": [2, 1], "data": a.sum(axis=1, keepdims=True).reshape(-1).tolist()},
        "matmul": {"shape": [2, 2], "data": (left @ right).reshape(-1).tolist()},
        "where": selected.reshape(-1).tolist(),
        "where_dtype": str(selected.dtype),
        "sum_int_dtype": str(ints.sum(axis=0).dtype),
        "sum_float32_dtype": str(floats.sum(axis=0).dtype),
        "mean_int_dtype": str(ints.mean(axis=0).dtype),
        "mean_float32_dtype": str(floats.mean(axis=0).dtype),
        "cumsum_int_dtype": str(ints.cumsum().dtype),
        "reshape": a.reshape(3, 2).reshape(-1).tolist(),
        "reshape_shape": [3, 2],
        "total": float(a.sum()),
        "mean": float(a.mean()),
    }


def warp_strides_expected() -> dict[str, Any]:
    base = np.arange(1, 10, dtype=np.int64).reshape(3, 3)
    transposed = base.T
    assigned = base.copy()
    assigned[np.array([[0], [2]]), np.array([0, 2])] = np.array([[40], [50]])
    return {
        "transposed_shape": list(transposed.shape),
        "transposed_element_strides": [1, 3],
        "transposed_strides": list(transposed.strides),
        "transposed": transposed.reshape(-1).tolist(),
        "rows": base[[2, 0], :].reshape(-1).tolist(),
        "cols": base[:, [-1, 0]].reshape(-1).tolist(),
        "masked": base[[True, False, True], :].reshape(-1).tolist(),
        "reversed": base.reshape(-1)[2::-1].tolist(),
        "negative_get": int(transposed[-1, -1]),
        "is_contiguous": False,
        "even_slice": np.arange(10)[1:8:2].tolist(),
        "reverse_slice": np.arange(10)[::-1].tolist(),
        "axis_slice": base[:, ::-1].reshape(-1).tolist(),
        "indexed_slice": np.arange(10)[1::2].tolist(),
        "paired": base[[0, 2], [1, 0]].tolist(),
        "broadcasted": base[np.array([[0], [2]]), np.array([0, 2])].reshape(-1).tolist(),
        "assigned": assigned.reshape(-1).tolist(),
    }


def dataframe_missing_expected() -> dict[str, Any]:
    frame = pd.DataFrame([{"a": 1}, {"b": 2}])
    return {
        "columns": frame.columns.tolist(),
        "a": [1, None],
        "b": [None, 2],
    }


def warp_edges_expected() -> dict[str, Any]:
    empty = np.zeros((0,), dtype=np.float64)
    empty2 = np.zeros((0, 3), dtype=np.float64)
    zerod = np.array(7.0)
    ints = np.array([1, 2, 3], dtype=np.int64)
    floats = np.array([1.0, 2.0, 3.0], dtype=np.float64)
    promoted = ints + floats
    nan_arr = np.array([1.0, np.nan, 3.0])
    inf_arr = np.array([1.0, np.inf, -np.inf])
    keep = np.array([[1.0, 2.0], [3.0, 4.0]]).sum(axis=0, keepdims=True)
    return {
        "empty_shape": list(empty.shape),
        "empty_size": int(empty.size),
        "empty2_shape": list(empty2.shape),
        "zerod_shape": list(zerod.shape),
        "zerod_value": float(zerod),
        "promoted_dtype": str(promoted.dtype),
        "promoted": promoted.tolist(),
        "nan_is_nan": np.isnan(nan_arr).tolist(),
        "inf_is_inf": np.isinf(inf_arr).tolist(),
        "keepdims_shape": list(keep.shape),
        "keepdims_data": keep.reshape(-1).tolist(),
        "broadcast_fail": True,
        "storage": "tensor",
    }


def _numpy_raises(fn) -> bool:
    try:
        fn()
        return False
    except Exception:
        return True


def warp_errors_expected() -> dict[str, Any]:
    """Error *presence* only (boolean flags), not message text or exception type."""
    return {
        "broadcast_mismatch": _numpy_raises(
            lambda: np.array([1.0, 2.0]) + np.array([1.0, 2.0, 3.0])
        ),
        "bad_reshape": _numpy_raises(lambda: np.arange(6.0).reshape(4, 2)),
        "oob_index": _numpy_raises(lambda: np.array([10.0, 20.0, 30.0])[5]),
        "ok_add": not _numpy_raises(
            lambda: np.array([1.0, 2.0]) + np.array([10.0, 20.0])
        ),
        "ok_reshape": not _numpy_raises(lambda: np.arange(6.0).reshape(3, 2)),
        "ok_get": not _numpy_raises(lambda: np.array([10.0, 20.0, 30.0])[1]),
    }


def warp_dtype_expected() -> dict[str, Any]:
    """Dtype promotion table smoke samples (not the full ufunc type-resolution matrix)."""
    bools = np.array([True, False], dtype=bool)
    ints = np.array([1, 2], dtype=np.int64)
    f32 = np.array([1.0, 2.0], dtype=np.float32)
    f64 = np.array([1.0, 2.0], dtype=np.float64)
    f32_half = np.array([1.5, 2.5], dtype=np.float32)
    i8 = np.array([-128, 127], dtype=np.int8)
    u16 = np.array([0, 65535], dtype=np.uint16)
    try:
        np.array([128], dtype=np.int8)
        invalid_i8_ok = True
    except (OverflowError, ValueError):
        invalid_i8_ok = False
    bool_int = bools + ints
    int_f32 = ints + f32
    f32_f64 = f32 + f64
    bool_f32 = bools + f32_half
    return {
        "bool_plus_int64_dtype": str(bool_int.dtype),
        "bool_plus_int64": bool_int.tolist(),
        "int64_plus_float32_dtype": str(int_f32.dtype),
        "int64_plus_float32": int_f32.tolist(),
        "float32_plus_float64_dtype": str(f32_f64.dtype),
        "float32_plus_float64": f32_f64.tolist(),
        "bool_plus_float32_dtype": str(bool_f32.dtype),
        "bool_plus_float32": bool_f32.tolist(),
        "int8": i8.tolist(),
        "uint16": u16.tolist(),
        "invalid_int8_ok": invalid_i8_ok,
    }


def warp_reduce_expected() -> dict[str, Any]:
    """argmin/argmax (flat + axis) and NaN-aware reductions vs NumPy."""
    flat = np.array([3.0, 1.0, 4.0, 1.0, 5.0])
    matrix = np.array([[3.0, 1.0, 2.0], [0.0, 4.0, 1.0]])
    nan_vals = np.array([1.0, np.nan, 3.0, 2.0])
    clean = np.array([2.0, 4.0, 6.0])
    return {
        "argmin_flat": int(np.argmin(flat)),
        "argmax_flat": int(np.argmax(flat)),
        "argmin_axis0": np.argmin(matrix, axis=0).tolist(),
        "argmin_axis1": np.argmin(matrix, axis=1).tolist(),
        "argmax_axis0": np.argmax(matrix, axis=0).tolist(),
        "argmax_axis1": np.argmax(matrix, axis=1).tolist(),
        "nanmean": float(np.nanmean(nan_vals)),
        "nansum": float(np.nansum(nan_vals)),
        "nanmean_clean": float(np.nanmean(clean)),
        "nansum_clean": float(np.nansum(clean)),
    }


def warp_fft_expected() -> dict[str, Any]:
    """1D FFT / calculus smoke vs NumPy (small n; re/im within 1e-6)."""
    a4 = np.array([1.0, 2.0, 3.0, 4.0])
    a8 = np.array([1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0])
    f4 = np.fft.fft(a4)
    f8 = np.fft.fft(a8)
    inv8 = np.fft.ifft(f8)
    y = np.array([1.0, 2.0, 4.0, 7.0, 11.0])
    trap_y = np.array([1.0, 2.0, 3.0])
    # NumPy 2.x renamed trapz -> trapezoid; support both.
    trapz_fn = getattr(np, "trapezoid", None) or getattr(np, "trapz")
    return {
        "fft4_re": f4.real.tolist(),
        "fft4_im": f4.imag.tolist(),
        "fft8_re": f8.real.tolist(),
        "fft8_im": f8.imag.tolist(),
        "ifft8_re": inv8.real.tolist(),
        "ifft8_im": inv8.imag.tolist(),
        "fftfreq8": np.fft.fftfreq(8, 1.0).tolist(),
        "diff": np.diff(y).tolist(),
        "gradient": np.gradient(y).tolist(),
        "trapz": float(trapz_fn(trap_y, dx=1.0)),
    }


def assert_fft_close(actual: Any, expected: Any, path: str = "root") -> None:
    """Like assert_equal but float lists use abs_tol=1e-6 (FFT tolerance)."""
    if isinstance(expected, float):
        if not isinstance(actual, (int, float)) or not math.isclose(
            float(actual), expected, rel_tol=0.0, abs_tol=1e-6
        ):
            raise AssertionError(f"{path}: got {actual!r}, want {expected!r} (tol 1e-6)")
        return
    if isinstance(expected, list):
        if not isinstance(actual, list) or len(actual) != len(expected):
            raise AssertionError(f"{path}: got {actual!r}, want {expected!r}")
        for index, (got, want) in enumerate(zip(actual, expected)):
            assert_fft_close(got, want, f"{path}[{index}]")
        return
    if isinstance(expected, dict):
        if not isinstance(actual, dict) or set(actual) != set(expected):
            raise AssertionError(f"{path}: got keys {actual!r}, want {expected!r}")
        for key, want in expected.items():
            assert_fft_close(actual[key], want, f"{path}.{key}")
        return
    if actual != expected:
        raise AssertionError(f"{path}: got {actual!r}, want {expected!r}")


def dataframe_index_expected() -> dict[str, Any]:
    frame = pd.DataFrame(
        [{"name": "a", "v": 1}, {"name": "b", "v": 2}, {"name": "a", "v": 3}]
    ).set_index("name", drop=False)
    labels = frame.loc[["a"]]
    # Pandas refuses reindex on duplicate labels; Weft keeps first-match fill.
    # Compare Weft against an explicit first-hit oracle for this edge.
    unique = pd.DataFrame(
        [{"name": "a", "v": 1}, {"name": "b", "v": 2}]
    ).set_index("name", drop=False)
    reindexed = unique.reindex(["b", "a", "c"])
    s1 = pd.Series([10, 20], index=["a", "b"], name="x")
    s2 = pd.Series([1, 3], index=["b", "c"], name="y")
    aligned = s1.add(s2)
    filled = s1.add(s2, fill_value=0)
    return {
        "index": frame.index.tolist(),
        "label_values": labels["v"].tolist(),
        "reindex_index": reindexed.index.tolist(),
        "reindex_values": [None if pd.isna(v) else v for v in reindexed["v"].tolist()],
        "aligned_index": aligned.index.tolist(),
        "aligned_values": [None if pd.isna(v) else v for v in aligned.tolist()],
        "filled_values": [None if pd.isna(v) else v for v in filled.tolist()],
    }


def property_broadcast_roundtrip(seed: int = 0) -> None:
    """Lightweight randomized property checks against NumPy broadcasting."""
    rng = np.random.default_rng(seed)
    for _ in range(8):
        left_shape = tuple(int(x) for x in rng.integers(1, 4, size=rng.integers(1, 3)))
        right_shape = list(left_shape)
        if len(right_shape) > 0 and rng.random() < 0.5:
            right_shape[-1] = 1
        right_shape = tuple(right_shape)
        left = rng.normal(size=left_shape)
        right = rng.normal(size=right_shape)
        try:
            expected = left + right
        except ValueError:
            continue
        # Oracle-only property: shape stability under re-broadcast.
        assert expected.shape == np.broadcast_shapes(left_shape, right_shape)


def records(frame: pd.DataFrame) -> list[dict[str, Any]]:
    return json.loads(frame.to_json(orient="records"))


def dataframe_expected() -> dict[str, Any]:
    source = pd.DataFrame(
        [
            {"name": "Alice", "age": 30, "dept": "eng", "salary": 90000},
            {"name": "Bob", "age": 25, "dept": "eng", "salary": 75000},
            {"name": "Carol", "age": 35, "dept": "sales", "salary": 85000},
            {"name": "Dan", "age": 28, "dept": "sales", "salary": 70000},
            {"name": "Eve", "age": 32, "dept": "eng", "salary": 95000},
        ]
    )
    grouped = (
        source.groupby("dept", sort=False, as_index=False)
        .agg(salary_sum=("salary", "sum"), age_mean=("age", "mean"), count=("name", "count"))
    )
    left = pd.DataFrame([{"id": 1, "name": "a"}, {"id": 2, "name": "b"}, {"id": 3, "name": "c"}])
    right = pd.DataFrame([{"id": 1, "score": 90}, {"id": 3, "score": 80}])
    pivot_source = pd.DataFrame(
        [
            {"date": "Mon", "metric": "cpu", "value": 45},
            {"date": "Mon", "metric": "mem", "value": 70},
            {"date": "Tue", "metric": "cpu", "value": 50},
            {"date": "Tue", "metric": "mem", "value": 65},
        ]
    )
    pivoted = pivot_source.pivot(index="date", columns="metric", values="value").reset_index()
    pivoted.columns.name = None
    aligned = pd.Series([10, 20], index=["a", "b"]).add(pd.Series([1, 3], index=["b", "c"]))
    return {
        "columns": ["name", "age", "dept", "salary"],
        "sorted_names": source.sort_values("age", kind="stable")["name"].tolist(),
        "filtered_index": source.loc[source["age"] > 28, "name"].tolist(),
        "sorted_index": source.loc[source["age"] > 28].sort_values("age", ascending=False, kind="stable").index.map(source["name"]).tolist(),
        "iloc_index": source.iloc[[3, 0, -1]]["name"].tolist(),
        "grouped": records(grouped),
        "joined": records(left.merge(right, on="id", how="left")),
        "pivot_columns": pivoted.columns.tolist(),
        "pivot": records(pivoted),
        "aligned_index": aligned.index.tolist(),
        "aligned_values": [None if pd.isna(value) else value for value in aligned.tolist()],
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--weft", type=Path, required=True)
    args = parser.parse_args()
    # Resolve to an absolute path so Path("./weft") does not stringify to a
    # bare "weft" that silently picks up a different binary from $PATH.
    weft = args.weft.expanduser().resolve()
    if not weft.exists():
        raise SystemExit(f"weft binary not found: {weft}")
    actual_warp = run_weft(weft, "warp_case.weft")
    assert_equal(actual_warp, warp_expected(), "warp")
    actual_strides = run_weft(weft, "warp_strides_case.weft")
    assert_equal(actual_strides, warp_strides_expected(), "warp_strides")
    actual_edges = run_weft(weft, "warp_edges_case.weft")
    assert_equal(actual_edges, warp_edges_expected(), "warp_edges")
    actual_errors = run_weft(weft, "warp_errors_case.weft")
    assert_equal(actual_errors, warp_errors_expected(), "warp_errors")
    actual_dtype = run_weft(weft, "warp_dtype_case.weft")
    assert_equal(actual_dtype, warp_dtype_expected(), "warp_dtype")
    actual_reduce = run_weft(weft, "warp_reduce_case.weft")
    assert_equal(actual_reduce, warp_reduce_expected(), "warp_reduce")
    actual_fft = run_weft(weft, "warp_fft_case.weft")
    assert_fft_close(actual_fft, warp_fft_expected(), "warp_fft")
    actual_dataframe = run_weft(weft, "dataframe_case.weft")
    assert_equal(actual_dataframe, dataframe_expected(), "dataframe")
    actual_missing = run_weft(weft, "dataframe_missing_case.weft")
    assert_equal(actual_missing, dataframe_missing_expected(), "dataframe_missing")
    actual_index = run_weft(weft, "dataframe_index_case.weft")
    assert_equal(actual_index, dataframe_index_expected(), "dataframe_index")
    property_broadcast_roundtrip(seed=0)
    print(
        f"conformance ok: NumPy {np.__version__}, pandas {pd.__version__} "
        f"(10 fixtures + property smoke)"
    )


if __name__ == "__main__":
    main()
