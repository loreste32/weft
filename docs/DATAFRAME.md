# dataframe — pandas-inspired tabular data for Weft

Pure `.weft` module, no native dependencies. It uses row-list storage with an explicit ordered schema, null-aware aggregation, grouping, joins, pivoting, rolling windows, and I/O. It is designed for lightweight ETL and analysis, not drop-in pandas compatibility.

```bash
weft get dataframe
```

```weft
use dataframe as df

fn main -> Result {
    t := df.read_csv("users.csv", null)?
    df.print_(t, 5)

    eng := df.filter_(t, fn(r) { r.dept == "eng" })
    stats := df.describe(eng, "salary")
    say("mean salary: " + str(stats.mean))
}
```

## Creation

| Function | What it does |
|----------|-------------|
| `from_rows([{…}, …])` | Row maps; derives the union schema in first-seen order |
| `from_columns({"col": [vals]})` | Column map → `Result`; all columns must have equal lengths |
| `from_csv(text, sep?)` | Parse quoted CSV with CRLF/LF support → `Result` |
| `from_json(text)` | Parse JSON array → Result |
| `from_jsonl(text)` | Parse newline-delimited JSON → Result |
| `read_csv(path, sep?)` | Read CSV file → Result |
| `read_json(path)` | Read JSON file → Result |
| `read_jsonl(path)` | Read JSONL file → Result |
| `empty(cols)` | Empty dataframe with column names |

```weft
// from code
t := df.from_rows([
    {"name": "Alice", "age": 30},
    {"name": "Bob", "age": 25},
])

// from CSV string
t := df.from_csv("name,age\nAlice,30\nBob,25\n", null)?

// from file
t := df.read_csv("data.csv", null)?
t := df.read_jsonl("data.jsonl")?
```

`from_rows` derives the first-seen union of row keys and normalizes absent fields to `null`, so a sparse record list remains column-accessible and deterministic.

## Shape & info

| Function | Returns |
|----------|---------|
| `columns(t)` | Column name list |
| `rows(t)` | Raw row list |
| `nrows(t)` / `ncols(t)` | Row / column count |
| `shape(t)` | `[rows, cols]` |
| `dtypes(t)` | Type map (`null`, `bool`, `int`, `float`, `str`, or `mixed`) scanned across all rows |
| `info(t)` | Summary map with row/column counts and approximate `memory_est_bytes` |

## Selection & indexing

| Function | |
|----------|--|
| `col(t, name)` | Extract one column as a list |
| `head(t, n?)` | First n rows (default 5) |
| `tail(t, n?)` | Last n rows (default 5) |
| `iloc(t, idx)` | Single row by position |
| `loc(t, start, stop?)` | Half-open row range by position; this is not label-based pandas `.loc` |
| `select_(t, [cols])` | Keep only listed columns |
| `drop(t, [cols])` | Remove listed columns |

### Series and labels

`col(t, name)` remains the compatibility API and returns a plain list. Use a
Series when the column needs a name and explicit labels:

| Function | Meaning |
|----------|---------|
| `series(values, name, index?)` | Construct a labeled Series; omitted index is `0..n-1` |
| `series_from(t, name)` / `as_series(t, name)` | Convert a DataFrame column to a Series |
| `series_values`, `series_name`, `series_index`, `series_shape`, `series_dtype` | Inspect Series metadata |
| `series_map` / `series_apply` | Apply a scalar function while preserving labels |
| `series_fillna` / `series_dropna` | Null handling while preserving surviving labels |
| `series_unique` / `series_value_counts` | Stable unique values or typed count map |
| `series_reindex` | Reorder or add labeled values with an explicit fill value |
| `series_add` / `series_sub` / `series_mul` / `series_div` | Label-align two Series; optional fill value handles missing labels |
| `index(t)` | Read the DataFrame index, defaulting to `0..n-1` |
| `index_name(t)` | Read the explicit index name, if present |
| `set_index(t, column, drop?)` | Move a column into explicit index metadata; drop defaults to true |
| `reset_index(t, name?)` | Materialize the index as a column; name defaults to `index` |
| `loc_labels(t, labels)` / `reindex(t, labels, fill?)` | Select or align rows by explicit labels |

