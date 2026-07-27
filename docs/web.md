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
| `app.before(fn)` | middleware; return response to short-circuit |
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
| `form` | form fields (first value per key) |
| `form_all` | form fields as lists (checkboxes / multi-select) |
| `files` | multipart uploads: `{field: {filename, content_type, size, body}}` |
| `cookies` | request cookies map |
| `htmx` | HTMX request map (see below) |

Helpers: `web.form` / `web.form_get` / `web.form_list` / `web.file` / `web.cookie_get`.

### Responses

```weft
web.text(200, "ok")
web.json({"a": 1})
web.json(201, {"created": true})
web.html("<p>hi</p>")
web.redirect("/login")
web.status(404, "nope")
web.sse(["hello", "world"])   // Server-Sent Events (flushed per chunk)
web.htmx("<div>partial</div>", {"trigger": "done"})  // HTMX fragment
```

Custom:

```weft
{"status": 200, "body": "…", "type": "text/plain", "headers": {"X-Trace": "1"}}
```

## HTMX

Weft treats HTMX as first-class: every request exposes `req.htmx`, and `web.htmx*` helpers set the response headers HTMX understands.

### Request (`req.htmx`)

| Field | Source header |
|-------|----------------|
| `request` (bool) | `HX-Request` |
| `boosted` (bool) | `HX-Boosted` |
| `history_restore` (bool) | `HX-History-Restore-Request` |
| `target` | `HX-Target` |
| `trigger` | `HX-Trigger` |
| `trigger_name` | `HX-Trigger-Name` |
| `current_url` | `HX-Current-URL` |
| `prompt` | `HX-Prompt` |

```weft
if web.is_htmx(req) {
    web.htmx("<li>item</li>")
} else {
    web.redirect("/")
}
```

### Response helpers

| Call | Effect |
|------|--------|
| `web.htmx(html, opts?)` | HTML fragment + optional HX-* headers |
| `web.htmx_redirect(url)` | `HX-Redirect` (client navigates) |
| `web.htmx_refresh()` | `HX-Refresh: true` |
| `web.htmx_trigger(event\|map, html?)` | `HX-Trigger` |
| `web.htmx_location(url\|opts)` | `HX-Location` soft nav |
| `web.htmx_cdn(version?)` | `<script src=unpkg htmx>` tag |
| `web.htmx_oob(id, html)` | fragment with `hx-swap-oob="true"` |

**`web.htmx` opts** (all optional):

| Opt | Header / effect |
|-----|-----------------|
| `trigger` / `trigger_after_settle` / `trigger_after_swap` | `HX-Trigger*` (str or map → JSON) |
| `redirect` | `HX-Redirect` |
| `refresh` | `HX-Refresh` |
| `location` | `HX-Location` (str or map → JSON) |
| `push_url` / `replace_url` | `HX-Push-Url` / `HX-Replace-Url` |
| `retarget` / `reswap` / `reselect` | matching HX-* |
| `oob` | str / list / `{id,html}` — OOB fragments appended to body |
| `cookie` / `cookies` | Set-Cookie (string or list) |
| `status` | HTTP status (default 200) |
| `headers` | extra response headers map |

### Cookies

```weft
web.cookie_get(req, "sid", "")
// response:
web.htmx(html, {"cookie": web.cookie("sid", "abc", {"max_age": 3600, "http_only": true})})
web.clear_cookie("sid")
```

Opts for `web.cookie`: `path`, `max_age`, `http_only`, `secure`, `same_site`.

Defaults: **HttpOnly=true**, **SameSite=Lax**, **Secure=false** (local HTTP). In production behind HTTPS, pass `"secure": true`. Cookie names reject `;` / CR/LF; OOB target ids are restricted to `[A-Za-z0-9_-]`.

Forms: `req.form` / `req.form_all` / `req.files` (multipart: body ≤ 32 MiB, part ≤ 8 MiB, max **1024** parts).

### Middleware

```weft
app.before(fn(req) {
    if web.cookie_get(req, "sid") == "" {
        return web.redirect("/login")   // short-circuit
    }
    null   // continue
})
```

Return a response map (`status` / `body` / `headers` / `cookies`) to stop; `null` / `false` continues.

`app.before` runs for **routes, static files, and WebSocket upgrades** — auth cannot be skipped via `/static` or WS alone.

### Files

```weft
f := web.file(req, "upload")
// f.filename, f.content_type, f.size, f.body
```

```weft
app.post("/save", fn(req) {
    web.htmx("<p class=\"ok\">saved</p>", {
        "trigger": {"saved": {"id": 1}},
        "push_url": "/items/1",
        "oob": [web.htmx_oob("#flash", "ok")],
    })
})
```

Demo: `weft run examples/htmx.weft` → http://127.0.0.1:8090

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
weft run examples/htmx.weft          # HTMX partials (counter + form)
weft run examples/chat.weft          # browser chat UI
weft run examples/webrtc_call.weft   # WebRTC room
weft run examples/server.weft        # minimal http.serve
```
