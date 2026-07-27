# Real pipelines in Weft

ETL-style **extract → transform → load** without a heavy data-frame stack.  
Compose with **`map` / `filter` (concurrent by default)** / `reduce`, **table.*** ops, **jsonl**.

## Mental model

```text
source (jsonl/csv/db/http)
    → filter / where
    → map / enrich
    → join / group / sort
    → sink (jsonl/csv/db/viz)
```

`|>` still works for single-value steps: `x |> f |> g`.

## List transforms (globals)

Callbacks are **named functions or `fn(x){…}` without outer locals**.

```weft
fn double(x) { x * 2 }
fn even(x) { x % 2 == 0 }
fn add(a, b) { a + b }

xs := [1, 2, 3, 4, 5]
map(xs, double)           // concurrent fan-out, order kept
map(xs, double, 8)        // cap workers
seq_map(xs, double)       // sequential (side-effect order)
filter(xs, even)          // concurrent predicates
seq_filter(xs, even)
reduce(xs, 0, add)        // sequential (dependent)
flat_map(xs, fn(x) { [x, x] })
each(xs, fn(x) { say(x) })  // sequential side effects
par_map(xs, double, 8)    // same as map(..., 8)
find(xs, even)
any(xs, even)  all(xs, even)  // list any/all — not task gather
```

Default workers: `WEFT_WORKERS` or `GOMAXPROCS` (min 2).

Also: `push` `pop` `slice` `concat` `range` `contains` `keys` `values` `delete`.

## Table ops (list of maps — no callbacks)

```weft
rows := jsonl.read("users.jsonl")?

active := table.where_truthy(rows, "ok")
active := table.where_eq(rows, "role", "admin")
active := table.where_ne(rows, "role", "guest")

cols   := table.project(active, ["name", "role"])
names  := table.pluck(active, "name")
ranked := table.sort(active, "name", false)   // desc: true
uniq   := table.unique(active, "email")
groups := table.group_by(active, "role")      // map role -> [rows]

top    := table.take(ranked, 10)
rest   := table.drop(ranked, 10)
tagged := table.set(active, "source", "etl")
renamed := table.rename(active, "name", "full_name")

// left-join on key
joined := table.merge(users, orgs, "org_id")
```

## JSONL

```weft
rows := jsonl.read("in.jsonl")?
jsonl.write("out.jsonl", rows)?
jsonl.append("out.jsonl", {"ok": true})?
text := jsonl.stringify(rows)
rows := jsonl.parse(text)?
```

## pipe helpers

```weft
pipe.batch(rows, 100)     // [[…], […], …]
pipe.flatten(nested)
pipe.compact(list)        // drop null
pipe.zip(a, b)
pipe.enumerate(list)      // [[0,x],[1,y],…]
pipe.map / filter / reduce / par_map   // aliases
```

## End-to-end example

```bash
weft run examples/pipeline_etl.weft
# → examples/data/out/active_users.jsonl
# → examples/data/out/active_users.csv
# → examples/data/out/roles.html

weft run examples/pipeline_parallel.weft
```

```weft
rows := jsonl.read("examples/data/users.jsonl")?
active := table.where_truthy(rows, "ok")
slim := table.project(active, ["name", "role"])
enriched := map(slim, with_score)          // named fn
ranked := table.sort(enriched, "score", true)
jsonl.write("out.jsonl", ranked)?
```

## With DB / HTTP / queues

```weft
// extract from SQL
c := db.open("sqlite:./app.db")?
rows := c.query("SELECT id, name, active FROM users")?

// transform
active := table.where_truthy(rows, "active")

// load
jsonl.write("export.jsonl", active)?

// or fan-out work
par_map(active, process_user, 16)

// or push to Redis for workers
r := redis.connect(env.get("REDIS_URL") ?? "redis://127.0.0.1:6379/0")?
each(active, fn(row) {
    r.lpush("jobs", json.stringify(row))
})
```

## Design notes

| | |
|--|--|
| Closures | **No outer local capture** — use top-level `fn` or pure `fn(x){…}` |
| Memory | `jsonl.read` loads the file; for huge data, batch with SQL/`pipe.batch` + workers |
| Parallelism | **default:** `map`/`filter`; also `gather`/`spawn`/`race`/`timeout` — see CONCURRENCY.md |
| Types | Rows are maps; CSV header mode also yields maps |

## Related

- [`docs/cli.md`](cli.md) — flags, sh, fs  
- [`docs/data.md`](data.md) — db/redis/nats/…  
- [`docs/PRODUCTION.md`](PRODUCTION.md) — timeouts, secrets, workers  
