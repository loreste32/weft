# Building applications with Weft

End-to-end examples of real applications. Each one runs as-is.

---

## 1. REST API with SQLite

A JSON API with CRUD, auth middleware, and a database:

```weft
fn main -> Result {
    c := db.open("sqlite:app.db")?
    c.exec("CREATE TABLE IF NOT EXISTS todos (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        title TEXT NOT NULL,
        done INTEGER DEFAULT 0
    )")?

    http.serve(":8080", fn(req) {
        match req.method + " " + req.path {
            "GET /todos" {
                rows := c.query("SELECT * FROM todos ORDER BY id")?
                http.json(rows)
            }
            "POST /todos" {
                body := json.parse(req.body)?
                c.exec("INSERT INTO todos(title) VALUES (?)", [body.title])?
                http.json({"ok": true}, 201)
            }
            "PUT /todos" {
                body := json.parse(req.body)?
                c.exec("UPDATE todos SET done = ? WHERE id = ?", [body.done, body.id])?
                http.json({"ok": true})
            }
            "DELETE /todos" {
                body := json.parse(req.body)?
                c.exec("DELETE FROM todos WHERE id = ?", [body.id])?
                http.json({"ok": true})
            }
            _ {
                http.text(404, "not found")
            }
        }
    })
}
```

```bash
weft run api.weft
curl -s localhost:8080/todos
curl -s -X POST localhost:8080/todos -d '{"title":"learn weft"}'
```

---

## 2. CLI tool with subcommands

A deployment tool with flags, subcommands, and colored output:

```weft
use color

fn main -> Result {
    p := cli.parse({
        "about": "deployctl — deploy and manage services",
        "flags": {
            "env": {"short": "e", "default": "dev", "help": "target environment"},
            "dry_run": {"short": "n", "bool": true, "help": "dry run"},
            "verbose": {"short": "v", "bool": true},
        },
        "commands": {
            "deploy": {"help": "deploy the service"},
            "status": {"help": "check service health"},
            "rollback": {"help": "roll back to previous version"},
        },
    })?

    if p.help || p.command == "" {
        say(p.usage)
        return Ok(unit)
    }

    match p.command {
        "deploy" {
            if p.dry_run {
                say(color.warn("dry run — no changes"))
                return Ok(unit)
            }
            say(color.info("deploying to ${p.env}..."))

            // check git status
            status := sh.capture("git", ["status", "--porcelain"])?
            if str.trim(status) != "" {
                say(color.fail("uncommitted changes — commit first"))
                cli.exit(1)
            }

            // run deploy
            r := sh.run("make", ["deploy"], {
                "env": {"DEPLOY_ENV": p.env},
                "timeout": "5m",
            })?
            if r.ok {
                say(color.ok("deployed to ${p.env}"))
            } else {
                say(color.fail("deploy failed: ${r.stderr}"))
                cli.exit(1)
            }
        }
        "status" {
            say(color.info("checking ${p.env}..."))
            mem := sysinfo.memory()?
            disk := sysinfo.disk("/")?
            say("  memory: ${mem.percent}%")
            say("  disk:   ${disk.percent}%")

            for port in [80, 443, 5432] {
                ping := netutil.tcp_ping("localhost", port)?
                if ping.open {
                    say(color.ok("  :$port  ${ping.latency_ms}ms"))
                } else {
                    say(color.fail("  :$port  down"))
                }
            }
        }
        "rollback" {
            say(color.warn("rolling back ${p.env}..."))
            sh.run("make", ["rollback"], {"env": {"DEPLOY_ENV": p.env}})?
            say(color.ok("rolled back"))
        }
        _ {
            say(color.fail("unknown command: ${p.command}"))
            cli.exit(2)
        }
    }
}
```

```bash
weft run deployctl.weft -- deploy -e staging
weft run deployctl.weft -- status
weft run deployctl.weft -- --help
```

---

## 3. LLM agent with tools

An agent that can search a database and send emails:

```weft
use mold

fn search_users(args) -> Result {
    c := db.open("sqlite:app.db")?
    c.query("SELECT name, email, role FROM users WHERE name LIKE ?",
        ["%${args.query}%"])?
}

fn send_notification(args) -> Result {
    say("sending email to ${args.email}: ${args.subject}")
    // email.send(args.email, args.subject, args.body)?
    Ok("sent")
}

fn main -> Result {
    reply := llm.ask("Find all admin users and send each one a reminder about the meeting tomorrow", [
        llm.tool("search_users", search_users, "Search users by name"),
        llm.tool("send_notification", send_notification, "Send email notification"),
    ], {
        "system": "You are an internal ops assistant. Use the tools to complete tasks.",
        "max_steps": 10,
    })?
    say(reply)
}
```

