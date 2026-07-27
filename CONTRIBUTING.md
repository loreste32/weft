# Contributing to Weft

Thanks for helping. Weft targets agent scripts, HTTP glue, and ops tooling — keep changes small and honest about limits.

## Develop

```bash
go build -o weft ./cmd/weft
make test          # go test ./...
bash scripts/ci.sh # full gate (what GitHub Actions runs)
```

Install: `make install` → `~/.local/bin/weft`.

## Before you open a PR

1. `bash scripts/ci.sh` passes  
2. Version string only bumps when intentional (see [docs/VERSIONING.md](docs/VERSIONING.md))  
3. Gold/train corpus stays honest: `weft train eval --quiet` should stay at 100% unless you meant to change it  
4. Prefer stdlib or `packages/*` over core language surface unless most scripts need it  

## Docs map

| Doc | |
|-----|--|
| [docs/README.md](docs/README.md) | Index |
| [docs/ROADMAP.md](docs/ROADMAP.md) | Now / next / never |
| [docs/LANGUAGE.md](docs/LANGUAGE.md) | Language reference |
| [docs/PRINCIPLES.md](docs/PRINCIPLES.md) | Design principles |

## License

By contributing you agree your work is under the project’s [Apache-2.0](LICENSE) license.
