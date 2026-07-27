# Cookbook examples

Runnable companions to [docs/COOKBOOK.md](../../docs/COOKBOOK.md) and the [first-hour tutorial](../../docs/TUTORIAL.md).

All of these are **offline-friendly** (no network, no hanging HTTP server, no live LLM).

## Run one

```bash
# from repo root
./weft run examples/cookbook/01_hello.weft
./weft run examples/cookbook/12_cli.weft -- greet Ada
./weft test examples/cookbook -q
```

## Run all (smoke)

```bash
./weft eval examples/cookbook
# or:
for f in examples/cookbook/[0-9]*.weft; do
  echo "== $f =="
  if [[ "$f" == *12_cli* ]]; then
    ./weft run "$f" -- greet Ada
  else
    ./weft run "$f"
  fi
done
./weft test examples/cookbook -q
```

## Index

| File | Topic |
|------|--------|
| `01_hello.weft` | Minimal entry |
| `02_style.weft` | Bindings, loops, pipeline |
| `03_json.weft` | Parse / pretty JSON |
| `04_files.weft` | Temp file read/write |
| `05_errors.weft` | `Result`, `?`, ensure |
| `06_enums_match.weft` | Enum + match arms |
| `07_closures.weft` | By-value capture |
| `08_map_filter.weft` | Concurrent map/filter |
| `09_parallel.weft` | `parallel` fan-out |
| `10_channels.weft` | Channel + spawn |
| `11_path_import.weft` | `use "./lib/…"` |
| `12_cli.weft` | Flags / subcommand |
| `13_agent.weft` | `llm.chat` messages, `ask`+opts, `stream_text` (mock offline) |
| `14_mold.weft` | structured models: parse / extract / tool_params (`mold` module) |
| `lib/math.weft` | Small library for import + tests |
| `math_test.weft` | `weft test` sample |

## Not here (on purpose)

Live HTTP and long-running `http.serve` live under `examples/` root (`server.weft`, `webapp.weft`, …). `13_agent.weft` runs offline under the eval mock; for a live model set `WEFT_PROVIDER` / API keys.

`14_mold.weft` needs the optional module:

```bash
weft get mold ./packages/mold && weft install
./weft run examples/cookbook/14_mold.weft
```
