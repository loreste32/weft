# cron — channel-coordinated recurring tasks

`cron` schedules interval and wall-clock jobs without mutating shared maps
across `spawn`. Worker loops own their counters; the caller talks to them over
a command channel.

```bash
weft registry install cron   # monorepo path or git@tag
weft install
```

```weft
use cron

fn main -> Result {
    job := cron.every(1.0, fn() {
        say("tick")
        Ok(null)
    })
    time.sleep(2.5)
    st := cron.stats(job)
    say(st.runs, st.errors, st.running)
    cron.stop(job)
    // Optional: release the worker (stops answering stats).
    cron.close(job)
}
```

## Why channels (not shared maps)

Weft closures capture outer locals **by value** (deep copy at creation).
`spawn` args are deep-copied too. That means:

- A map created outside the worker and mutated inside the worker is **not**
  visible to the caller.
- Mutating the caller's job handle from the worker (or the reverse) cannot
  share live counters.
- The old “update a stats map from inside the tick loop” pattern is a silent
  no-op under deep-copy captures.

The fix is a small **control channel** per job. The worker owns
`runs` / `errors` / `last_run` / `running`. The public API sends messages on
that channel; the worker replies on a short-lived reply channel for stats.

See also [`docs/CONCURRENCY.md`](../../docs/CONCURRENCY.md).

## Channel control protocol

Each job handle holds a `cmd` channel. Payloads:

| Message | Effect |
|---------|--------|
| `"stop"` | Worker leaves the run loop; stays alive to answer `stats` until close. |
| `"close"` | Worker exits fully (after stop). Used by `cron.close`. |
| `{"op": "stats", "reply": ch}` | Worker `send`s a stats map on `reply`. |

`cron.stats` uses `select_recv` with a 2s timeout. If the worker does not
answer, the result includes `"stale": true` and zeroed counters.

After `stop`, the worker remains available for `stats` until `close` (or a
second `"stop"` while draining). `close` sends `"stop"` then `"close"`.

**Note:** Module-private helpers are not reliably callable from `spawn`
closures, so the run/drain loops are intentionally inlined in each spawn body.

## Public API

| Call | Role |
|------|------|
| `every(interval_sec, f)` | Recurring interval job. `interval_sec` must be positive. |
| `at(hour, minute, f)` | Daily wall-clock job (`hour` 0..23, `minute` 0..59). |
| `schedule([{name, interval\|hour, minute?, handler}, ...])` | Named multi-job scheduler. |
| `stop(job\|scheduler)` | Request stop; scheduler form delegates to `stop_all`. |
| `stop_all(scheduler)` | Stop every named job. |
| `stats(job\|scheduler)` | Live counters from the worker (or aggregated scheduler stats). |
| `close(job\|scheduler)` | Stop and tear down the worker (no further stats). |
| `wait` | Block forever (`while true { time.sleep(3600) }`) — keep a long-running process alive. |

Handlers should return `Ok(...)` or `Err(...)`. Errors are counted and logged
via `log.warn`; they do not stop the schedule.

### Job handle shape

```weft
{
    "_cron": true,
    "cmd": <channel>,
    "kind": "every" | "at",
    "meta": { /* interval or hour/minute */ },
    "running": true,   // caller-side flag; authoritative running is from stats
}
```

Scheduler:

```weft
{
    "_cron_scheduler": true,
    "jobs": { "name": <job handle>, ... },
    "running": true,
}
```

### Stats shape

Single job:

```weft
{"runs": 0, "errors": 0, "running": true, "last_run": null}
// or, on timeout: same fields plus "stale": true
```

Scheduler:

```weft
{
    "total_runs": 0,
    "total_errors": 0,
    "running": false,
    "jobs": [{"name": "a", "runs": 0, "errors": 0, "running": false, "last_run": null}, ...],
}
```

## Examples

### Interval job with stop and stats

```weft
use cron

fn main -> Result {
    probe := channel(8)
    job := cron.every(0.1, fn() {
        send(probe, "tick")
        Ok(null)
    })

    // Wait for at least one tick over a channel (shared across spawn).
    let mut ticks = 0
    let mut i = 0
    while i < 40 && ticks < 1 {
        msg := try_recv(probe)?
        if msg.ok { ticks = ticks + 1 }
        time.sleep(0.05)
        i = i + 1
    }

    st := cron.stats(job)
    say("runs", st.runs, "running", st.running)
    cron.stop(job)
    time.sleep(0.1)
    st2 := cron.stats(job)
    say("after stop", st2.running)  // false
    cron.close(job)
}
```

### Daily wall-clock job

```weft
job := cron.at(9, 30, fn() {
    // fires once per day near 09:30 local wall time
    Ok(null)
})
// ...
cron.stop(job)
```

### Multi-job scheduler

```weft
sched := cron.schedule([
    {"name": "heartbeat", "interval": 5.0, "handler": fn() {
        Ok(null)
    }},
    {"name": "morning", "hour": 8, "minute": 0, "handler": fn() {
        Ok(null)
    }},
])
st := cron.stats(sched)   // total_runs, jobs[]
cron.stop_all(sched)
// or: cron.close(sched)
```

### Long-running process

```weft
fn main {
    cron.every(60, fn() { say("minute"); Ok(null) })
    cron.wait   // never returns
}
```

## Boundaries

- Timing uses `time.sleep` in ~50ms steps; intervals are not real-time or
  high-precision.
- `at` schedules from local wall clock via `time.format` hour/minute fields;
  it does not parse time zones or DST tables beyond the host clock.
- This package coordinates in-process jobs only — not OS crontabs, distributed
  locks, or durable queues.
- Job handles are not serializable; do not put them in checkpoints or JSON.
