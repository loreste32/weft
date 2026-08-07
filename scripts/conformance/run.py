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
import warnings
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


def warp_index_expected() -> dict[str, Any]:
    """ellipsis (`...`) and newaxis (`None`) selectors: values, shapes, dtypes,
    assignment through both, and the IndexError cases (error presence only)."""
    cube = np.arange(24, dtype=np.int64).reshape(2, 3, 4)
    matrix = np.arange(6, dtype=np.int64).reshape(2, 3)
    f32 = np.arange(1, 7, dtype=np.float32).reshape(2, 3)
    set_ellipsis = matrix.copy()
    set_ellipsis[..., 0] = 5
    set_newaxis = matrix.copy()
    set_newaxis[None, 0] = 7
    set_fewer = matrix.copy()
    set_fewer[0] = 9
    set_cube = cube.copy()
    set_cube[..., 1] = np.arange(100, 106, dtype=np.int64).reshape(2, 3)
    return {
        "ell_trailing_shape": list(cube[..., 0].shape),
        "ell_trailing": cube[..., 0].reshape(-1).tolist(),
        "ell_trailing_dtype": str(cube[..., 0].dtype),
        "ell_zero_shape": list(matrix[1, ...].shape),
        "ell_zero": matrix[1, ...].reshape(-1).tolist(),
        "newaxis_wrap_shape": list(matrix[None, ..., None].shape),
        "newaxis_pair_shape": list(matrix[None, :, None].shape),
        "newaxis_pair": matrix[None, :, None].reshape(-1).tolist(),
        "newaxis_scalar_shape": list(matrix[0, None].shape),
        "newaxis_only_shape": list(matrix[None, None, 0, 0].shape),
        "adv_newaxis_shape": list(matrix[[0, 1], None, :].shape),
        "adv_newaxis": matrix[[0, 1], None, :].reshape(-1).tolist(),
        "adv_separated_shape": list(matrix[[0, 1], None, [0, 1]].shape),
        "adv_separated": matrix[[0, 1], None, [0, 1]].reshape(-1).tolist(),
        "slice_ellipsis_shape": list(cube[..., 0:2].shape),
        "slice_ellipsis": cube[..., 0:2].reshape(-1).tolist(),
        "typed_dtype": str(f32[..., 1].dtype),
        "typed": f32[..., 1].reshape(-1).tolist(),
        "set_ellipsis": set_ellipsis.reshape(-1).tolist(),
        "set_newaxis": set_newaxis.reshape(-1).tolist(),
        "set_fewer": set_fewer.reshape(-1).tolist(),
        "set_cube_shape": list(set_cube.shape),
        "set_cube_lane": set_cube[..., 1].reshape(-1).tolist(),
        "err_index_two_ellipsis": _numpy_raises(lambda: cube[..., ...]),
        "err_index_too_many": _numpy_raises(lambda: matrix[0, 0, 0]),
        "err_index_too_many_newaxis": _numpy_raises(lambda: matrix[None, 0, 0, 0]),
        "err_set_two_ellipsis": _numpy_raises(lambda: matrix.__setitem__((..., ..., 0), 1)),
        "err_set_too_many": _numpy_raises(lambda: matrix.__setitem__((0, 0, 0), 1)),
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
    "float16", "float32", "float64",
]


def warp_dtype_expected() -> dict[str, Any]:
    """Full 12x12 np.promote_types matrix for the supported dtypes, plus
    representative range-checked casting cases (error presence as booleans)
    and float16 rounding/overflow semantics (values are exact: every binary16
    value is representable in float64)."""
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
    f16 = np.array([0.1, 1.5, 65504.0, 2048.0001], dtype=np.float16)
    f16_sub = np.array(
        [5.960464477539063e-08, 2.9802322387695312e-08, 8.940696716308594e-08, 1.0e-10],
        dtype=np.float16,
    )
    f16_ties = np.array(
        [1.00048828125, 1.00146484375, 0.999755859375, 0.9996337890625],
        dtype=np.float16,
    )
    with warnings.catch_warnings():
        # np.float16 overflow is a RuntimeWarning, not an error; warp matches
        # by producing ±inf without an Err.
        warnings.simplefilter("ignore", RuntimeWarning)
        f16_over = np.array([1e10, -1e10, 65519.0, 65520.0], dtype=np.float16)
    f16_sum = np.float16(0.1) + np.float16(0.2)
    f16_roundtrip = f16.astype(np.float64)
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
        "float16_dtype": str(f16.dtype),
        "float16_itemsize": f16.dtype.itemsize,
        "float16_round": f16.astype(np.float64).tolist(),
        "float16_subnormal": f16_sub.astype(np.float64).tolist(),
        "float16_ties": f16_ties.astype(np.float64).tolist(),
        "float16_overflow_pos_inf": bool(np.isinf(f16_over[0]) and f16_over[0] > 0),
        "float16_overflow_neg_inf": bool(np.isinf(f16_over[1]) and f16_over[1] < 0),
        "float16_below_max": float(f16_over[2]),
        "float16_midpoint_inf": bool(np.isinf(f16_over[3])),
        "float16_sum": [float(f16_sum)],
        "float16_sum_dtype": str(np.asarray(f16_sum).dtype),
        "float16_int_scalar_dtype": str((np.array([1], dtype=np.float16) + 1).dtype),
        "float16_roundtrip": f16_roundtrip.tolist(),
        "float16_roundtrip_dtype": str(f16_roundtrip.dtype),
    }


