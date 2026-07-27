# Web apps & realtime (Weft)

Build HTTP APIs, HTML apps, WebSocket services, and **WebRTC** signaling — one binary, no framework stack.

| Package | Role |
|---------|------|
| `web` | App, routes, static, templates, response helpers |
| `http` | Client (`get`/`post`/`fetch`) + tiny `serve` |
| `ws` | WebSocket (via `app.ws`) |
| `webrtc` | Signaling hub for browser P2P realtime |

## Quick app

```weft
fn main {
    app := web.app()

    app.get("/", fn(req) {
        web.html("<h1>hello weft</h1>")
    })

    app.get("/api/users/:id", fn(req) {
        web.json({"id": req.params["id"]})
    })

    app.post("/api/echo", fn(req) {
        web.json({"body": req.body})
    })

    app.static("/static", "public")
    app.listen(":8080")
}
```

```bash
weft run examples/webapp.weft
```

### Routing

| Method | |
|--------|--|
| `app.get(path, handler)` | GET |
| `app.post` / `put` / `delete` / `patch` | |
| `app.route(method, path, handler)` | any method |
| `app.static(prefix, dir)` | file server |
| `app.templates(dir)` | `*.html` (Go `html/template`) |
| `app.render(name, data)` | HTML response |
| `app.ws(path, handler)` | WebSocket |
| `app.listen(addr)` / `app.run(addr)` | block & serve |
| `app.handle(method, path, body?)` | in-process (tests) |

Path params: `/users/:id` or `/users/{id}` → `req.params["id"]`.

### Request map

| Field | |
|-------|--|
| `method` `path` `query` `body` | basics |
| `params` | path params map |
| `query_map` | parsed query string |
| `headers` | request headers |
| `host` `remote` | connection meta |

### Responses

```weft
web.text(200, "ok")
web.json({"a": 1})
web.json(201, {"created": true})
web.html("<p>hi</p>")
web.redirect("/login")
web.status(404, "nope")
web.sse(["hello", "world"])   // Server-Sent Events (flushed per chunk)
```

Custom:

```weft
{"status": 200, "body": "…", "type": "text/plain", "headers": {"X-Trace": "1"}}
```

### SSE / token stream to callers

`web.sse(source)` returns a streaming `text/event-stream` response. `source` is a list or iterator of:

| Item | Written as |
|------|------------|
| `str` | `data: <str>\n\n` (or raw if already framed) |
| `{kind:"text", text}` | llm.stream event → `data: text` |
| `{kind:"done"}` | `data: [DONE]` |
| `{event, data}` | optional named event |

```weft
// proxy model tokens to the browser
app.get("/chat", fn(req) {
    events := llm.stream(req.body)?
    web.sse(events)
})

// fixed events
app.get("/ticks", fn(req) {
    web.sse(["one", "two", "three"])
})
```

Each chunk is **flushed** as it is pulled — the body is not buffered whole first.

## WebSocket

```weft
app.ws("/ws/echo", fn(conn) {
    conn.send("hi")?
    while true {
        msg := conn.recv()?
        conn.send("echo: $msg")?
    }
})
```

| `conn` | |
|--------|--|
| `send(text)` | → `Result` |
| `recv()` | → `Result[str]` (blocks) |
| `close()` | |
| `path` `params` `query` | |

Pure Go RFC6455 server — no external deps.

Example: `examples/chat.weft`.

## WebRTC (realtime A/V & data)

Weft is the **signaling server**. Browsers run `RTCPeerConnection`; media goes **peer-to-peer** (not through Weft). That is the usual way to build calls/meetings without a media SFU.

```weft
fn main {
    app := web.app()
    hub := webrtc.hub()
    hub.attach(app, "/ws/signal")?
    app.get("/rtc", fn(req) { web.html(page()) })
    app.listen(":8080")
}
```

### Signaling protocol (JSON over WebSocket)

**Client → server**

```json
{"type":"join","room":"lobby","peer":"alice"}
{"type":"offer","to":"bob","sdp":{...}}
{"type":"answer","to":"alice","sdp":{...}}
{"type":"ice","to":"bob","candidate":{...}}
{"type":"broadcast","payload":{"chat":"hi"}}
{"type":"leave"}
```

**Server → client**

```json
{"type":"welcome","peer":"alice","room":"lobby"}
{"type":"peers","peers":["bob"]}
{"type":"peer-joined","peer":"bob"}
{"type":"peer-left","peer":"bob"}
```

Offer/answer/ice are relayed with `"from"` set.

```weft
webrtc.ice_servers()   // default public STUN list for browser config
hub.rooms()
hub.peers("lobby")
```

Full demo: `examples/webrtc_call.weft` (two browser tabs → camera/mic P2P).

### Production notes

| Topic | Guidance |
|-------|----------|
| HTTPS / WSS | Terminate TLS (Caddy, nginx, or Go `ListenAndServeTLS`) in front of Weft |
| TURN | Add TURN URLs to `iceServers` for strict NATs |
| SFU / recording | Out of band (mediasoup, livekit, etc.); Weft still does signaling + APIs |
| Auth | Check tokens on HTTP; pass `?token=` on WS and validate in handler before `hub` |

## At a glance

| | Weft |
|--|------|
| Language | Weft (Go runtime, one binary) |
| Routes | `app.get` / `app.post` |
| Templates | `html/template` via `app.templates` |
| Realtime | `app.ws` + `webrtc.hub` built-in |
| Deploy | `weft run app.weft` |

## Examples

```bash
weft run examples/webapp.weft        # routes + API + ws echo
weft run examples/chat.weft          # browser chat UI
weft run examples/webrtc_call.weft   # WebRTC room
weft run examples/server.weft        # minimal http.serve
```
