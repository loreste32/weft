# Production notes (0.3)

Practical defaults for agents, small APIs, queue workers, and CLIs on one binary. This is not a full ops platform — just what the runtime already does for timeouts, secrets, and similar.

## Checklist

| Concern | How |
|---------|-----|
| HTTP hangs | Default client ~30s; per-call `timeout` / `timeout_ms` |
| Cancel work | Pass a context into `RunSource` / `RunFile`; HTTP and LLM respect it |
| Secrets in JSON | `Secret` prints as `***`; call `secrets.unwrap` only at the edge |
| Multi-write SQL | `conn.begin` / `conn.tx(fn)` |
| Queues | Redis brpop/blpop/subscribe; NATS/AMQP consume helpers |
| Web server | Read/write/idle timeouts on `web.listen` |
| Logs | `log.info` / `warn` / `error` / `debug`, optional map of fields |
| Hashing | `crypto.*` (sha256, hmac, …) |
| Simple validation | `re.*` |

## Patterns

### Worker

```bash
weft run examples/prod_worker.weft -- seed
weft run examples/prod_worker.weft -- once
# REDIS_URL=redis://127.0.0.1:6379/0 weft run examples/prod_worker.weft -- loop
```

### HTTP with limits

```weft
r := http.get(url, {"timeout": 5})?
if !r.ok { log.error("upstream", r.status); cli.exit(1) }

r := http.fetch({
    "url": url,
    "method": "POST",
    "body": json.stringify(payload),
    "headers": {"Authorization": "Bearer " + secrets.unwrap(token)},
    "timeout_ms": 2500,
    "retries": 3,
    "retry_ms": 100,
    "circuit": true,
    "circuit_threshold": 5,
    "circuit_cooldown": 30,
})?
```

| Opt | |
|-----|--|
| `timeout` / `timeout_ms` | deadline |
| `retries` / `retry_ms` | network + 429/502/503/504 |
| `circuit` / `_threshold` / `_cooldown` | optional per-host breaker |
| `form` / `files` | multipart |

### Transactions

```weft
// Closures capture outer locals by value (deep-copied at creation).
c.tx(fn(tx) {
    tx.exec("UPDATE accounts SET bal = bal - ? WHERE id = ?", [10, 1])?
    tx.exec("UPDATE accounts SET bal = bal + ? WHERE id = ?", [10, 2])?
    Ok(1)
})?
```

### Secrets

```weft
key := secrets.require("OPENAI_API_KEY")?
// printing / json → ***
http.get(url, {"headers": {"Authorization": "Bearer " + secrets.unwrap(key)}})
```

### Cancel from Go host

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
err := weftCtx.RunFile(ctx, "worker.weft")
```

## Deploy (sketch)

1. Build: `go build -o weft ./cmd/weft`
2. Ship the binary, your scripts, and `vendor/` if you use packages  
3. Configure with env (`DATABASE_URL`, `REDIS_URL`, API keys, …)  
4. Terminate TLS in front of `web.listen` if you expose HTTP  
5. Run workers under systemd/k8s as you would any long process  

## Still elsewhere

| Gap | Notes |
|-----|--------|
| ORM / migrations | Use SQL or your existing tools |
| Kafka / JetStream | Not in core; Redis/NATS cover many cases |
| Sandbox for untrusted code | Containers / separate processes |
| Multi-region failover | Infra, not the language |

Streaming: `llm.stream` reads SSE incrementally; `web.sse` can flush chunks to clients. Both are usable but not a full streaming product.

## Version and direction

0.3.30 on the 0.3.x line ([VERSIONING.md](VERSIONING.md)). For “what works today” vs “what we’re aiming at,” see [ROADMAP.md](ROADMAP.md).
