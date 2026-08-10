# dataframe — pandas-inspired tabular data for Weft

Pure `.weft` module with an optional `warp` dependency for explicit numeric
array interchange. It uses row-list storage, explicit ordered schema,
null-aware aggregation, grouping, joins, pivoting, rolling windows, and I/O.
It is designed for lightweight ETL analysis, not drop-in pandas compatibility.

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
| `from_csv_opts(text, sep?, opts)` | `from_csv` with explicit dtype/null policies → `Result` |
| `from_json(text)` | Parse JSON array → Result |
| `from_jsonl(text)` | Parse newline-delimited JSON → Result |
| `read_csv(path, sep?)` | Read CSV file → Result |
| `read_csv_opts(path, sep?, opts)` | `read_csv` with explicit dtype/null policies → `Result` |
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

### CSV type and null policies

`from_csv_opts` / `read_csv_opts` accept an opts map (Weft has no default
arguments, so the opts form is a separate function, like `merge_opts`):

- `dtypes`: `{col: "int" | "float" | "str" | "bool"}` — listed columns are
  parsed strictly; a violating value is an error naming the row and column.
  `"str"` keeps the raw text, so quoted numerals like `"045"` stay strings.
  `"float"` accepts integer literals (coerced, e.g. `3` → `3.0`).
- `null_values`: extra sentinel strings that map to `null`
  (e.g. `["NA", "nil", "-"]`).
- `keep_default_null_values`: default `true`. When `false`, the built-in
  sentinels (`""`, `"null"`, `"NULL"`, `"None"`) no longer map to `null` —
  only the custom `null_values` do.

Null sentinels apply to unquoted cells only; a quoted `"NA"` is an explicit
string. With no opts, parsing is byte-identical to `from_csv`.

```weft
t := df.read_csv_opts("data.csv", null, {
    "dtypes": {"id": "int", "score": "float", "tag": "str"},
    "null_values": ["NA", "-"],
})?
```

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
| `loc_label(t, row_sel, col_sel?)` | Label-based selection (pandas `.loc` semantics — see below) |
| `loc_set(t, row_sel, col_sel, value)` | Label-based assignment with broadcasting; returns a new frame |
| `iloc_set(t, row_sel, col_sel, value)` | Positional assignment with broadcasting; returns a new frame |
| `select_(t, [cols])` | Keep only listed columns |
| `drop(t, [cols])` | Remove listed columns |

#### Label selection and assignment

`loc_label` row selectors follow pandas `.loc`:

- **scalar label** — exact match (on a multi-level index: first-level match;
  a list is a partial prefix key). A unique match returns the row as a map
  (the cell value when `col_sel` is one column name, a restricted map for a
  column list); duplicate labels return all matching rows as a frame. An
  absent label is an `Err` naming it (pandas `KeyError`).
- **list of labels** — each label matched as above, in selector order;
  unknown labels are `Err`s.
- **list of booleans** — a positional mask whose length must equal the row
  count (`Err` "Boolean index has wrong length: X instead of Y" otherwise).
  An all-boolean list is always a mask, even on a boolean index (observed
  pandas 3.0 behavior).
- **`{"from": a, "to": b}`** — label slice, inclusive of both ends. Found
  bounds resolve positionally (first match for `from`, last for `to`), so
  slices work on unsorted indexes when both labels exist; a non-unique
  `from` on a single-level index is an `Err` (pandas wording). A missing
  bound uses insertion-point semantics on a monotonic index (either
  direction) and is an `Err` on a non-monotonic one. `null` ends are open.

`col_sel` is `null` (all columns), one column name, or a list of names.

`loc_set`/`iloc_set` return updated frames (frames are immutable) and accept
the same selectors (`iloc_set` takes integer positions / lists / `null` for
both axes). `value` may be a scalar (broadcast to the selection) or a list:
per-row when one column is selected and the length matches the selected
rows, per-column broadcast when the length matches the selected columns, or
a single-element list broadcast everywhere — all observed pandas 3.0
behaviors; anything else is an explicit `Err`. Assignment through a
duplicate label updates every matching row (pandas rule). An empty
selection is a no-op for scalars. Unlike pandas, unknown columns are not
created — an `Err` names the column.

### DataFrame ↔ Warp interchange

| Function | Meaning |
|----------|---------|
| `to_warp(t, columns?, dtype?)` | Copy selected non-null numeric columns into a packed 2-D Warp array; defaults to all columns and `float64` |
| `from_warp(a, columns?)` | Copy a 1-D or 2-D Warp array into a DataFrame; generated names are `column_0`, `column_1`, ... |

