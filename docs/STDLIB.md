# Stdlib overview (81 packages)

The standard library is **in the binary**. Packages are imported with `use name` (or just called as `name.member` after import — most scripts use `use`).

Live inventory:

```bash
weft stdlib           # all packages
weft stdlib http      # members of one package
```

This page is a map of what is there and when to reach for it. It is **broad and shallow** on purpose: good for glue and ops scripts.

What we keep vs won’t: **[STDLIB_GAPS.md](STDLIB_GAPS.md)**.

---

## By job

| Job | Packages |
|-----|----------|
| Files & paths | `fs`, `io`, `archive`, `compress` (gzip/zlib), `copy` |
| HTTP client / tiny server | `http`, `web`, `url`, `ws`, `webrtc` |
| JSON / config | `json`, `jsonl`, `yaml`, `toml`, `ini`, `xml` |
| Text / encoding | `str`, `re`, `html`, `base64`, `encoding` (hex, base32, URL), `mime`, `difflib` |
| Time | `time` |
| Env / process / OS | `env`, `os`, `platform`, `sh`, `shlex`, `signal`, `cli`, `log`, `secrets` |
| Numbers | `math`, `decimal`, `random`, `uuid`, `ip`, `binstruct` |
| Network / packets | `pcap`, `dns` (A/AAAA/SRV/CNAME/NS/MX/TXT/PTR), `tls` (cert inspection) |
| Data tables | `csv`, `table`, `db`, `redis`, `mongo` |
| Messaging | `nats`, `amqp`, `email`, `socket` |
| LLM | `llm`, `ollama`, `vllm` |
| AI integration | `mcp` (Model Context Protocol), `deepgram` (streaming STT), `elevenlabs` (streaming TTS), `mlinfer` (ONNX/Triton/HF inference) |
| Collections helpers | `iter`, `collections`, `heap`, `bisect`, `pipe`, `functools` |
| System info | `sysinfo` (CPU, memory, disk, uptime, interfaces) |
| Process mgmt | `proc` (list, find, kill, exists) |
| Network diag | `netutil` (port check, TCP ping, DNS, port scan) |
| Runtime / infra | `governor` (token/cost budgets), `supervisor` (Erlang-style), `cluster` (distributed state via Redis), `ratelimit`, `migrate` |
| ML / data | `tokenizer`, `dataset`, `metrics` |
| Crypto | `crypto` (sha256, hmac, argon2id, pbkdf2, random_bytes, uuid) |
| Errors | `traceback` |
| Charts | `viz` |
| GraphQL | `graphql` |
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

### pcap

Build and read PCAP packet captures — useful for ops, security, and network debugging scripts.

```weft
use pcap

// build a SYN packet
pkt := pcap.ethernet({
    "dst": "ff:ff:ff:ff:ff:ff",
    "src": "00:11:22:33:44:55",
    "payload": pcap.ipv4({
        "src": "10.0.0.1",
        "dst": "10.0.0.2",
        "payload": pcap.tcp({
            "src_port": 12345,
            "dst_port": 80,
            "flags": "SYN",
        }),
    }),
})
pcap.write("capture.pcap", [pkt])?

// read back
pkts := pcap.read("capture.pcap")?
say(len(pkts), "packets")
```

Builders: `ethernet`, `ipv4`, `tcp`, `udp`, `raw`, `hex`, `packet`.  
I/O: `write`, `read`.

TCP flags: `"SYN"`, `"ACK"`, `"SYN|ACK"`, `"FIN"`, `"RST"`, `"PSH"`, `"URG"` (pipe-separated).

### sysinfo

System metrics — cross-platform, structured, no subprocess parsing.

```weft
info := sysinfo.memory()?
say("RAM: ${info.percent}% used (${info.used} / ${info.total})")

disk := sysinfo.disk("/")?
say("Disk: ${disk.percent}% used")

up := sysinfo.uptime()?
say("Up ${up.human}")

load := sysinfo.loadavg()?
say("Load: $load")

ifaces := sysinfo.net_interfaces()?
for iface in ifaces { say(iface.name, iface.addrs) }

say(sysinfo.cpu_count())
say(sysinfo.env_summary())
```

Members: `memory`, `disk`, `uptime`, `loadavg`, `cpu_count`, `net_interfaces`, `env_summary`.

### proc

Process management — list, find, kill, check.

```weft
me := proc.self()
say("pid=${me.pid}")

proc.exists(1)?          // check if PID exists
procs := proc.list()?    // all processes
nginx := proc.find("nginx")?   // search by name
proc.kill(pid, "TERM")?  // signal a process
```

Members: `self`, `exists`, `list`, `find`, `kill`.

### netutil

Network diagnostics — port checks, TCP ping, DNS lookups, port scanning.

```weft
open := netutil.port_open("localhost", 8080)?
ping := netutil.tcp_ping("example.com", 443)?
say("latency: ${ping.latency_ms}ms")

ips := netutil.resolve("example.com")?
mx := netutil.lookup_mx("example.com")?
txt := netutil.lookup_txt("example.com")?
names := netutil.reverse_lookup("8.8.8.8")?

results := netutil.scan_ports("localhost", [22, 80, 443, 8080])?
for r in results { say("port ${r.port}: ${r.open}") }
```

