# Databases & messaging

Weft talks to the systems you already run in production: **SQL**, **Redis**, **MongoDB**, **NATS**, **RabbitMQ (AMQP)**, and **GraphQL** — all from the language stdlib (Go clients in the binary).

| Package | Systems |
|---------|---------|
| `db` | SQLite (embedded), PostgreSQL, MySQL |
| `redis` | Redis / Valkey / compatible |
| `mongo` | MongoDB |
| `nats` | NATS core (pub/sub, request) |
| `amqp` | RabbitMQ / AMQP 0-9-1 |
| `graphql` | Any GraphQL HTTP endpoint |

## SQL (`db`)

```weft
// SQLite file (pure Go driver — great for local/dev/CI)
c := db.open("sqlite:./app.db")?

// PostgreSQL
c := db.open("postgres://user:pass@localhost:5432/app")?

// MySQL
c := db.open("mysql://user:pass@localhost:3306/app")?

c.exec("CREATE TABLE IF NOT EXISTS users(id INTEGER PRIMARY KEY, name TEXT)")?
c.exec("INSERT INTO users(name) VALUES (?)", ["Ada"])?

rows := c.query("SELECT id, name FROM users WHERE name = ?", ["Ada"])?
for row in rows {
    say(row.id, row.name)
}

one := c.query_one("SELECT name FROM users LIMIT 1")?
c.close()?
```

| Call | |
|------|--|
| `db.open(dsn)` | connect + ping |
| `db.drivers()` | supported schemes |
| `conn.query(sql, [params])` | list of row maps |
| `conn.query_one(sql, [params])` | one map or null |
| `conn.exec(sql, [params])` | `{rows_affected, last_id}` |
| `conn.ping()` / `conn.close()` | |

DSN schemes: `sqlite:`, `postgres://`, `postgresql://`, `mysql://`, or bare path → SQLite.

```bash
weft run examples/db_sqlite.weft
```

## Redis

```weft
r := redis.connect("redis://127.0.0.1:6379/0")?
// or: redis.connect(env.get("REDIS_URL"))?

r.set("session:1", json.stringify(user), 3600)?  // TTL seconds
val := r.get("session:1")?                        // str or null
r.hset("user:1", "name", "Ada")?
r.hgetall("user:1")?
r.lpush("queue", job)?
r.rpop("queue")?
r.publish("events", payload)?
r.incr("hits")?
r.close()?
```

## MongoDB

```weft
m := mongo.connect("mongodb://localhost:27017")?
col := m.collection("app", "users")

col.insert({"name": "Ada", "active": true})?
col.insert_many([{"name": "Bob"}, {"name": "Cy"}])?

docs := col.find({"active": true}, {"limit": 50})?
one := col.find_one({"name": "Ada"})?
col.update({"name": "Ada"}, {"active": false})?      // $set wrapped
col.delete({"name": "Bob"})?
n := col.count({})?
m.close()?
```

## NATS

```weft
nc := nats.connect("nats://127.0.0.1:4222")?

nc.publish("jobs.create", json.stringify(job))?

// request/reply
reply := nc.request("svc.echo", "ping", 2)?   // timeout seconds

// async subscribe (handler runs on each message)
sub := nc.subscribe("jobs.>", fn(msg) {
    say(msg.subject, msg.data)
    if msg.reply != "" {
        msg.respond("ok")?
    }
})?

nc.flush(2)?
// sub.unsubscribe()?
nc.close()?
```

Queue groups:

```weft
nc.queue_subscribe("work", "workers", fn(msg) { ... })?
```

## RabbitMQ (AMQP)

```weft
ch := amqp.connect("amqp://guest:guest@127.0.0.1:5672/")?

ch.queue_declare("jobs", {"durable": true})?
ch.exchange_declare("events", "topic", {"durable": true})?
ch.bind("jobs", "events", "job.*")?

ch.publish("events", "job.created", json.stringify(payload), {
    "content_type": "application/json",
})?

// consumer (background goroutines)
ch.consume("jobs", fn(msg) {
    say(msg.data)
    msg.ack()?
    // or: msg.nack(true)?  // requeue
}, {"auto_ack": false})?

// keep process alive as needed (e.g. web server or sleep loop)
ch.close()?
```

## GraphQL

```weft
res := graphql.query(
    "https://api.example.com/graphql",
    `query User($id: ID!) { user(id: $id) { name email } }`,
    {"id": "42"},
    {"token": env.get("API_TOKEN")},   // Authorization: Bearer …
)?

say(res.data.user.name)
// res.errors if partial GraphQL errors with data
```

Aliases: `graphql.mutation`, `graphql.request` (same HTTP POST shape).

## Env-based URLs (12-factor)

```weft
c := db.open(env.require("DATABASE_URL")?)?
r := redis.connect(env.require("REDIS_URL")?)?
nc := nats.connect(env.get("NATS_URL"))?
```

```bash
export DATABASE_URL=postgres://...
export REDIS_URL=redis://...
weft run worker.weft
```

## With CLI tools

```weft
fn main -> Result {
    p := cli.parse({"flags": {"dsn": {"default": "sqlite:./app.db"}}})?
    c := db.open(p.dsn)?
    // ...
}
```

## Examples

```bash
weft run examples/db_sqlite.weft
weft run examples/data_stack.weft -- all-urls
weft run examples/data_stack.weft -- sqlite
# with services up:
weft run examples/data_stack.weft -- redis
```

## Design notes

| | |
|--|--|
| Drivers | Pure Go SQLite (`modernc.org/sqlite`); official Go clients for the rest |
| Params | Prefer `?` placeholders with param lists (driver-specific under the hood) |
| Errors | All I/O returns `Result` — use `?` |
| Subscribe | NATS/AMQP handlers run concurrently; pair with `web` or a wait loop for workers |
| Not v1 | Connection pools tuning UI, ORM, migrations DSL, gRPC codegen |

For heavy ORMs or schema migrations, keep using your existing tools; Weft is optimized for **scripts, agents, and workers** that open a connection and get work done.