def warp_reduce_expected() -> dict[str, Any]:
    """argmin/argmax (flat + axis), NaN-aware reductions, and the
    initial=/where=/ddof/cumulative-axis reduction options vs NumPy."""
    flat = np.array([3.0, 1.0, 4.0, 1.0, 5.0])
    matrix = np.array([[3.0, 1.0, 2.0], [0.0, 4.0, 1.0]])
    nan_vals = np.array([1.0, np.nan, 3.0, 2.0])
    clean = np.array([2.0, 4.0, 6.0])
    a = np.array([[1.0, 2.0, 3.0], [4.0, 5.0, 6.0]])
    mask = np.array([[True, False, True], [False, True, False]])
    none = np.zeros((2, 3), dtype=bool)
    sum_axis1_initial = np.sum(a, axis=1, initial=10, keepdims=True)
    sum_where_flat_keepdims = np.sum(a, where=mask, keepdims=True)
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
        "sum_initial": float(np.sum(a, initial=100)),
        "prod_initial": float(np.prod(a, initial=2)),
        "sum_axis1_initial_keepdims": {
            "shape": list(sum_axis1_initial.shape),
            "data": sum_axis1_initial.reshape(-1).tolist(),
        },
        "sum_where": float(np.sum(a, where=mask)),
        "sum_where_axis1": np.sum(a, axis=1, where=mask).tolist(),
        "min_where_axis1": np.min(a, axis=1, where=mask, initial=10).tolist(),
        "max_where_axis0": np.max(a, axis=0, where=mask, initial=0).tolist(),
        # NumPy requires initial= whenever where= is used with min/max.
        "min_where_no_initial_raises": _numpy_raises(lambda: np.min(a, axis=1, where=mask)),
        "sum_where_empty": float(np.sum(a, where=none)),
        "prod_where_empty": float(np.prod(a, where=none)),
        "sum_where_empty_axis0": np.sum(a, axis=0, where=none).tolist(),
        "min_where_empty_initial": float(np.min(a, where=none, initial=99)),
        "min_where_empty_raises": _numpy_raises(lambda: np.min(a, where=none)),
        "max_where_empty_raises": _numpy_raises(lambda: np.max(a, where=none)),
        "sum_where_flat_keepdims": {
            "shape": list(sum_where_flat_keepdims.shape),
            "data": sum_where_flat_keepdims.reshape(-1).tolist(),
        },
        "var_ddof0": float(np.var(clean, ddof=0)),
        "var_ddof1": float(np.var(clean, ddof=1)),
        "std_ddof1": float(np.std(clean, ddof=1)),
        # NumPy warns and divides anyway when ddof >= n: the result is +inf,
        # or NaN when the deviation sum is zero.
        "var_ddof3_isinf": bool(np.isinf(np.var(clean, ddof=3))),
        "var_ddof4_isinf": bool(np.isinf(np.var(clean, ddof=4))),
        "var_zero_dev_ddof_isnan": bool(np.isnan(np.var(np.array([5.0, 5.0]), ddof=2))),
        "cumsum_axis0": np.cumsum(a, axis=0).reshape(-1).tolist(),
        "cumsum_axis1": np.cumsum(a, axis=1).reshape(-1).tolist(),
        "cumsum_axis_neg": np.cumsum(a, axis=-1).reshape(-1).tolist(),
        "cumprod_axis1": np.cumprod(a, axis=1).reshape(-1).tolist(),
        "cumsum_axis_dtype": str(np.cumsum(np.array([[1, 2], [3, 4]]), axis=0).dtype),
    }


