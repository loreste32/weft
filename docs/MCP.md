# MCP (Model Context Protocol)

Connect Weft to AI assistants or expose Weft functions as tools that any MCP-compatible client can call.

```weft
use mcp
```

The `mcp` stdlib package supports both directions:
- **Server**: expose Weft functions as MCP tools (for AI assistants to call)
- **Client**: connect to existing MCP servers and use their tools from Weft

---

## MCP server — expose Weft tools

Write functions, wrap them with `mcp.tool`, serve on stdio. Any MCP client (AI assistants, IDE extensions, automation) can discover and call your tools.

```weft
fn lookup_user(args) -> Result {
    db := db.open("sqlite:app.db")?
    db.query("SELECT * FROM users WHERE name = ?", [args.name])?
}

fn check_server(args) -> Result {
    mem := sysinfo.memory()?
    disk := sysinfo.disk("/")?
    Ok({"memory_pct": mem.percent, "disk_pct": disk.percent})
}

fn run_query(args) -> Result {
    db := db.open("sqlite:app.db")?
    db.query(args.sql)?
}

fn main {
    mcp.serve_stdio([
        mcp.tool("lookup_user", "Find a user by name", lookup_user, {
            "type": "object",
            "properties": {
                "name": {"type": "string", "description": "User name to look up"},
            },
        }),
        mcp.tool("check_server", "Get system health metrics", check_server),
        mcp.tool("run_query", "Run a SQL query on the app database", run_query, {
            "type": "object",
            "properties": {
                "sql": {"type": "string", "description": "SQL query to execute"},
            },
        }),
    ])
}
```

Run it:

```bash
weft mcp serve tools.weft
```

### Adding to an AI assistant

Point your MCP client at the weft command:

```json
{
  "mcpServers": {
    "my-tools": {
      "command": "weft",
      "args": ["mcp", "serve", "/path/to/tools.weft"]
    }
  }
}
```

The AI assistant will see `lookup_user`, `check_server`, and `run_query` as available tools and can call them during conversations.

### Tool schemas

Pass a JSON Schema as the 4th argument to `mcp.tool` to describe the input parameters. This helps the AI understand what arguments to pass:

```weft
mcp.tool("create_user", "Create a new user account", create_user, {
    "type": "object",
    "properties": {
        "name": {"type": "string"},
        "email": {"type": "string", "format": "email"},
        "role": {"type": "string", "enum": ["admin", "user", "guest"]},
    },
    "required": ["name", "email"],
})
```

---

## MCP client — use external tools

Connect to any MCP server and call its tools from your Weft scripts.

### Stdio connection

```weft
fn main -> Result {
    // connect to a filesystem MCP server
    client := mcp.connect("npx", [
        "-y", "@modelcontextprotocol/server-filesystem", "/tmp",
    ])?

    // list available tools
    tools := client.list_tools()?
    for t in tools {
        say("tool: ${t.name} — ${t.description}")
    }

    // call a tool
    result := client.call_tool("read_file", {"path": "/tmp/data.txt"})?
    say(result)

    // list resources
    resources := client.list_resources()?
    say(resources)

    // read a resource
    content := client.read_resource("file:///tmp/data.txt")?
    say(content)

    client.close()
}
```

### HTTP+SSE connection

```weft
fn main -> Result {
    client := mcp.connect_sse("https://mcp.example.com")?

    tools := client.list_tools()?
    say(tools)

    result := client.call_tool("search", {"query": "weft language"})?
    say(result)
}
```

### Client methods

| Method | What it does |
|--------|-------------|
| `client.list_tools()` | List available tools → `[{name, description}]` |
| `client.call_tool(name, args?)` | Call a tool with arguments → Result |
| `client.list_resources()` | List available resources |
| `client.read_resource(uri)` | Read a resource by URI |
| `client.close()` | Disconnect |

---

## Combining MCP with LLM agents

Use MCP tools as part of an LLM agent workflow:

```weft
fn main -> Result {
    // connect to an MCP server for external data
    client := mcp.connect("my-data-server", [])?
    tools := client.list_tools()?

    // build Weft tool wrappers for the MCP tools
    mut weft_tools := []
    for t in tools {
        name := t.name
        push(weft_tools, llm.tool(name, fn(args) -> Result {
            client.call_tool(name, args)
        }, t.description))
    }

    // use them with llm.ask
    reply := llm.ask("Find all users in the NYC office", weft_tools)?
    say(reply)

    client.close()
}
```

---

## Practical examples

### DevOps tools server

```weft
fn check_ports(args) -> Result {
    mut results := []
    for port in args.ports {
        ping := netutil.tcp_ping(args.host, port)?
        push(results, {"port": port, "open": ping.open, "latency_ms": ping.latency_ms})
    }
    Ok(results)
}

fn list_processes(args) -> Result {
    if args.name != null {
        proc.find(args.name)
    } else {
        proc.list()
    }
}

fn disk_usage(args) -> Result {
    path := if args.path != null { args.path } else { "/" }
    sysinfo.disk(path)
}

fn main {
    mcp.serve_stdio([
        mcp.tool("check_ports", "Check if ports are open on a host", check_ports),
        mcp.tool("list_processes", "List or search running processes", list_processes),
        mcp.tool("disk_usage", "Check disk usage for a path", disk_usage),
    ])
}
```

### Database query server

```weft
fn query(args) -> Result {
    db := db.open(env.require("DATABASE_URL")?)?
    db.query(args.sql)?
}

fn tables(args) -> Result {
    db := db.open(env.require("DATABASE_URL")?)?
    db.query("SELECT name FROM sqlite_master WHERE type='table'")?
}

fn main {
    mcp.serve_stdio([
        mcp.tool("query", "Run a read-only SQL query", query),
        mcp.tool("tables", "List all database tables", tables),
    ])
}
```

### Telecom tools server

```weft
use telecom

fn check_extensions(args) -> Result {
    ari := telecom.ari_connect(null)
    telecom.ari_endpoints(ari)
}

fn originate_call(args) -> Result {
    ari := telecom.ari_connect(null)
    telecom.ari_originate(ari, args.endpoint, {"caller_id": args.caller_id})
}

fn main {
    mcp.serve_stdio([
        mcp.tool("check_extensions", "List Asterisk endpoints", check_extensions),
        mcp.tool("originate_call", "Place an outbound call", originate_call),
    ])
}
```

---

## Protocol details

The `mcp` package implements [Model Context Protocol](https://modelcontextprotocol.io/) version `2024-11-05`:

- **Transport**: stdio (newline-delimited JSON-RPC 2.0) or HTTP+SSE
- **Server**: handles `initialize`, `tools/list`, `tools/call`
- **Client**: sends `initialize`, calls `tools/list`, `tools/call`, `resources/list`, `resources/read`
