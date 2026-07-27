# Stdlib overview

The standard library is **in the binary**. Packages are imported with `use name` (or just called as `name.member` after import — most scripts use `use`).

Live inventory:

```bash
weft stdlib           # all packages
weft stdlib http      # members of one package
```

This page is a map of what is there and when to reach for it. It is **broad and shallow** on purpose: good for glue and ops scripts.

Coverage tiers and non-goals: **[STDLIB_GAPS.md](STDLIB_GAPS.md)**.

---

## By job

| Job | Packages |
|-----|----------|
| Files & paths | `fs`, `io`, `archive`, `copy` |
| HTTP client / tiny server | `http`, `web`, `url`, `ws`, `webrtc` |
| JSON / config | `json`, `jsonl`, `yaml`, `toml`, `ini`, `xml` |
| Text | `str`, `re`, `html`, `base64`, `mime`, `difflib` |
| Time | `time` |
| Env / process | `env`, `platform`, `sh`, `shlex`, `signal`, `cli`, `log`, `secrets` |
| Numbers | `math`, `decimal`, `random`, `uuid`, `ip`, `binstruct` |
| Data tables | `csv`, `table`, `db`, `redis`, `mongo` |
| Messaging | `nats`, `amqp`, `email`, `socket` |
| LLM | `llm`, `ollama`, `vllm` |
| Collections helpers | `iter`, `collections`, `heap`, `bisect`, `pipe`, `functools` |
| Crypto | `crypto` |
| Charts | `viz` |
| GraphQL | `graphql` |
| Errors | `traceback` |
| Tests | `test` (no import required in tests) |
| Pickle-like | `pickle` (limited) |

---

## Prelude (no import)

Always available (non-exhaustive):

| Name | Role |
|------|------|
| `say` / `println` | Print |
| `Ok` / `Err` | Results |
| `len` / `push` / `range` | Basics |
| `map` / `seq_map` / `filter` / `seq_filter` | List transform (map/filter concurrent by default) |
| `reduce` / `each` / `par_map` | More pipelines |
| `find` / `any` / `all` / `sort` / `reverse` / `unique` | Queries |
| `zip` / `flatten` / `enumerate` / `count` | Shape |
| `spawn` / `parallel` / `gather` / `race` / `timeout` / `group` | Concurrency |
| `channel` / `send` / `recv` / `close` / `try_recv` / `select_recv` | Channels |
| `ensure` / `bail` | Errors |
| `int.parse` (and friends as exposed) | Parsing helpers where registered |

`WEFT_WORKERS=N` caps default concurrency for `map` / `filter`.

---

## Package notes

### fs

Read/write files, paths, walk, temp files, glob.

```weft
text := fs.read("a.txt")?
fs.write("b.txt", text + "\n")?
paths := fs.glob("**/*.weft")
```

Common: `read`, `write`, `append`, `exists`, `list`, `mkdir`, `join`, `base`, `dir`, `ext`, `glob`, `rglob`, `walk`, `temp_file`, `cwd`, `abs`, `rel`.

### http

```weft
r := http.get("https://example.com")?
say(r.status, r.body)
data := http.get_json("https://httpbin.org/json")?   // GET + parse
http.serve(":8080", fn(req) {
    http.json({"ok": true, "path": req.path})
})
```

Client: `get`, `get_json`, `post`, `put`, `patch`, `delete`, `fetch`, `post_form`.  
Server helpers: `serve`, `text`, `json`.

### json

```weft
data := json.parse(raw)?
s := json.stringify(data)
pretty := json.pretty(data)
name := json.get(data, "user.name", "anon")?   // default if missing
```

Also: `set`, `has`, `merge`, `clone`.

### str / re / time / env

```weft
say(str.upper("hi"))
say(str.starts_with("hello", "he"))   // alias: has_prefix
say(str.split("a,b,c", ","))
m := re.find(r"\d+", "ab12cd")
say(time.iso(time.now()))
home := env.get("HOME", "/tmp")       // optional default
```

### llm

Provider-agnostic when `WEFT_PROVIDER` / env is set.

```weft
reply := llm.chat("Explain Result in one line")?
reply := llm.chat("hi", {"system": "Be terse."})?
reply := llm.chat([{"role": "user", "content": "hi"}])?
reply := llm.ask("Use tools", [llm.tool("add", add)])?
reply := llm.ask("2+3?", [llm.tool("add", add)], {"max_steps": 6})?
text  := llm.stream_text("one word: hi")?
```

Members: `chat`, `ask`, `agent`, `tool`, `stream`, `stream_text`, `extract`, `client`.  
See [LLM_PROVIDERS.md](LLM_PROVIDERS.md) and [LLM_LOCAL.md](LLM_LOCAL.md).

### cli

```weft
p := cli.parse({
    "about": "my tool",
    "flags": {
        "verbose": {"short": "v", "bool": true},
    },
})?
if p.help { say(p.usage); cli.exit(0) }
```

See [cli.md](cli.md).

### test

Used by `weft test` — typically no `use test` needed.

```weft
fn test_math {
    test.eq(1 + 1, 2)
    test.is_true(len([1]) == 1)
}
```

See [TESTING.md](TESTING.md).

### db / csv / table

SQLite-oriented `db`, CSV helpers, and table transforms for small data jobs. See [data.md](data.md).

### web / viz

Static files, SSE, simple multi-route apps: [web.md](web.md).  
Charts to SVG/HTML: [viz.md](viz.md).

---

## Gaps by design

- No NumPy/pandas  
- No full browser DOM  
- No every cloud vendor SDK  
- Messaging drivers (`redis`, `nats`, `amqp`, `mongo`) are thin connectors, not full clients  

If something is huge or domain-specific, prefer a **module** under `packages/` rather than growing the binary forever ([modules.md](modules.md), [ML.md](ML.md)).

---

## Discoverability

```bash
weft stdlib
weft stdlib fs
weft doctor          # env / optional backends
```

When writing examples for models or people, prefer **short real calls** over inventing APIs. If `weft stdlib pkg` does not list a member, it is not there.