def warp_ufunc_expected() -> dict[str, Any]:
    """hypot/expm1/log1p/floor_divide/remainder/square/reciprocal/deg2rad/
    rad2deg/copysign/rint vs the NumPy ufuncs. Zero-divisor cases follow
    NumPy's warn-and-continue semantics (0 for ints, NaN for floats)."""
    with np.errstate(divide="ignore", invalid="ignore"):
        fd_zero_float = np.floor_divide(np.array([1.0]), np.array([0.0]))
        rem_zero_float = np.remainder(np.array([1.0]), np.array([0.0]))
        fd_zero_int = np.floor_divide(np.array([1, 2]), np.array([0, 2]))
    fd_int = np.floor_divide(np.array([-7, 7, -7, 7]), np.array([2, 2, -2, -2]))
    sq = np.square(np.array([-3, 2]))
    ri = np.rint(np.array([0.5, 1.5, 2.5, -0.5, -1.5, -2.5, 2.3, 2.7]))
    return {
        "hypot": np.hypot(np.array([3.0, 4.0]), np.array([4.0, 3.0])).tolist(),
        "hypot_dtype": str(np.hypot(np.array([3.0, 4.0]), np.array([4.0, 3.0])).dtype),
        "expm1": np.expm1(np.array([1.0e-10, 1.0, -1.0, 0.25, -0.25])).tolist(),
        "log1p": np.log1p(np.array([1.0e-10, 1.0, -0.5, 0.0])).tolist(),
        "floor_divide_int": fd_int.tolist(),
        "floor_divide_int_dtype": str(fd_int.dtype),
        "floor_divide_float": np.floor_divide(np.array([-7.5, 7.5]), np.array([2.0, -2.0])).tolist(),
        "floor_divide_zero_int": fd_zero_int.tolist(),
        "floor_divide_zero_float_isinf": bool(np.isinf(fd_zero_float[0])),
        "remainder_int": np.remainder(np.array([-7, 7, -7, 7]), np.array([3, 3, -3, -3])).tolist(),
        "remainder_float": np.remainder(np.array([-7.5, 7.5]), np.array([2.0, -2.0])).tolist(),
        "remainder_zero_float_isnan": bool(np.isnan(rem_zero_float[0])),
        "square": sq.tolist(),
        "square_dtype": str(sq.dtype),
        "reciprocal": np.reciprocal(np.array([4.0, -0.5])).tolist(),
        "deg2rad": np.deg2rad(np.array([0.0, 90.0, 180.0])).tolist(),
        "rad2deg": np.rad2deg(np.array([1.5707963267948966, 3.141592653589793])).tolist(),
        "copysign": np.copysign(np.array([2.0, -2.0, 2.0]), np.array([-1.0, -1.0, 1.0])).tolist(),
        "rint": ri.tolist(),
        "rint_dtype": str(ri.dtype),
    }


def warp_random_expected() -> dict[str, Any]:
    """Seeded RNG: deterministic properties only. Warp's generator is a
    combined L'Ecuyer MRG, not NumPy's PCG64, so bit-parity against
    np.random.default_rng is explicitly NOT checked; the fixture locks its own
    per-seed draws and checks coverage/distribution invariants as booleans."""
    return {
        "draws_replay_equal": True,
        "draws_seed_diverges": True,
        "locked_uniform": True,
        "locked_normal": True,
        "locked_integers": True,
        "locked_shuffle": True,
        "locked_permutation": True,
        "locked_choice": True,
        "shuffle_is_permutation": True,
        "permutation_covers": True,
        "choice_distinct": True,
        "integers_in_bounds": True,
        "uniform_mean_in_bounds": True,
        "uniform_std_in_bounds": True,
        "normal_mean_in_bounds": True,
        "normal_std_in_bounds": True,
    }