---

## 4. Voice agent (IVA) with telecom

An interactive voice agent for a restaurant reservation system:

```weft
use telecom

fn check_availability(args) -> Result {
    c := db.open("sqlite:reservations.db")?
    rows := c.query(
        "SELECT count(*) as cnt FROM reservations WHERE date = ? AND time = ?",
        [args.date, args.time]
    )?
    available := rows[0].cnt < 20
    Ok({"available": available, "remaining": 20 - rows[0].cnt})
}

fn make_reservation(args) -> Result {
    c := db.open("sqlite:reservations.db")?
    c.exec("INSERT INTO reservations(name, date, time, party_size, phone) VALUES (?,?,?,?,?)",
        [args.name, args.date, args.time, args.party_size, args.phone])?
    Ok("Reservation confirmed for ${args.name}, party of ${args.party_size} on ${args.date} at ${args.time}")
}

fn main -> Result {
    agent := telecom.iva({
        "system": "You are the reservation assistant for Luigi's Italian Restaurant. Help callers check availability and make reservations. Be warm and friendly. Always confirm the details before making a reservation.",
        "tools": [
            llm.tool("check_availability", check_availability, "Check if a table is available"),
            llm.tool("make_reservation", make_reservation, "Make a restaurant reservation"),
        ],
        "greeting": "Thank you for calling Luigi's Italian Restaurant! I can help you make a reservation. What date and time were you thinking?",
        "voice": "nova",
        "language": "en-US",
    })

    telecom.webhook_server(8080, fn(event) {
        telecom.iva_handle_event(agent, event)?
    })
}
```

---

## 5. Real-time voice pipeline (Deepgram + LLM + ElevenLabs)

Lowest-latency voice AI — stream STT, process with LLM, stream TTS:

```weft
fn main -> Result {
    // open streaming connections
    stt := deepgram.stream({
        "model": "nova-2",
        "language": "en",
        "encoding": "linear16",
        "sample_rate": 16000,
        "interim_results": true,
        "endpointing": 300,
    })?

    tts := elevenlabs.stream_ws({
        "voice_id": "21m00Tcm4TlvDq8ikWAM",
        "model": "eleven_turbo_v2_5",
        "output_format": "pcm_16000",
    })?

    say("voice pipeline ready — send audio to process")

    // process loop
    while true {
        // get transcription
        result := stt.recv()?
        if !result.speech_final { continue }
        if result.transcript == "" { continue }

        say("heard: ${result.transcript}")

        // stream LLM response to TTS
        for event in llm.stream(result.transcript, {"system": "Be concise.."})? {
            if event.kind == "text" {
                tts.send(event.text)?
            }
        }
        tts.flush()

        // collect audio chunks
        while true {
            chunk := tts.recv()?
            if chunk.is_final { break }
            // forward chunk.audio to caller's media stream
        }
    }
}
```

---

## 6. Monitoring dashboard

A web dashboard that shows system health with auto-refresh:

```weft
fn main -> Result {
    http.serve(":3000", fn(req) {
        match req.path {
            "/api/health" {
                mem := sysinfo.memory()?
                disk := sysinfo.disk("/")?
                up := sysinfo.uptime()?
                load := sysinfo.loadavg()?

                services := map([80, 443, 5432, 6379, 8080], fn(port) {
                    ping := netutil.tcp_ping("localhost", port)?
                    {"port": port, "open": ping.open, "latency_ms": ping.latency_ms}
                })?

                procs := proc.find("nginx")?

                http.json({
                    "memory": mem,
                    "disk": disk,
                    "uptime": up.human,
                    "load": load,
                    "services": services,
                    "nginx_procs": len(procs),
                })
            }
            _ {
                http.text(200, "<html><head><title>Health</title></head>
                <body><h1>System Health</h1>
                <pre id='data'>loading...</pre>
                <script>
                setInterval(() => fetch('/api/health').then(r=>r.json()).then(d=>{
                    document.getElementById('data').textContent = JSON.stringify(d, null, 2)
                }), 2000)
                </script></body></html>")
            }
        }
    })
}
```

---

## 7. Data pipeline (ETL)

