# Sysops with Weft

Weft is built for **ops scripts**: runbooks, host checks, small CLIs, workers, and HTTP glue — one binary, no venv, packages vendored into `vendor/`.

This page is the ops map. Language details live in [LANGUAGE.md](LANGUAGE.md); CLI flags in [cli.md](cli.md); production defaults in [PRODUCTION.md](PRODUCTION.md).

## What you can do today

| Task | Packages / tools |
|------|------------------|
| Flags, usage, exit codes | `cli` |
| Run external commands | `sh.run`, `sh.capture`, `sh.ok`, `sh.shell`, `sh.which` |
| Files & paths | `fs` (read/write/glob/lines/atomic write/chmod/…) |
| Env & process identity | `env`, `platform` |
| Secrets at the edge | `secrets` (+ redaction in JSON) |
| Structured logs | `log` |
| Config | `json`, `yaml`, `toml`, `ini` |
| HTTP / webhooks | `http`, `web` |
| Data jobs | `csv`, `jsonl`, `table`, `pipe`, `db` |
| Archives | `archive` (tar/zip/gzip) |
| Scaffold a tool | `weft new cli <name>` |

```bash
go build -o weft ./cmd/weft   # or: make install
./weft doctor
./weft run examples/sysops_host.weft -- info
./weft run examples/sysops_host.weft -- tools
./weft run examples/sysops_host.weft -- check -r git,sh
./weft run examples/cli_tool.weft -- --help
./weft new cli myctl
```

## Host check (example)

```bash
weft run examples/sysops_host.weft -- info     # os/arch/user/cwd/cpus
weft run examples/sysops_host.weft -- tools    # git/docker/kubectl presence
weft run examples/sysops_host.weft -- check    # fail if required tools missing
weft run examples/sysops_host.weft -- shell 'df -h | head'
```

`check` exits non-zero when a tool listed in `-r/--require` is missing (default `git,sh`).

## Typical runbook pattern

```weft
fn main -> Result {
    p := cli.parse({
        "about": "deployctl",
        "flags": {
            "env": {"short": "e", "default": "dev"},
            "dry_run": {"short": "n", "bool": true},
        },
    })?
    if p.help { say(p.usage); cli.exit(0) }

    log.set_level("info")
    need := sh.which("git")
    if need == null { return Err("git required", "deploy") }

    status := sh.capture("git", ["status", "-sb"])?
    log.info("git", str.trim(status))

    if p.dry_run {
        say("dry-run: would deploy to " + p.env)
        return Ok(unit)
    }

    r := sh.run("make", ["deploy"], {"env": ["DEPLOY_ENV=" + p.env]})?
    if !r.ok {
        log.error("deploy failed", r.code, r.stderr)
        return Err("deploy failed", "deploy")
    }
    Ok(unit)
}
```

## Shell vs argv

| API | Use when |
|-----|----------|
| `sh.run("git", ["status", "-sb"])` | Fixed command + args (preferred) |
| `sh.capture(...)` | Need stdout string; fails on non-zero |
| `sh.ok(...)` | Boolean success |
| `sh.shell("df -h \| head")` | Pipelines / one-off glue (quote carefully) |

Pass `{"dir": "…", "timeout": "30s", "env": ["K=V"], "stdin": "…"}` as the last opts map when needed.

## Related examples

| Example | Role |
|---------|------|
| [`examples/sysops_host.weft`](../examples/sysops_host.weft) | Host facts + tool check + shell |
| [`examples/cli_tool.weft`](../examples/cli_tool.weft) | Full devops CLI sample |
| [`examples/data_pipeline.weft`](../examples/data_pipeline.weft) | File/CSV/JSONL jobs |
| [`examples/prod_worker.weft`](../examples/prod_worker.weft) | SQLite (+ optional Redis) worker |
| [`examples/cookbook/12_cli.weft`](../examples/cookbook/12_cli.weft) | Minimal CLI recipe |

## Install for daily ops

```bash
make install          # ~/.local/bin/weft
export PATH="$HOME/.local/bin:$PATH"
weft version          # expect 0.3.x current
weft doctor
```

CI gate for the whole tree: `bash scripts/ci.sh` (same as GitHub Actions on `main`).
