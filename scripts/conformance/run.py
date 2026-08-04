#!/usr/bin/env python3
"""Differential checks for Weft warp/dataframe against pinned NumPy/pandas."""

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
    }


def dataframe_missing_expected() -> dict[str, Any]:
    frame = pd.DataFrame([{"a": 1}, {"b": 2}])
    return {
        "columns": frame.columns.tolist(),
        "a": [1, None],
        "b": [None, 2],
    }


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
    actual_warp = run_weft(args.weft, "warp_case.weft")
    assert_equal(actual_warp, warp_expected(), "warp")
    actual_strides = run_weft(args.weft, "warp_strides_case.weft")
    assert_equal(actual_strides, warp_strides_expected(), "warp_strides")
    actual_dataframe = run_weft(args.weft, "dataframe_case.weft")
    assert_equal(actual_dataframe, dataframe_expected(), "dataframe")
    actual_missing = run_weft(args.weft, "dataframe_missing_case.weft")
    assert_equal(actual_missing, dataframe_missing_expected(), "dataframe_missing")
    print(f"conformance ok: NumPy {np.__version__}, pandas {pd.__version__}")


if __name__ == "__main__":
    main()
