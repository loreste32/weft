# Sysops with Weft

Weft is for **ops scripts**: runbooks, host checks, small CLIs, workers, and HTTP glue — one binary, packages in `vendor/`.

Language: [LANGUAGE.md](LANGUAGE.md) · CLI flags: [cli.md](cli.md) · Stdlib map: [STDLIB.md](STDLIB.md) · A/B coverage: [STDLIB_GAPS.md](STDLIB_GAPS.md).

## Packages you will use

| Task | API |
|------|-----|
| Flags + subcommands | `cli.parse` (`flags`, `commands` → `p.command`), `cli.prompt`, exit helpers |
| Shell safely | `shlex.split` / `quote` / `join` |
| Run commands | `sh.run` / `capture` / `ok` / `shell` / `which` / `lines` / `code` |
| Process opts | `dir`, `env`, `stdin`, `timeout` (sec or `"5s"`), `merge`, `check` |
| Shutdown | `signal.listen` / `received` / `reset` |
| Files | `fs.*` including `stem`, `with_suffix`, `parents`, `read_bytes` / `write_bytes` |
| Secrets | `secrets.require` / `token_hex` / `token_urlsafe` / `compare` |
| Logs | `log.info`… + `log.set_json` |
| HTTP | `http.get/post/…` opts: `timeout`, `retries`, `headers`, `insecure` |
| Config | `json`, `yaml`, `toml`, `ini` (`sections`, `has_section`) |
| Binary / diff / stats | `binstruct`, `difflib`, `math.quantile` / `mode` |
| Scaffold | `weft new cli <name>` |

```bash
go build -o weft ./cmd/weft   # or: make install
./weft doctor
./weft run examples/sysops_host.weft -- info
./weft run examples/tier_ab.weft -- demo
./weft new cli myctl
```

## Host check

```bash
weft run examples/sysops_host.weft -- info
weft run examples/sysops_host.weft -- tools
weft run examples/sysops_host.weft -- check -r git,sh
weft run examples/sysops_host.weft -- shell 'uname -s'
```

## Runbook sketch

```weft
fn main -> Result {
    p := cli.parse({
        "about": "deployctl",
        "flags": {
            "env": {"short": "e", "default": "dev"},
            "dry_run": {"short": "n", "bool": true},
        },
        "commands": {
            "ship": {"help": "deploy"},
            "status": {"help": "health"},
        },
    })?
    if p.help || p.command == "" {
        say(p.usage)
        cli.exit(0)
    }
    signal.listen()
    log.set_level("info")

    if sh.which("git") == null {
        return Err("git required", "deploy")
    }
    status := sh.capture("git", ["status", "-sb"])?
    log.info("git", {"s": str.trim(status)})

    if p.command == "status" {
        say(status)
        return Ok(unit)
    }
    if p.dry_run {
        say("dry-run ship to", p.env)
        return Ok(unit)
    }
    r := sh.run("make", ["deploy"], {
        "env": {"DEPLOY_ENV": p.env},
        "timeout": "10m",
    })?
    if !r.ok {
        return Err("deploy failed: " + r.stderr, "deploy")
    }
    Ok(unit)
}
```

## Notes

- Prefer `sh.run("cmd", [args…])` over `sh.shell` when you can.
- `http` `insecure: true` is for local TLS only.
- Full A/B inventory and tests: [STDLIB_GAPS.md](STDLIB_GAPS.md).
