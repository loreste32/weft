# Todo app

A complete web app in one Weft file: SQLite + JSON API + HTML UI.

## Run

```bash
weft run examples/todoapp/main.weft
# or with options:
weft run examples/todoapp/main.weft -- --port 3000 --db my.db
# or with watch mode:
weft run examples/todoapp/main.weft --watch
```

Open http://localhost:8080 in your browser.

## API

```bash
# list
curl localhost:8080/api/todos

# add
curl -X POST localhost:8080/api/todos -H 'Content-Type: application/json' -d '{"title":"Buy milk"}'

# toggle done
curl -X PUT localhost:8080/api/todos/1/toggle

# delete
curl -X DELETE localhost:8080/api/todos/1
```

## What it shows

- `db.open` + SQL with parameterized queries
- `web.app` with routes, JSON responses, HTML
- `cli.parse` for flags
- `Result` + `?` error handling throughout
- JSON auto-parse from SQLite TEXT columns
