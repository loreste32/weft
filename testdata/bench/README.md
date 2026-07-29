# Glue bench pairs

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