The boundary is intentionally explicit: DataFrames use row-list storage, so
conversion copies. `to_warp` rejects nulls and non-numeric values rather than
silently changing them; `from_warp` preserves array values but does not
promise zero-copy ownership.

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
| `align(left, right, join?, fill?)` | Return two DataFrames on the same outer/inner/left/right label index; supported profile assumes unique labels |
| `index(t)` | Read the DataFrame index, defaulting to `0..n-1` |
| `index_name(t)` | Read the explicit index name, if present (single-level) |
| `index_levels(t)` | Level names: multi → `_index_levels`, single named → `[name]`, default → `[null]` |
| `index_nlevels(t)` | Number of index levels (`1` for default/single-level frames) |
| `set_index(t, column, drop?)` | Move a column into explicit index metadata; drop defaults to true |
| `set_multi_index(t, columns, drop?)` | Minimal multi-level index from a list of columns; labels stored as lists |
| `reset_index(t, name?)` | Materialize the index as column(s); multi uses level names; single name defaults to `index` |
| `loc_labels(t, labels)` / `reindex(t, labels, fill?)` | Select or align rows by explicit labels |

The index model is explicit. Single-level indexes remain the common path.
`set_multi_index` is a **minimal** multi-level foundation (not a full pandas
MultiIndex): each row label is a list of values and level names live in
`_index_levels`. `loc_labels` accepts full multi-keys, a prefix list, or a
scalar first-level match; `reindex` requires exact full keys. `loc` remains
positional half-open slicing; `loc_label`/`loc_set` provide the label-based
pandas semantics (partial keys included).

### Index set operations, index sorting & cross-sections

pandas-matching index algebra (verified against pandas 3.0.1). The set
operations accept DataFrames (operating on the current index) or plain label
lists, and always return a plain list of labels:

| Function | Meaning |
|----------|---------|
| `index_union(a, b)` | Sorted unique labels; duplicate inputs keep max(left, right) multiplicity per label. Union with an empty side returns the other side as-is (unsorted) |
| `index_intersection(a, b)` | Sorted unique labels present on both sides; duplicates always collapse |
| `index_difference(a, b)` | Sorted unique labels of the left side absent from the right |
| `sort_index(t, desc?)` | Stable sort of rows by index label; multi-level sorts level by level, descending flips every level |
| `sort_index_opts(t, opts)` | `opts.ascending` (default true), `opts.na_position` (`"last"` default, `"first"`) |
| `is_monotonic(t, desc?)` | Whether the index is monotonically increasing (desc truthy → decreasing), non-strict |
| `xs(t, key, level?)` | Cross-section: rows where one level equals key, other levels free; selected level is dropped from the index (pandas default) |
| `xs_opts(t, key, opts)` | `opts.level` (int/str, default 0), `opts.drop_level` (default true) |

Nulls are ordinary labels: they participate in set membership (null matches
null) and sort last regardless of direction (`na_position="last"`), matching
pandas. Mixed-type labels use pandas' "safe" ordering — numbers/bools first
(bools compare as 0/1), then strings, then other values by JSON encoding.
Unlike pandas, `sort_index` on mixed-type labels does **not** raise; it
applies the same safe ordering (documented deviation). Any null label makes
`is_monotonic` false, also matching pandas.

`xs` takes a scalar key (with `level`, default 0) or a map `{level: key}`
matching several levels at once (map keys are level names or numeric
strings). Following pandas `xs`: the selected levels are dropped by default
(`drop_level=false` keeps them), selecting every level leaves a default
positional index, and a key absent from its level is an Err (pandas raises
`KeyError`) — not an empty frame.

```weft
u := df.index_union(t1, t2)              // sorted unique labels
s := df.sort_index(t, null)              // ascending by index
x := df.xs(mi, "A", null)                // level 0 == "A", any level 1
x2 := df.xs(mi, {"g": "A", "n": 2}, null) // multi-level map key
```

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

## Resampling

`resample(t, time_column, rule, aggs, origin?)` groups finite numeric
timestamps measured in seconds into fixed-width bins and applies the same
aggregation map accepted by `group_by`. `rule` may be a positive number or a
string such as `"10s"`, `"5m"`, `"1h"`, or `"1d"`. Bins are sorted by their
numeric start time. Datetime parsing, timezone-aware values, calendar-aware
offsets, and empty-bin materialization are not claimed yet.

```weft
binned := df.resample(events, "timestamp", "1m", {"latency": "mean"}, null)?
```

## Sorting