The index model is explicit and single-level. `loc` remains positional
half-open slicing; label selection and MultiIndex semantics are not claimed.

```weft
names := df.col(t, "name")       // ["Alice", "Bob", ...]
top5 := df.head(t, 5)
subset := df.select_(t, ["name", "salary"])
```

## Filtering

| Function | |
|----------|--|
| `filter_(t, fn(row) -> bool)` | Keep rows where predicate is true |
| `query(t, col, op, value)` | Simple filter: `">"`, `"<="`, `"=="`, etc. |
| `isin(t, col, [values])` | Keep rows where col value is in list |
| `between(t, col, low, high)` | Keep rows where col is in range |
| `notnull(t, col)` | Keep rows where col is not null |

```weft
seniors := df.query(t, "age", ">=", 30)
eng := df.filter_(t, fn(r) { r.dept == "eng" })
selected := df.isin(t, "status", ["active", "pending"])
```

## Sorting

| Function | |
|----------|--|
| `sort_by(t, col, desc?)` | Sort by column (desc=true for descending) |
| `sort_by_multi(t, [cols], [desc])` | Multi-column sort |
| `nlargest(t, n, col)` | Top n rows by column |
| `nsmallest(t, n, col)` | Bottom n rows by column |

```weft
by_salary := df.sort_by(t, "salary", true)  // highest first
top3 := df.nlargest(t, 3, "salary")
```

## Column operations

| Function | |
|----------|--|
| `add_column(t, name, fn(row))` | Compute new column |
| `assign(t, {name: fn(row), …})` | Add multiple columns at once |
| `rename(t, {old: new, …})` | Rename columns |
| `apply_(t, col, fn(val))` | Transform one column |
| `apply_row(t, fn(row))` | Map function over rows, return list |
| `replace(t, col, old, new)` | Replace values in column |
| `clip_(t, col, lo, hi)` | Clamp column values |
| `fill_value(t, col, fill)` | Replace nulls in one column |

```weft
t2 := df.assign(t, {
    "bonus": fn(r) { r.salary * 0.1 },
    "senior": fn(r) { r.age >= 30 },
})
t3 := df.apply_(t, "name", fn(v) { str.upper(v) })
```

## Missing data

| Function | |
|----------|--|
| `fillna(t, {col: fill, …})` | Fill nulls per column |
| `dropna(t, subset?)` | Drop rows with nulls (subset = column list) |
| `isna(t, col)` | List of booleans |
| `count_na(t, col)` | Count nulls |

```weft
clean := df.dropna(t, ["age", "salary"])
filled := df.fillna(t, {"age": 0, "dept": "unknown"})
```

## Duplicates

| Function | |
|----------|--|
| `drop_duplicates(t, subset?)` | Remove duplicate rows |
| `duplicated(t, subset?)` | List of booleans (true = duplicate) |

## Grouping & aggregation

| Function | |
|----------|--|
| `group_by(t, col, aggs)` | Group and aggregate |
| `agg(t, aggs)` | Aggregate whole dataframe |
| `value_counts(t, col)` | Count occurrences per value |
| `nunique(t, col)` | Count unique values |

Aggregation ops: `"sum"`, `"mean"`, `"count"`, `"min"`, `"max"`, `"first"`, `"last"`, `"std"`. Nulls are skipped; `count` counts non-null values and `std` uses sample standard deviation.

```weft
summary := df.group_by(t, "dept", {
    "headcount": {"col": "name", "op": "count"},
    "avg_salary": {"col": "salary", "op": "mean"},
    "total_salary": {"col": "salary", "op": "sum"},
})
df.print_(summary, null)
```

## Statistics

| Function | |
|----------|--|
| `describe(t, col)` | count, mean, std, min, 25%, 50%, 75%, max, sum |
| `describe_all(t)` | describe for all numeric columns |
| `corr(t, col_a, col_b)` | Pearson correlation |
| `cov(t, col_a, col_b)` | Covariance |