Read CSV, transform, load into SQLite, generate report:

```weft
fn main -> Result {
    // extract
    rows := csv.read("sales.csv")?
    say("loaded ${len(rows)} rows")

    // transform
    c := db.open("sqlite:analytics.db")?
    c.exec("CREATE TABLE IF NOT EXISTS sales (
        date TEXT, product TEXT, quantity INTEGER, revenue REAL, region TEXT
    )")?

    // load
    mut total := 0.0
    for row in rows {
        c.exec("INSERT INTO sales VALUES (?,?,?,?,?)",
            [row.date, row.product, int.parse(row.quantity)?, float.parse(row.revenue)?, row.region])?
        total = total + float.parse(row.revenue)?
    }

    // report
    top := c.query("SELECT product, SUM(revenue) as rev FROM sales GROUP BY product ORDER BY rev DESC LIMIT 5")?
    say("\nTop products:")
    for p in top {
        say("  ${p.product}: $${p.rev}")
    }

    by_region := c.query("SELECT region, COUNT(*) as cnt, SUM(revenue) as rev FROM sales GROUP BY region")?
    say("\nBy region:")
    for r in by_region {
        say("  ${r.region}: ${r.cnt} sales, $${r.rev}")
    }

    say("\nTotal revenue: $${total}")
}
```

---

## 8. MCP tool server for DevOps

Expose your infrastructure as MCP tools for AI assistants:

```weft
fn server_health(args) -> Result {
    host := if args.host != null { args.host } else { "localhost" }
    mem := sysinfo.memory()?
    disk := sysinfo.disk("/")?
    up := sysinfo.uptime()?
    Ok({
        "host": host,
        "memory_pct": mem.percent,
        "disk_pct": disk.percent,
        "uptime": up.human,
    })
}

fn check_ports(args) -> Result {
    mut results := []
    for port in args.ports {
        p := netutil.tcp_ping(args.host, port)?
        push(results, {"port": port, "open": p.open, "ms": p.latency_ms})
    }
    Ok(results)
}

fn find_process(args) -> Result {
    proc.find(args.name)
}

fn run_query(args) -> Result {
    c := db.open(env.require("DATABASE_URL")?)?
    c.query(args.sql)?
}

fn main {
    mcp.serve_stdio([
        mcp.tool("server_health", "Check server memory, disk, uptime", server_health),
        mcp.tool("check_ports", "Check if ports are open", check_ports, {
            "type": "object",
            "properties": {
                "host": {"type": "string"},
                "ports": {"type": "array", "items": {"type": "integer"}},
            },
        }),
        mcp.tool("find_process", "Find running processes by name", find_process),
        mcp.tool("run_query", "Run a database query", run_query),
    ])
}
```

```bash
weft mcp serve devops-tools.weft
```

---

## 9. Webhook processor with retry

Process incoming webhooks with retry logic and logging:

```weft
use retry

fn process_event(event) -> Result {
    match event.type {
        "order.created" {
            say("new order: ${event.data.id}")
            // notify fulfillment
            retry.run(fn() -> Result {
                http.post("https://fulfillment.internal/orders", json.stringify(event.data))?
                Ok(unit)
            }, {"max_attempts": 3, "delay_ms": 1000})?
        }
        "payment.received" {
            say("payment: $${event.data.amount}")
        }
        _ {
            log.warn("unknown event", {"type": event.type})
        }
    }
    Ok(unit)
}

fn main -> Result {
    log.set_level("info")
    log.set_json(true)

    http.serve(":9000", fn(req) {
        if req.path != "/webhook" {
            return http.text(404, "not found")
        }

        event := json.parse(req.body)?
        log.info("webhook", {"type": event.type, "id": event.id})

        // process async
        spawn(fn() {
            r := process_event(event)
            if r.is_err {
                log.error("failed", {"error": r.err.message, "event": event.id})
            }
        })

        http.json({"received": true})
    })
}
```

---

## Project structure

A typical Weft project:

```text
myapp/
  main.weft          # entry point
  lib/
    db.weft          # database helpers
    auth.weft        # auth logic
  vendor/
    mold/            # installed from registry
    telecom/
  weft.json          # manifest + deps
  weft.lock          # lockfile
  main_test.weft     # tests
```

```bash
weft new app myapp           # scaffold
cd myapp
weft registry install mold   # add deps
weft run main.weft           # run
weft test                    # test
weft check --types           # type check
weft fmt                     # format
```
