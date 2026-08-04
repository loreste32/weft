#!/usr/bin/env python3
"""Differential smoke checks for Weft warp/dataframe/ml against pinned NumPy/pandas/sklearn.

Honesty: this is still a smoke suite (small fixtures + a seeded property
harness), not a full NumPy/pandas/sklearn replacement matrix. It locks a few
high-value value/dtype/error behaviors from the numerical roadmap; gaps
remain outside these samples. Every check runs real Weft programs and
compares against the oracle libraries — nothing here is oracle-only.
"""

from __future__ import annotations

import argparse
import json
import math
import random
import subprocess
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd
from sklearn.linear_model import LinearRegression, LogisticRegression
from sklearn.metrics import accuracy_score, mean_squared_error, r2_score
from sklearn.preprocessing import StandardScaler


ROOT = Path(__file__).resolve().parents[2]


def run_weft(weft: Path, program: str, stdin: str | None = None) -> dict[str, Any]:
    result = subprocess.run(
        [str(weft), "run", str(ROOT / "testdata" / "conformance" / program)],
        cwd=ROOT,
        check=False,
        capture_output=True,
        text=True,
        input=stdin,
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


_WARP_DTYPES = [
    "bool", "int8", "int16", "int32", "int64",
    "uint8", "uint16", "uint32", "uint64",
    "float32", "float64",
]


def warp_dtype_expected() -> dict[str, Any]:
    """Full 11x11 np.promote_types matrix for the supported dtypes, plus
    representative range-checked casting cases (error presence as booleans)."""
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
    promoted = {
        f"{a}+{b}": np.promote_types(np.dtype(a), np.dtype(b)).name
        for a in _WARP_DTYPES
        for b in _WARP_DTYPES
    }
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
        "promoted": promoted,
        "uint8_256_ok": not _numpy_raises(lambda: np.array([256], dtype=np.uint8)),
        "uint8_neg_ok": not _numpy_raises(lambda: np.array([-1], dtype=np.uint8)),
        "int16_underflow_ok": not _numpy_raises(lambda: np.array([-32769], dtype=np.int16)),
        "uint16_65536_ok": not _numpy_raises(lambda: np.array([65536], dtype=np.uint16)),
        "int32_over_ok": not _numpy_raises(lambda: np.array([2147483648], dtype=np.int32)),
        "uint32_over_ok": not _numpy_raises(lambda: np.array([4294967296], dtype=np.uint32)),
        "uint64_neg_ok": not _numpy_raises(lambda: np.array([-1], dtype=np.uint64)),
        "int64_huge_float_ok": not _numpy_raises(lambda: np.array([1e30], dtype=np.int64)),
        "float_to_int8_trunc": np.array([2.7, -2.7]).astype(np.int8).tolist(),
        "float_to_int8_trunc_dtype": str(np.array([2.7, -2.7]).astype(np.int8).dtype),
        "bool_cast": np.array([0, 2]).astype(bool).tolist(),
        "bool_cast_dtype": str(np.array([0, 2]).astype(bool).dtype),
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


def assert_close(actual: Any, expected: Any, tol: float, path: str = "root") -> None:
    """Like assert_equal but floats use abs_tol=tol (for approximate algorithms)."""
    if isinstance(expected, float):
        if not isinstance(actual, (int, float)) or not math.isclose(
            float(actual), expected, rel_tol=0.0, abs_tol=tol
        ):
            raise AssertionError(f"{path}: got {actual!r}, want {expected!r} (tol {tol})")
        return
    if isinstance(expected, list):
        if not isinstance(actual, list) or len(actual) != len(expected):
            raise AssertionError(f"{path}: got {actual!r}, want {expected!r}")
        for index, (got, want) in enumerate(zip(actual, expected)):
            assert_close(got, want, tol, f"{path}[{index}]")
        return
    if isinstance(expected, dict):
        if not isinstance(actual, dict) or set(actual) != set(expected):
            raise AssertionError(f"{path}: got keys {actual!r}, want {expected!r}")
        for key, want in expected.items():
            assert_close(actual[key], want, tol, f"{path}.{key}")
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


# ──────────────────────────────────────────────────────────────────────────
# sklearn differential fixture (ml_case.weft)
#
# Tolerance rationale: Weft ml.linear_fit/logistic_fit are full-batch gradient
# descent; sklearn LinearRegression is closed-form and LogisticRegression is
# lbfgs. Iterative GD cannot match those optima to 1e-10, so coefficients,
# predictions, probabilities, mse/r2/accuracy are compared at abs_tol=1e-2.
# The fixture standardizes features first (population std, matching
# StandardScaler) so GD converges tightly — observed agreement is ~1e-9, so
# 1e-2 is loose enough to be stable yet tight enough to catch a wrong
# optimum, a broken sigmoid, or a converged-to-garbage model. ml.standardize
# itself is exact arithmetic and is compared at the strict 1e-10 tolerance.
# ──────────────────────────────────────────────────────────────────────────

ML_TOL = 1e-2

_ML_FEATURES = [
    [0.0, 7.0], [1.0, 3.0], [2.0, 5.0], [3.0, 1.0],
    [4.0, 6.0], [5.0, 2.0], [6.0, 4.0], [7.0, 0.5],
]
_ML_TARGETS = [-12.5, -1.5, -2.5, 8.5, 1.5, 12.5, 11.5, 21.5]
_ML_LOG_FEATURES = [
    [-2.0, -1.0], [-1.5, 0.5], [-1.0, -0.5], [-0.5, 1.0], [0.5, -1.0], [0.8, 0.6],
    [2.0, 1.0], [1.5, -0.5], [1.0, 0.8], [0.5, -0.2], [-0.6, -0.4], [0.8, 0.5],
]
_ML_LOG_TARGETS = [0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1]


def ml_expected() -> dict[str, Any]:
    features = np.array(_ML_FEATURES)
    targets = np.array(_ML_TARGETS)
    scaler = StandardScaler().fit(features)
    x = scaler.transform(features)
    lin = LinearRegression().fit(x, targets)
    lin_pred = lin.predict(x)

    log_features = np.array(_ML_LOG_FEATURES)
    log_targets = np.array(_ML_LOG_TARGETS)
    log_scaler = StandardScaler().fit(log_features)
    xl = log_scaler.transform(log_features)
    # Unregularized (C=inf) so the lbfgs optimum matches Weft's l2=0 objective.
    log = LogisticRegression(
        C=np.inf, l1_ratio=0, solver="lbfgs", max_iter=100000, tol=1e-15
    ).fit(xl, log_targets)

    return {
        "standardize": {
            "mean": scaler.mean_.tolist(),
            "scale": scaler.scale_.tolist(),
            "values": x.tolist(),
        },
        "linear": {
            "weights": lin.coef_.tolist(),
            "bias": float(lin.intercept_),
            "predictions": lin_pred.tolist(),
            "mse": float(mean_squared_error(targets, lin_pred)),
            "r2": float(r2_score(targets, lin_pred)),
        },
        "logistic": {
            "weights": log.coef_[0].tolist(),
            "bias": float(log.intercept_[0]),
            "probabilities": log.predict_proba(xl)[:, 1].tolist(),
            "accuracy": float(accuracy_score(log_targets, log.predict(xl))),
        },
    }


# ──────────────────────────────────────────────────────────────────────────
# Property harness (warp_property_case.weft)
#
# A seeded RNG generates op specs; each spec is fed to the Weft driver on
# stdin, which builds inputs deterministically from the seed (arithmetic
# patterns of exact binary fractions — no unseeded randomness on either
# side) and the result is compared against NumPy at the standard tolerance.
# Note: 0-size shapes are only generated for elementwise ops — Weft
# (deliberately) errors on reductions over an empty axis where NumPy
# returns an identity value, so that divergence is out of scope here.
# ──────────────────────────────────────────────────────────────────────────


def _prop_value(kind: str, seed: int, i: int) -> float:
    # Mirror of _aval/_bval/_dval in warp_property_case.weft. Keep in sync.
    if kind == "a":
        return (((seed * 31 + i * 7) % 21) - 10) / 4.0
    if kind == "b":
        return (((seed * 17 + i * 11) % 19) - 9) / 4.0
    return (((seed * 13 + i * 3) % 16) + 4) / 8.0


def _prop_array(kind: str, seed: int, shape: list[int]) -> np.ndarray:
    n = math.prod(shape)
    return np.array([_prop_value(kind, seed, i) for i in range(n)]).reshape(shape)


def _prop_expected(spec: dict[str, Any]) -> dict[str, Any]:
    a = _prop_array("a", spec["seed"], spec["a_shape"])
    if spec["transpose_a"]:
        a = a.T  # matches warp.transpose_view(a, null) (reversed axes)
    op = spec["op"]
    if op in ("add", "sub", "mul", "div", "less", "greater", "equal",
              "less_equal", "greater_equal", "not_equal"):
        kind = "d" if op == "div" else "b"
        b = _prop_array(kind, spec["seed"], spec["b_shape"])
        result = {
            "add": a + b, "sub": a - b, "mul": a * b, "div": a / b,
            "less": a < b, "greater": a > b, "equal": a == b,
            "less_equal": a <= b, "greater_equal": a >= b, "not_equal": a != b,
        }[op]
    elif op in ("sum", "mean", "min", "max"):
        fn = {"sum": np.sum, "mean": np.mean, "min": np.min, "max": np.max}[op]
        result = fn(a, axis=spec["axis"], keepdims=spec["keepdims"])
    elif op == "matmul":
        result = a @ _prop_array("b", spec["seed"], spec["b_shape"])
    elif op == "reshape":
        result = a.reshape(spec["new_shape"])
    elif op == "transpose":
        result = a.transpose(spec["axes"])
    else:
        raise AssertionError(f"unknown property op {op}")
    out = np.asarray(result)
    return {
        "shape": list(out.shape),
        "dtype": str(out.dtype),
        "data": out.reshape(-1).tolist(),
    }


def _broadcastable(rng: random.Random, shape: list[int]) -> list[int]:
    b = [1 if rng.random() < 0.4 else d for d in shape]
    while len(b) > 1 and rng.random() < 0.3:
        b = b[1:]
    return b


def _rand_shape(rng: random.Random, max_rank: int = 3, max_dim: int = 4) -> list[int]:
    return [rng.randint(1, max_dim) for _ in range(rng.randint(1, max_rank))]


def _prop_specs(seed: int = 20240817) -> list[dict[str, Any]]:
    rng = random.Random(seed)
    specs: list[dict[str, Any]] = []

    # Fixed edge cases: 0-size dims (elementwise only; see note above) and
    # exact-equality comparisons.
    specs.append({"op": "add", "seed": 101, "a_shape": [0, 3], "b_shape": [3], "transpose_a": False})
    specs.append({"op": "mul", "seed": 102, "a_shape": [2, 0], "b_shape": [2, 1], "transpose_a": False})
    specs.append({"op": "sub", "seed": 103, "a_shape": [0], "b_shape": [0], "transpose_a": False})
    specs.append({"op": "equal", "seed": 104, "a_shape": [3, 3], "b_shape": [3, 3], "transpose_a": False})

    elem_ops = ["add", "sub", "mul", "div", "less", "greater",
                "less_equal", "greater_equal", "not_equal"]
    for _ in range(9):
        op = rng.choice(elem_ops)
        a_shape = _rand_shape(rng)
        transpose_a = rng.random() < 0.35 and len(a_shape) > 1
        eff = a_shape[::-1] if transpose_a else a_shape
        specs.append({
            "op": op, "seed": rng.randint(0, 10**6), "a_shape": a_shape,
            "b_shape": _broadcastable(rng, eff), "transpose_a": transpose_a,
        })

    for _ in range(5):
        op = rng.choice(["sum", "mean", "min", "max"])
        a_shape = _rand_shape(rng, max_rank=3)
        transpose_a = rng.random() < 0.35 and len(a_shape) > 1
        eff = a_shape[::-1] if transpose_a else a_shape
        specs.append({
            "op": op, "seed": rng.randint(0, 10**6), "a_shape": a_shape,
            "axis": rng.randrange(len(eff)), "keepdims": rng.random() < 0.5,
            "transpose_a": transpose_a,
        })

    for _ in range(3):
        m, k, n = (rng.randint(1, 4) for _ in range(3))
        transpose_a = rng.random() < 0.4
        specs.append({
            "op": "matmul", "seed": rng.randint(0, 10**6),
            "a_shape": [k, m] if transpose_a else [m, k],
            "b_shape": [k, n], "transpose_a": transpose_a,
        })

    for _ in range(2):
        a_shape = _rand_shape(rng)
        size = math.prod(a_shape)
        factors = [d for d in range(1, size + 1) if size % d == 0]
        d = rng.choice(factors)
        specs.append({
            "op": "reshape", "seed": rng.randint(0, 10**6), "a_shape": a_shape,
            "new_shape": [d, size // d],
            "transpose_a": False,
        })

    for _ in range(2):
        rank = rng.randint(2, 3)
        a_shape = [rng.randint(1, 4) for _ in range(rank)]
        specs.append({
            "op": "transpose", "seed": rng.randint(0, 10**6), "a_shape": a_shape,
            "axes": rng.sample(range(rank), rank), "transpose_a": False,
        })

    return specs


def property_cases(weft: Path) -> int:
    specs = _prop_specs()
    for spec in specs:
        actual = run_weft(weft, "warp_property_case.weft", stdin=json.dumps(spec))
        expected = _prop_expected(spec)
        assert_equal(actual, expected, f"property[{json.dumps(spec)}]")
    return len(specs)


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
    assert_close(actual_fft, warp_fft_expected(), 1e-6, "warp_fft")
    actual_dataframe = run_weft(weft, "dataframe_case.weft")
    assert_equal(actual_dataframe, dataframe_expected(), "dataframe")
    actual_missing = run_weft(weft, "dataframe_missing_case.weft")
    assert_equal(actual_missing, dataframe_missing_expected(), "dataframe_missing")
    actual_index = run_weft(weft, "dataframe_index_case.weft")
    assert_equal(actual_index, dataframe_index_expected(), "dataframe_index")
    expected_ml = ml_expected()
    actual_ml = run_weft(weft, "ml_case.weft")
    assert_equal(actual_ml["standardize"], expected_ml["standardize"], "ml.standardize")
    assert_close(actual_ml["linear"], expected_ml["linear"], ML_TOL, "ml.linear")
    assert_close(actual_ml["logistic"], expected_ml["logistic"], ML_TOL, "ml.logistic")
    num_property = property_cases(weft)
    print(
        f"conformance ok: NumPy {np.__version__}, pandas {pd.__version__} "
        f"(11 fixtures + {num_property} Weft-backed property cases)"
    )


if __name__ == "__main__":
    main()