Members: `port_open`, `tcp_ping`, `resolve`, `lookup_host`, `lookup_txt`, `lookup_mx`, `reverse_lookup`, `scan_ports`.

### mcp

Model Context Protocol — connect to MCP servers or expose Weft functions as MCP tools.

```weft
// connect to an MCP server
client := mcp.connect("npx", ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"])?
tools := client.list_tools()?
result := client.call_tool("read_file", {"path": "/tmp/data.txt"})?

// expose Weft functions as MCP tools
mcp.serve_stdio([
    mcp.tool("lookup_user", "Find a user by name", lookup_user),
    mcp.tool("check_balance", "Get account balance", check_balance),
])
```

Client: `connect`, `connect_sse` → `list_tools`, `call_tool`, `list_resources`, `read_resource`, `close`.
Server: `tool`, `serve_stdio`.

### deepgram

Streaming speech-to-text via Deepgram's WebSocket API. Low-latency, real-time transcription.

```weft
stream := deepgram.stream({"model": "nova-2", "language": "en", "interim_results": true})?
stream.send(audio_bytes)?
result := stream.recv()?
if result.is_final { say(result.transcript) }
stream.close()

// or REST for pre-recorded audio
result := deepgram.transcribe("https://example.com/call.wav")?
say(result.transcript)
```

Members: `stream`, `transcribe`. Env: `DEEPGRAM_API_KEY`.

### elevenlabs

Streaming text-to-speech via ElevenLabs WebSocket API. Low-latency audio generation.

```weft
// one-shot stream
stream := elevenlabs.stream("Hello, how can I help?", {"voice_id": "...", "output_format": "pcm_16000"})?
chunk := stream.recv()?  // {audio (base64), is_final}

// bidirectional (lowest latency)
ws := elevenlabs.stream_ws({"voice_id": "...", "output_format": "pcm_16000"})?
ws.send("Hello, ")?
ws.send("how are you?")?
ws.flush()
chunk := ws.recv()?

// REST
result := elevenlabs.speak("Hello", {"voice_id": "..."})?
voices := elevenlabs.voices()?
```

Members: `stream`, `stream_ws`, `speak`, `voices`. Env: `ELEVENLABS_API_KEY`.

### mlinfer

ML inference clients for ONNX Runtime, Triton, HuggingFace, and custom endpoints.

```weft
// generic
result := mlinfer.predict("http://model:8080/predict", {"text": "hello"})?

// ONNX Runtime Server
result := mlinfer.onnx("http://localhost:8001", "sentiment", {"text": "great"})?
healthy := mlinfer.onnx_health("http://localhost:8001")?

// Triton
result := mlinfer.triton("http://gpu:8000", "bert", {"inputs": [...]})?

// HuggingFace
result := mlinfer.hf("facebook/bart-large-mnli", {"inputs": "classify this"})?

// shortcuts
label := mlinfer.classify("http://localhost:8080/classify", "refund my order")?
vec := mlinfer.embed("http://localhost:8080/embed", "search query")?
results := mlinfer.batch("http://localhost:8080/classify", ["text1", "text2"])?
```

Members: `predict`, `onnx`, `onnx_health`, `onnx_models`, `triton`, `triton_health`, `triton_models`, `hf`, `classify`, `embed`, `detect`, `batch`.

### db / csv / table

SQLite-oriented `db`, CSV helpers, and table transforms for small data jobs. See [data.md](data.md).

### web / viz

Static files, SSE, simple multi-route apps: [web.md](web.md).  
Charts to SVG/HTML: [viz.md](viz.md).

---

## Gaps by design

- No heavy scientific arrays or dataframes  
- No full browser DOM  
- No every cloud vendor SDK  
- Messaging drivers (`redis`, `nats`, `amqp`, `mongo`) are thin connectors, not full clients  

If something is huge or domain-specific, prefer a **module** under `packages/` rather than growing the binary forever ([modules.md](modules.md)).

### Optional modules (not stdlib)

Not listed by `weft stdlib` — they live under `packages/` and install into `vendor/`.  
**Map of how they fit with `llm` / web:** [ECOSYSTEM.md](ECOSYSTEM.md).

| Module | Job | Doc |
|--------|-----|-----|
| `mold` | Structured models, LLM JSON validate, JSON Schema / tool params | [MOLD.md](MOLD.md) |
| `ml` | Embeddings, vectors, RAG index, metrics | [ML.md](ML.md) |
| `tokensave` | Context thrift, memory, teach → train | [`packages/tokensave`](../packages/tokensave/) |

```bash
weft packages list
weft get mold
```

---

## Discoverability

```bash
weft stdlib
weft stdlib fs
weft doctor          # env / optional backends
```

When writing examples for models or people, prefer **short real calls** over inventing APIs. If `weft stdlib pkg` does not list a member, it is not there.