def warp_sort_expected() -> dict[str, Any]:
    """put / sort_axis / argsort_axis / unique options vs NumPy."""
    base = np.array([1.0, 2.0, 3.0, 4.0])

    def put_case(indices: list[int], values: list[float]) -> list[float]:
        out = base.copy()
        np.put(out, indices, values)
        return out.tolist()

    m = np.array([[3.0, 1.0, 2.0], [0.0, 4.0, 1.0]])
    uniq_values, uniq_index, uniq_counts = np.unique(
        np.array([3, 1, 3, 2, 1, 3]), return_index=True, return_counts=True
    )
    return {
        "put_basic": put_case([0, 3], [10.0, 40.0]),
        "put_duplicate_last_wins": put_case([1, 1], [7.0, 9.0]),
        "put_cycle": put_case([0, 1, 2, 3], [0.0]),
        "put_negative": put_case([-1], [99.0]),
        "put_oob_raises": _numpy_raises(lambda: np.put(base.copy(), [4], [0.0])),
        "put_preserves_shape": list(np.array([[1, 2], [3, 4]]).shape),
        "put_source_unchanged": base.tolist(),
        "sort_axis0": np.sort(m, axis=0).reshape(-1).tolist(),
        "sort_axis1": np.sort(m, axis=1).reshape(-1).tolist(),
        "sort_axis_neg": np.sort(m, axis=-2).reshape(-1).tolist(),
        "argsort_axis0": np.argsort(m, axis=0, kind="stable").reshape(-1).tolist(),
        "argsort_axis1": np.argsort(m, axis=1, kind="stable").reshape(-1).tolist(),
        "unique_values": uniq_values.tolist(),
        "unique_index": uniq_index.tolist(),
        "unique_counts": uniq_counts.tolist(),
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
    a7 = np.array([1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0])
    r4 = np.fft.rfft(a4)
    r8 = np.fft.rfft(a8)
    r7 = np.fft.rfft(a7)
    odd5 = np.array([0.0, 1.0, 2.0, 3.0, 4.0])
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
        "rfft4_re": r4.real.tolist(),
        "rfft4_im": r4.imag.tolist(),
        "rfft8_re": r8.real.tolist(),
        "rfft8_im": r8.imag.tolist(),
        "irfft8": np.fft.irfft(r8).tolist(),
        "rfft7_re": r7.real.tolist(),
        "rfft7_im": r7.imag.tolist(),
        "irfft7": np.fft.irfft(r7, 7).tolist(),
        "rfftfreq8": np.fft.rfftfreq(8, 1.0).tolist(),
        "rfftfreq7": np.fft.rfftfreq(7, 0.5).tolist(),
        "fftshift_even": np.fft.fftshift(a4).tolist(),
        "fftshift_odd": np.fft.fftshift(odd5).tolist(),
        "ifftshift_even": np.fft.ifftshift(a4).tolist(),
        "ifftshift_odd": np.fft.ifftshift(odd5).tolist(),
    }


def warp_poly_expected() -> dict[str, Any]:
    """np.poly* family vs warp's polynomial helpers. Roots are sorted by
    descending real part (then descending imaginary) on both sides, because
    NumPy's companion-matrix ordering is unspecified. roots_deg3_unsupported
    and polyfit_underdetermined_err are warp-only contracts: NumPy computes
    degree-3 roots via eigenvalues and only warns on underdetermined fits,
    while warp returns an explicit error (documented deviation)."""
    x = np.array([0.0, 1.0, 2.0, 3.0, 4.0])
    x1 = np.array([0.0, 1.0, 2.0, 3.0])
    r2 = _sorted_roots([1.0, -3.0, 2.0])
    r1 = _sorted_roots([2.0, -6.0])
    rc = _sorted_roots([1.0, 0.0, 1.0])
    rt = _sorted_roots([0.0, 1.0, 2.0])
    return {
        "fit2": np.polyfit(x, x * x + x + 1.0, 2).tolist(),
        "fit1": np.polyfit(x1, 2.0 * x1 + 1.0, 1).tolist(),
        "fit1_noisy": np.polyfit(x, np.array([1.1, 2.9, 5.2, 6.8, 9.1]), 1).tolist(),
        "val": np.polyval([2.0, -3.0, 1.0], np.array([0.0, 1.0, 2.5])).tolist(),
        "val_scalar": float(np.polyval([2.0, -3.0, 1.0], 2.5)),
        "der": np.polyder([2.0, -3.0, 1.0]).tolist(),
        "der_const": np.polyder([5.0]).tolist(),
        "integ": np.polyint([2.0, -3.0, 1.0]).tolist(),
        "roots2_re": r2[0],
        "roots2_im": r2[1],
        "roots1_re": r1[0],
        "roots_complex_re": rc[0],
        "roots_complex_im": rc[1],
        "roots_trim_re": rt[0],
        "roots_const_re": _sorted_roots([5.0])[0],
        "roots_deg3_unsupported": True,
        "polyfit_underdetermined_err": True,
        "polyfit_mismatch_err": _numpy_raises(lambda: np.polyfit(x1, [1.0], 1)),
    }