```weft
stats := df.describe(t, "salary")
// {count: 100, mean: 85000, std: 12000, min: 55000, 25%: 75000, 50%: 85000, 75%: 95000, max: 120000, sum: 8500000}

r := df.corr(t, "age", "salary")  // 0.72
```

## Joins & merging

| Function | |
|----------|--|
| `join(left, right, on, how?)` | Join on same column name. `how`: `"inner"`, `"left"`, `"outer"`; overlapping right columns receive `_right` suffixes |
| `merge(left, right, left_on, right_on, how?)` | Join on different column names; supports `"inner"`, `"left"`, and `"outer"` |
| `concat_df([df1, df2, …])` | Concatenate vertically |

```weft
users := df.read_csv("users.csv", null)?
orders := df.read_csv("orders.csv", null)?
combined := df.join(users, orders, "user_id", "left")
```

## Reshaping

| Function | |
|----------|--|
| `pivot(t, index, column, value)` | Pivot table (rows → columns) |
| `melt(t, id_cols, value_cols?, var_name?, val_name?)` | Unpivot (columns → rows) |
| `crosstab(t, row_col, col_col)` | Cross-tabulation (frequency table) |

```weft
// pivot: one row per date, columns = metrics
p := df.pivot(metrics, "date", "metric", "value")
// date  cpu  mem  disk
// Mon   45   70   55
// Tue   50   65   58

// melt: columns back to rows
m := df.melt(wide, ["name"], null, "subject", "score")
```

## Window & rolling

| Function | |
|----------|--|
| `shift(t, col, periods?)` | Shift column values down (null fill) |
| `diff(t, col, periods?)` | Difference from previous row |
| `pct_change(t, col, periods?)` | Percentage change from previous |
| `rolling(t, col, window, op)` | Full-window aggregation; windows containing nulls return null |
| `expanding(t, col, op)` | Expanding (cumulative) aggregation |
| `rank(t, col)` | Add rank column |
| `cumsum_(t, col)` | Cumulative sum |

```weft
// 3-day moving average
smoothed := df.rolling(prices, "close", 3, "mean")

// daily returns
returns := df.pct_change(prices, "close", 1)

// running total
cumulative := df.cumsum_(sales, "revenue")
```

## Sampling

| Function | |
|----------|--|
| `sample(t, n?, seed?)` | Random sample of n rows; seed makes the result reproducible |
| `shuffle(t)` | Randomly reorder all rows |

## Output

| Function | |
|----------|--|
| `to_csv(t, sep?)` | CSV string |
| `to_json(t)` | JSON array string |
| `to_jsonl(t)` | JSONL string |
| `to_records(t)` | Raw row list |
| `to_dict(t)` | Column-oriented map |
| `write_csv(t, path, sep?)` | Write CSV file → Result |
| `write_json(t, path)` | Write JSON file → Result |
| `write_jsonl(t, path)` | Write JSONL file → Result |
| `print_(t, n?)` | Pretty-print with aligned columns |

```weft
df.print_(t, 10)
// name    age  dept   salary
// ------  ---  -----  ------
// Alice   30   eng    90000
// Bob     25   eng    75000
// ... 3 more rows
// [5 rows x 4 columns]

df.write_csv(t, "output.csv", null)?
```

## Limitations

- Pure Weft — row-list storage, not columnar. No universal row-count limit is promised; benchmark your workload.
- Sort uses the runtime comparator and is covered by the package scale tests.
- Indexes are single-level metadata; operations that construct a new frame may
  return a default positional index. `loc` is positional, not label-based.
- There is no MultiIndex, categorical dtype, timezone-aware datetime, or
  pandas extension-array protocol.
- `pivot` rejects duplicate index/column pairs; use an explicit aggregation before reshaping.
- No datetime parsing (use `time.*` stdlib manually).
- `corr`/`cov` are pairwise, not correlation matrices, and use pairwise non-null numeric rows.

## API count

103 exported functions, including aligned Series and explicit single-level
index APIs. The package currently has 82 regression tests.
