# Cluster, Governor & Supervisor

Production primitives for distributed deployments, token budgeting, and process supervision.

---

## Governor — token & cost budgeting

Prevent LLM agents from running away with your API budget.

```weft
fn main -> Result {
    gov := governor.new({
        "max_tokens": 8000,
        "max_duration_sec": 60,
        "max_cost_usd": 1.00,
        "max_steps": 10,
        "on_exceeded": "raise",
    })

    // agent loop
    mut step := 0
    while gov.check()? {
        step = step + 1
        reply := llm.chat("Step $step: continue the task")?

        // track usage after each call
        gov.track(150, 200, 0.002)?  // prompt_tokens, completion_tokens, cost

        say("step $step: $reply")
    }

    stats := gov.stats()
    say("total tokens: ${stats.total_tokens}")
    say("total cost: $${stats.cost_usd}")
    say("elapsed: ${stats.elapsed_sec}s")
}
```

### Governor options

| Option | Default | What it does |
|--------|---------|-------------|
| `max_tokens` | unlimited | Total token budget (prompt + completion) |
| `max_duration_sec` | unlimited | Wall-clock timeout |
| `max_cost_usd` | unlimited | Cost cap in USD |
| `max_steps` | unlimited | Max LLM calls |
| `on_exceeded` | `"raise"` | `"raise"` (error), `"truncate"` (return status), `"hangup"` (for telecom) |

### Governor methods

| Method | Returns |
|--------|---------|
| `gov.track(prompt, completion, cost?)` | Result — errors if budget exceeded |
| `gov.check()` | Result[bool] — true if still within budget |
| `gov.stats()` | Map with total_tokens, cost_usd, steps, elapsed_sec |
| `gov.remaining_tokens()` | Int (-1 if unlimited) |
| `gov.elapsed()` | Float (seconds) |
| `gov.cancel()` | Cancel the execution context |

### Governor with telecom

```weft
use telecom

fn main -> Result {
    gov := governor.new({
        "max_tokens": 4000,
        "max_duration_sec": 120,
        "on_exceeded": "hangup",
    })

    agent := telecom.iva({
        "system": "You are a phone assistant. Be concise.",
        "max_turns": 20,
    })

    telecom.webhook_server(8080, fn(event) {
        if !gov.check()? {
            return telecom.actions([
                telecom.speak("I'm sorry, our time is up. Goodbye.", null),
                telecom.hangup(null),
            ])
        }
        telecom.iva_handle_event(agent, event)?
    })
}
```

---

## Supervisor — process supervision

Erlang-style supervision for telecom and long-lived services.

```weft
fn call_handler(args) -> Result {
    say("handling call ${args[0]}")
    // simulate work
    time.sleep(5)
    Ok(unit)
}

fn media_processor(args) -> Result {
    say("processing media ${args[0]}")
    time.sleep(10)
    Ok(unit)
}

fn main -> Result {
    sup := supervisor.new({
        "strategy": "one_for_one",
        "max_restarts": 5,
        "within_seconds": 30,
    })

    sup.start_child("call-1", call_handler, ["session-abc"])?
    sup.start_child("media-1", media_processor, ["stream-xyz"])?

    say(sup.stats())
    // {total: 2, running: 2, restarts: 0, strategy: "one_for_one"}

    say(sup.children())
    // [{name: "call-1", running: true}, {name: "media-1", running: true}]

    // if call-1 crashes, only call-1 restarts (one_for_one)
    // if strategy is "one_for_all", all children restart
    // if "rest_for_one", call-1 and everything started after it restarts
}
```

### Supervision strategies

| Strategy | Behavior |
|----------|----------|
| `one_for_one` | Only the crashed child restarts |
| `one_for_all` | All children restart when one crashes |
| `rest_for_one` | Crashed child + all children started after it restart |

### Actor processes with mailboxes

```weft
fn main -> Result {
    proc := supervisor.process(fn() {
        // this runs in isolation
        while true {
            msg := proc.recv(10)?  // wait up to 10 seconds
            say("got message: $msg")
        }
    })

    proc.start()?
    proc.send("hello")?
    proc.send({"action": "process", "data": [1, 2, 3]})?

    time.sleep(1)
    say("alive: ${proc.alive()}")
    proc.stop()
}
```

---

## Cluster — distributed state

Scale across multiple Weft instances using Redis-backed shared state.

```weft
fn main -> Result {
    store := cluster.store("redis://localhost:6379")?

    // distributed lock
    lock := cluster.lock(store, "deploy-lock", {"ttl_sec": 30, "wait_sec": 10})?
    say("acquired lock: ${lock.key}")

    // do critical work...
    sh.run("make", ["deploy"])?

    lock.release()?
    say("lock released")
}
```

### Node registry

```weft
fn main -> Result {
    store := cluster.store("redis://localhost:6379")?

    // register this node
    node := cluster.register(store, "worker-1", {
        "ttl_sec": 30,
        "metadata": {"region": "us-east", "role": "ivr"},
    })?

    // heartbeat runs automatically every ttl/3 seconds

    // list active nodes
    nodes := cluster.nodes(store)?
    for n in nodes {
        say("node: ${n.id} (${n.metadata.region})")
    }

    // on shutdown
    node.deregister()?
}
```

### Distributed rate limiting

```weft
fn main -> Result {
    store := cluster.store("redis://localhost:6379")?

    // 100 requests per 60 seconds, shared across all instances
    allowed := cluster.rate_limit(store, "api-calls", 100, 60)?
    if !allowed {
        say("rate limited")
        return Ok(unit)
    }
    say("request allowed")
}
```

### Distributed counters

```weft
fn main -> Result {
    store := cluster.store("redis://localhost:6379")?

    counter := cluster.counter(store, "active-calls")?
    counter.incr()
    say("active calls: ${counter.get()}")

    // on call end
    counter.reset()
}
```

### Pub/sub

```weft
fn main -> Result {
    store := cluster.store("redis://localhost:6379")?

    // publish events to other instances
    cluster.publish(store, "call-events", json.stringify({
        "type": "call_started",
        "node": "worker-1",
        "call_id": "abc-123",
    }))?
}
```