def _sorted_roots(coeffs: list[float]) -> tuple[list[float], list[float]]:
    r = np.roots(np.array(coeffs, dtype=float))
    order = sorted(range(len(r)), key=lambda i: (-r[i].real, -r[i].imag))
    return [float(r[i].real) for i in order], [float(r[i].imag) for i in order]


def warp_search_expected() -> dict[str, Any]:
    """searchsorted / digitize vs NumPy, with before-first, after-last, and
    exact bin-edge values. digitize's empty-bins case maps every x to 0 in
    NumPy; warp mirrors that."""
    a = np.array([1.0, 2.0, 2.0, 3.0])
    x = np.array([0.5, 1.0, 2.0, 3.5])
    return {
        "ss_left": np.searchsorted(a, np.array([0.5, 2.0, 4.0])).tolist(),
        "ss_right": np.searchsorted(a, np.array([0.5, 2.0, 4.0]), side="right").tolist(),
        "ss_scalar_left": int(np.searchsorted(a, 2.0)),
        "ss_scalar_right": int(np.searchsorted(a, 2.0, side="right")),
        "ss_before": int(np.searchsorted(a, -1.0)),
        "ss_after": int(np.searchsorted(a, 99.0, side="right")),
        "ss_dtype": str(np.searchsorted(a, np.array([0.5])).dtype),
        "ss_bad_side_err": _numpy_raises(lambda: np.searchsorted(a, 1.0, side="middle")),
        "dig_inc": np.digitize(x, np.array([1.0, 2.0, 3.0])).tolist(),
        "dig_inc_right": np.digitize(x, np.array([1.0, 2.0, 3.0]), right=True).tolist(),
        "dig_dec": np.digitize(x, np.array([3.0, 2.0, 1.0])).tolist(),
        "dig_dec_right": np.digitize(x, np.array([3.0, 2.0, 1.0]), right=True).tolist(),
        "dig_scalar": int(np.digitize(2.0, np.array([1.0, 2.0, 3.0]))),
        "dig_single_bin": np.digitize(np.array([0.5, 1.0, 2.0]), np.array([1.0])).tolist(),
        "dig_equal_bins": np.digitize(np.array([0.5, 1.0, 1.5]), np.array([1.0, 1.0, 2.0])).tolist(),
        "dig_empty_bins": np.digitize(np.array([0.5, 2.0]), np.array([])).tolist(),
        "dig_dtype": str(np.digitize(x, np.array([1.0, 2.0, 3.0])).dtype),
        "dig_nonmono_err": _numpy_raises(lambda: np.digitize(x, np.array([1.0, 3.0, 2.0]))),
    }


