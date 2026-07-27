# CLI apps for devops & data processing

Weft is a strong fit for **small ops tools** and **pipe-friendly data jobs**: one binary runtime, no environment activation, flags + shell + files + JSON in the stdlib.

## Packages

| Package | Role |
|---------|------|
| `cli` | argv, flags, **subcommands**, usage, `prompt`, exit codes |
| `sh` / `shlex` | run / capture / lines / code; safe split/quote |
| `fs` | read/write/glob/lines/paths/`stem`/`read_bytes`/… |
| `signal` | listen / received / reset (SIGINT/SIGTERM) |
| `io` | stdin lines, stderr log |
| `str` | split/join/trim/fields/lines |
| `json` | parse/stringify pipelines |
| `env` | environment variables |
| `len` `push` `concat` `slice` `range` | list building for pipelines |
| `csv` | parse/stringify/read/write CSV |
| `time` | now, iso, sleep, format |

## Minimal CLI

```weft
fn main -> Result {
    p := cli.parse({
        "about": "mytool — does a thing",
        "flags": {
            "env": {"short": "e", "default": "dev", "help": "target env"},
            "verbose": {"short": "v", "bool": true},
            "file": {"short": "f", "default": "", "help": "input file"},
        },
    })?

    if p.help {
        say(p.usage)
        cli.exit(0)
    }

    if p.verbose {
        io.eprintln("env=" + p.env)
    }

    // positionals: p.args  (list)
    // flags: p.env, p.verbose, p.file
}
```

```bash
weft run mytool.weft -- --help
weft run mytool.weft -- -e prod -v deploy
weft run mytool.weft -- --env=prod path/to/file
```

### `cli.parse` result

| Field | |
|-------|--|
| `help` | true if `-h` / `--help` |
| `usage` | generated help text |
| `command` | matched subcommand name, or `""` |
| `args` | remaining positionals (after command if any) |
| `flags` | map of flag values |
| *name* | each flag also flat on the result (`p.env`) |

### Subcommands (built-in)

```weft
p := cli.parse({
    "about": "ctl",
    "flags": {"env": {"short": "e", "default": "dev"}},
    "commands": {
        "deploy": {"help": "ship it"},
        "status": {"help": "show status"},
    },
})?
if p.help || p.command == "" {
    say(p.usage)
    cli.exit(0)
}
if p.command == "deploy" { /* … */ }
```

Also: `cli.prompt("name: ")?` reads one line from stdin.

### Exit

```weft
cli.exit(0)           // success, silent
cli.die("bad input")  // stderr + exit 1
cli.ok("done")        // print + exit 0
```

`fn main -> Result` that returns `Err(...)` also exits **1**.

## Shell / devops

```weft
// structured
r := sh.run("git", ["status", "-sb"])?
say(r.code, r.ok)
say(r.stdout)

// fail if non-zero
out := sh.capture("uname", ["-s"])?
say(out)

// glue
r := sh.shell("ls examples | wc -l")?

// options
sh.run("make", ["test"], {"dir": ".", "timeout": 120, "env": {"CI": "1"}, "check": true})?

// path lookup
git := sh.which("git")  // str or null
```

| Call | |
|------|--|
| `sh.run(cmd, [args], opts?)` | `{code, ok, stdout, stderr}` |
| `sh.capture(...)` | stdout string, error if non-zero |
| `sh.ok(...)` | bool Result |
| `sh.shell(line)` | `sh -c` |
| `sh.which(bin)` | absolute path or null |

## Filesystem

```weft
text := fs.read("cfg.json")?
fs.write("out.txt", text)?
fs.append("log.txt", "line\n")?
lines := fs.lines("data.jsonl")?
names := fs.list(".")?
files := fs.glob("examples/**/*.weft")?   // Go filepath.Glob patterns
fs.mkdir("out/dir")?
path := fs.join("a", "b", "c.weft")
say(fs.cwd()?, fs.base(path), fs.ext(path))
```

## Pipes & data jobs

```weft
// stdin JSONL → filter → stdout
lines := io.lines()?
for line in lines {
    row := json.parse(str.trim(line))?
    if row.ok {
        say(json.stringify(row))
    }
}
```

```bash
cat data.jsonl | weft run filter.weft -- -f ok
weft run filter.weft -- -i data.jsonl -o good.jsonl -v
```

### String helpers

```weft
str.split("a,b,c", ",")
str.fields("  a   b  c")     // awk-like
str.lines(file_text)
str.trim(s)  str.lower(s)  str.contains(s, "x")
str.join(["a","b"], ",")
str.replace(s, "old", "new")
```

### Lists (pipelines)

```weft
mut xs := []
push(xs, row)
push(xs, other)
ys := concat(xs, more)
part := slice(xs, 0, 10)
for i in range(5) { ... }
```

### CSV

```weft
data := csv.read("metrics.csv", {"header": true})?
// data.header, data.rows (list of maps)
say(csv.stringify(data.rows, {"header": data.header}))
csv.write("out.csv", data.rows, {"header": data.header})?
```

### Time

```weft
say(time.iso())
say(time.format("date"))
t0 := time.now()
// ... work ...
say(time.since(t0))   // seconds
time.sleep(1)
```

## Scaffold a CLI

```bash
weft new cli myctl
weft run myctl/main.weft -- --help
weft run myctl/main.weft -- greet Ada -e prod
```

## Examples

```bash
# flags + subcommands + shell
weft run examples/cli_tool.weft -- --help
weft run examples/cli_tool.weft -- greet Ada -e prod
weft run examples/cli_tool.weft -- lines examples/hello.weft
weft run examples/cli_tool.weft -- shell "uname -s"
weft run examples/tier_ab.weft -- demo

# JSONL filter pipeline
weft run examples/data_pipeline.weft -- \
  -i examples/data/users.jsonl -f ok --field name -v

# CSV → table + chart
weft run examples/csv_report.weft -- -i examples/data/metrics.csv -c /tmp/m.html -v
```

## Patterns

### Config from env + flags

```weft
token := env.get("API_TOKEN")
if token == null {
    cli.die("set API_TOKEN")
}
base := p.url
if base == "" {
    base = "https://api.example.com"
}
```

### Exit codes for CI

| Code | Meaning |
|------|---------|
| 0 | success (`cli.exit(0)` or clean `main`) |
| 1 | `Err` / `cli.die` / failed `?` |
| 2 | usage error (your choice via `cli.exit(2)`) |
| *n* | forward `sh` exit: `cli.exit(r.code)` |

## Scaffold

```bash
weft new app myctl
# edit main.weft — add cli.parse + sh/fs
weft run myctl/main.weft -- --help
```

## vs bash

| | bash | Weft |
|--|------|------|
| JSON | jq glue | `json` built-in |
| Flags | getopts | `cli.parse` |
| Ship | shell + deps | single `weft` binary + `.weft` |
| Types | no | optional + `weft check` |
| Concurrency | `&` | `spawn` / `parallel` |