| Function | |
|----------|--|
| `sort_by(t, col, desc?)` | Sort by column (desc=true for descending) |
| `sort_by_multi(t, [cols], [desc])` | Multi-column sort |
| `sort_index(t, desc?)` / `sort_index_opts(t, opts)` | Stable sort by index label(s); see index section |
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
| `rolling(t, col, window, op)` | Full-window aggregation; windows containing nulls return null. Ops: `sum mean count min max first last std var` |
| `expanding(t, col, op)` | Expanding (cumulative) aggregation over the same op set |
| `ewm_mean(t, col, opts)` | Exponentially-weighted mean (pandas 3.0 semantics) |
| `ewm_sum(t, col, opts)` | Exponentially-weighted sum; `adjust=true` only (pandas rule) |
| `ewm_var(t, col, opts)` | Exponentially-weighted variance, `bias=false` default |
| `ewm_std(t, col, opts)` | Exponentially-weighted standard deviation |
| `rank(t, col)` | Add rank column |
| `cumsum_(t, col)` | Cumulative sum |

`ewm_*` opts mirror pandas: exactly one of `alpha` / `span` / `halflife`
(`alpha = 2/(span+1)`, `alpha = 1 - exp(-ln2/halflife)`), plus `adjust`
(default `true`), `ignore_na` (default `false`), and `bias` (var/std only,
default `false`). The column is replaced in place, like `rolling`. Nulls
before the first observation stay null; later nulls hold the previous value.

```weft
// 3-day moving average
smoothed := df.rolling(prices, "close", 3, "mean")

// exponentially-weighted mean with a 3-sample span
ewm := df.ewm_mean(prices, "close", {"span": 3})

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

## SQL bridge

Via the stdlib `db` package (SQLite supported; any `db.open` conninfo or
connection handle works as `source`):

| Function | |
|----------|--|
| `read_sql(source, query)` | Run a query → DataFrame `Result` |
| `to_sql(t, source, table, opts?)` | Write a DataFrame → `Result` |

```weft
conn := db.open("sqlite::memory:", {"max_open": 1})?
df.to_sql(t, conn, "people", {"if_exists": "replace"})?
back := df.read_sql(conn, "SELECT * FROM people")?
```

- `source` may be a connection handle or a conninfo string such as
  `"sqlite:data.db"`. A conninfo string is opened with `max_open: 1` (so
  `":memory:"` stays on one connection) and closed before returning.
- `to_sql` opts: `if_exists` is `"fail"` (default), `"replace"`, or
  `"append"`. The write runs in one transaction; any failure rolls back.
- Table and column names are always quoted as SQL identifiers (double
  quotes, embedded quotes doubled); values are bound parameters. Neither is
  ever interpolated into SQL raw.
- Column order from `read_sql` follows the query's SELECT list. An empty
  result yields an empty DataFrame with no columns (rows carry the schema).

Type mapping (SQLite):

| Weft | to_sql DDL | read_sql |
|------|-----------|----------|
| `int` | `INTEGER` | `int` |
| `float` | `REAL` | `float` |
| `str` | `TEXT` | `str` |
| `bool` | `BOOLEAN` | `int` (1/0 — SQLite has no boolean storage class) |
| `null` | `NULL` | `null` |
| mixed `int`+`float` | `REAL` | `float` |
| other mixes, lists, maps | explicit `Err` | BLOB → `str` |

All-null columns are declared `TEXT`. Inherited caveat from the `db` stdlib:
TEXT values that look like a JSON object/array are auto-parsed into
maps/lists on read, so such strings do not round-trip.

## Limitations

- Pure Weft row-list storage, not columnar. Warp interchange is an explicit
  copying boundary; no zero-copy columnar path is claimed yet. No universal
  row-count limit is promised; benchmark your workload.
- Sort uses the runtime comparator and is covered by the package scale tests.
- Indexes are metadata on the row list. Single-level is complete; multi-level
  (`set_multi_index`) is minimal — no hierarchical grouping or advanced
  pandas MultiIndex APIs. `swaplevel`/`droplevel`, index set operations
  (`index_union`/`index_intersection`/`index_difference`), `sort_index`, and
  `xs` cross-sections are supported. Operations that construct a new frame
  may drop to a default positional index. `loc` is positional; label-based
  selection/assignment lives in `loc_label`/`loc_set` (a bare list is always
  a label list, so a full multi-key is passed wrapped: `[["a", 1]]`).
- There is no categorical dtype, timezone-aware datetime, or pandas
  extension-array protocol.
- `pivot` rejects duplicate index/column pairs; use `pivot_table(t, {index, columns, values, aggfunc, fill_value?})` to aggregate duplicates instead.
- No datetime parsing or timezone-aware resampling (use numeric seconds from
  `time.*` manually).
- `corr`/`cov` are pairwise, not correlation matrices, and use pairwise non-null numeric rows.

## API count

129 exported functions, including aligned Series/DataFrames, numeric-second
resampling, single-level index APIs, index set operations, `sort_index`,
`xs` cross-sections, and a minimal multi-level index foundation. Regression tests live in
`packages/dataframe/dataframe_test.weft` and
`packages/dataframe/sql_test.weft` (SQL bridge + CSV policies).