def warp_stats_expected() -> dict[str, Any]:
    """histogram / bincount / cov / corrcoef / average / quantile vs NumPy."""
    counts, edges = np.histogram(np.array([0.5, 1.5, 2.5, 3.5, 0.0, 4.0]), bins=4, range=(0.0, 4.0))
    auto_counts, auto_edges = np.histogram(np.array([0.0, 4.0]), bins=4)
    exp_counts, exp_edges = np.histogram(np.array([1.0, 2.0, 3.0, 0.5, 3.5]), bins=np.array([1.0, 2.0, 3.0]))
    empty_counts, empty_edges = np.histogram(np.array([]), bins=4)
    const_counts, const_edges = np.histogram(np.array([2.0, 2.0]), bins=4)
    q_src = np.array([3.0, 1.0, 4.0, 2.0])
    return {
        "hist_counts": counts.tolist(),
        "hist_edges": edges.tolist(),
        "hist_auto_counts": auto_counts.tolist(),
        "hist_auto_edges": auto_edges.tolist(),
        "hist_explicit_counts": exp_counts.tolist(),
        "hist_explicit_edges": exp_edges.tolist(),
        "hist_counts_dtype": str(counts.dtype),
        "hist_edges_dtype": str(edges.dtype),
        "hist_empty_auto_counts": empty_counts.tolist(),
        "hist_empty_auto_edges": empty_edges.tolist(),
        "hist_const_counts": const_counts.tolist(),
        "hist_const_edges": const_edges.tolist(),
        "hist_bins0_err": _numpy_raises(lambda: np.histogram(np.array([1.0]), bins=0)),
        "hist_nonmono_err": _numpy_raises(lambda: np.histogram(np.array([1.0]), bins=np.array([3.0, 1.0]))),
        "bincount": np.bincount(np.array([0, 3, 3, 1])).tolist(),
        "bincount_empty": np.bincount(np.array([], dtype=np.int64)).tolist(),
        "bincount_dtype": str(np.bincount(np.array([0, 3, 3, 1])).dtype),
        "bincount_neg_err": _numpy_raises(lambda: np.bincount(np.array([-1]))),
        "cov_pair": np.cov(np.array([1.0, 2.0, 4.0]), np.array([2.0, 1.0, 3.0])).reshape(-1).tolist(),
        "cov_single": float(np.cov(np.array([1.0, 2.0, 4.0]))),
        "cov_mismatch_err": _numpy_raises(lambda: np.cov(np.array([1.0, 2.0]), np.array([1.0]))),
        "corr_pair": np.corrcoef(np.array([1.0, 2.0, 4.0]), np.array([2.0, 1.0, 3.0])).reshape(-1).tolist(),
        "corr_single": float(np.corrcoef(np.array([1.0, 2.0, 3.0]))),
        "avg": float(np.average(np.array([1.0, 2.0, 3.0]))),
        "avg_w": float(np.average(np.array([1.0, 2.0, 3.0]), weights=np.array([1.0, 1.0, 2.0]))),
        "avg_zero_weights_err": _numpy_raises(
            lambda: np.average(np.array([1.0, 2.0]), weights=np.array([0.0, 0.0]))
        ),
        "q": float(np.quantile(q_src, 0.25)),
        "q0": float(np.quantile(q_src, 0)),
        "q1": float(np.quantile(q_src, 1)),
        "q_list": np.quantile(q_src, [0.0, 0.5, 1.0]).tolist(),
        "q_range_err": _numpy_raises(lambda: np.quantile(np.array([1.0]), 1.5)),
        "q_empty_err": _numpy_raises(lambda: np.quantile(np.array([]), 0.5)),
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
    ew_source = pd.Series([1.0, 2.0, np.nan, 4.0, 5.0])
    ewma_adjust = ew_source.ewm(alpha=0.5).mean()
    ewma_recursive = ew_source.ewm(alpha=0.5, adjust=False, ignore_na=True).mean()
    ewm_var = ew_source.ewm(span=3).var()
    ewm_sum = ew_source.ewm(halflife=1).sum()

    def ewm_list(series: pd.Series) -> list[Any]:
        return [None if pd.isna(value) else value for value in series.tolist()]

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
        "ewma_adjust": ewm_list(ewma_adjust),
        "ewma_recursive": ewm_list(ewma_recursive),
        "ewm_var": ewm_list(ewm_var),
        "ewm_sum": ewm_list(ewm_sum),
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
    actual_warp_index = run_weft(weft, "warp_index_case.weft")
    assert_equal(actual_warp_index, warp_index_expected(), "warp_index")
    actual_edges = run_weft(weft, "warp_edges_case.weft")
    assert_equal(actual_edges, warp_edges_expected(), "warp_edges")
    actual_errors = run_weft(weft, "warp_errors_case.weft")
    assert_equal(actual_errors, warp_errors_expected(), "warp_errors")
    actual_dtype = run_weft(weft, "warp_dtype_case.weft")
    assert_equal(actual_dtype, warp_dtype_expected(), "warp_dtype")
    actual_reduce = run_weft(weft, "warp_reduce_case.weft")
    assert_equal(actual_reduce, warp_reduce_expected(), "warp_reduce")
    actual_ufunc = run_weft(weft, "warp_ufunc_case.weft")
    assert_equal(actual_ufunc, warp_ufunc_expected(), "warp_ufunc")
    actual_random = run_weft(weft, "warp_random_case.weft")
    assert_equal(actual_random, warp_random_expected(), "warp_random")
    actual_sort = run_weft(weft, "warp_sort_case.weft")
    assert_equal(actual_sort, warp_sort_expected(), "warp_sort")
    actual_poly = run_weft(weft, "warp_poly_case.weft")
    assert_equal(actual_poly, warp_poly_expected(), "warp_poly")
    actual_search = run_weft(weft, "warp_search_case.weft")
    assert_equal(actual_search, warp_search_expected(), "warp_search")
    actual_stats = run_weft(weft, "warp_stats_case.weft")
    assert_equal(actual_stats, warp_stats_expected(), "warp_stats")
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
        f"(18 fixtures + {num_property} Weft-backed property cases)"
    )


if __name__ == "__main__":
    main()
