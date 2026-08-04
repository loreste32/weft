# Glue and scale benches

## Glue pairs (`make bench-glue`)

Paired Weft / Python3 scripts used by `make bench-glue` and CI **output parity** checks.

| Pair | Purpose |
|------|---------|
| `json_roundtrip` | API/agent-shaped JSON stringify + parse |
| `seq_map` | Sequential list transform |
| `str_split_join` | Split / join / upper loops |
| `fib` | Pure recursion (language core, not IO glue) |

Rules:

1. Each `.weft` and `.py` pair must print **byte-identical** stdout.
2. Workloads stay offline and deterministic (no network, no clocks).
3. CI fails on output drift; wall times are local-only (`make bench-glue`).
4. Optional Lua/LuaJIT: add `name.lua` and the shell script will time it if `luajit` is on PATH.

## Numerical micro-benches (`make bench-numerical`)

- `warp_matmul.weft` — fixed 64×64 matmul checksum
- `dataframe_groupby.weft` — 5k-row groupby smoke

Wall times only; not PR-gated on numbers.

## Scale budgets (`make bench-scale`)

Multi-fixture scale smoke. Writes `reports/scale-bench.json`.

| Fixture | Default size | Env knobs | Soft budget (ms) |
|---------|--------------|-----------|------------------|
| `warp_scale.weft` (matmul) | 256×256 | `WEFT_WARP_N`, `WEFT_WARP_MODE=matmul` | 120000 |
| `warp_scale.weft` (elementwise) | 100k sum | `WEFT_WARP_MODE=elementwise`, `WEFT_WARP_ELEMS` | 60000 |
| `dataframe_scale.weft` | 100k rows | fixed in fixture | 120000 |
| `dataframe_scale_1m.weft` | **250k** rows (1M optional) | `WEFT_DF_ROWS=1000000` for full million | 300000 |

### Budget policy

- **Soft by default:** exceeding a budget prints a warning and still exits 0 if
  the program ran successfully.
- **Strict:** `WEFT_SCALE_STRICT=1` fails the script on budget misses.
- **Always fail:** fixture crash / non-zero `weft run` exit / missing fixture.
- **RSS:** advisory peak RSS budget default 4 GiB when `/usr/bin/time` can
  measure it (`BUDGET_PEAK_RSS_KB`).

Override individual budgets:

```sh
BUDGET_WARP_MATMUL_MS=60000 \
BUDGET_DF_250K_MS=180000 \
bash scripts/bench-scale.sh
```

### Notes

- DataFrame storage is row-list pure Weft; 1M rows is intentionally optional
  because host memory and wall time grow quickly. Prefer the 250k default in CI.
- Warp matmul at N=256 is the default scale point; raise `WEFT_WARP_N` locally
  for larger CPU checks (script caps N at 2048 inside the fixture).
- Timing is recorded by the shell harness, not the Weft programs (programs
  only print JSON results / checksums).
