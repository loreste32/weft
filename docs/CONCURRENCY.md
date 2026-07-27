# Concurrency

Fan-out is normal here: agents and I/O often run several things at once. You don’t opt into a special async mode, and there are no `async`/`await` keywords.

Ordinary `fn` plus helpers like `parallel`, `race`, `timeout`, and `spawn` schedule work on the Go runtime under the VM.

**Closures:** function literals **do** capture outer locals **by value** (deep-copied at creation). That is safe under fan-out. You can still pass args into `spawn(fn, arg1, arg2)`; args are deep-copied too so tasks don’t share a mutable heap.

## Core API (prelude)

```weft
// Lists: map/filter fan out by default (order preserved)
// workers? optional; default WEFT_WORKERS or GOMAXPROCS
out := map(urls, fetch)           // concurrent
out := map(urls, fetch, 16)       // cap concurrency
out := seq_map(urls, fetch)       // sequential when side-effect order matters
kept := filter(items, ok)         // concurrent predicates
kept := seq_filter(items, ok)

// Fan-out independent work
results := parallel([
    fn() { http.get("https://a.example")? },
    fn() { http.get("https://b.example")? },
])?
results := gather([fn() { 1 }, fn() { 2 }])?   // alias of parallel

// First completed wins
v := race([fn() { slow() }, fn() { fast() }])?

// Deadline
v := timeout(2, fn() { slow_call() })?

// Background task
h := spawn(work, arg1, arg2)
raw := h.join()
ok  := h.await()?     // prefer for ? pipelines

// Task group
g := group()
g.go(fn() { 10 })
g.go(fn() { 20 })
rs := g.wait()?

// Channels
ch := channel(8)
spawn(producer, ch)
x := recv(ch)?
// non-blocking
peek := try_recv(ch)?   // {ok, value}
```

Env: `WEFT_WORKERS=N` sets default concurrency for `map` / `filter` / `par_map`.

## Agents

When the model returns **multiple tool calls** in one step, Weft runs them **concurrently** and reattaches results in order. You do not write an async agent framework.

## Rules of thumb

1. Prefer `parallel` / `par_map` for independent I/O.
2. Prefer `spawn` + channels for pipelines and producers.
3. Prefer `timeout` over ad-hoc sleep loops.
4. Never mutate shared maps/lists across tasks — pass copies or use channels.
5. Do **not** expect `async`/`await` keywords. They will not be added — that would reintroduce coloring after we eliminated it.

## See also

- [`docs/PRINCIPLES.md`](PRINCIPLES.md)
- Examples: `examples/parallel.weft`, `examples/channels.weft`
