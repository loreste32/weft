# Contributing to Weft

We welcome contributions. Here's how to get started.

## Quick start

```bash
git clone https://github.com/loreste32/weft.git
cd weft
go build -o weft ./cmd/weft
./weft doctor
make test    # 1246 tests
make ci      # gofmt + vet + tests
```

## What to work on

- **Issues:** Check [github.com/loreste32/weft/issues](https://github.com/loreste32/weft/issues)
- **Good first issues:** Look for the `good-first-issue` label
- **Docs:** Typos, unclear explanations, missing examples
- **Tests:** Improve coverage for stdlib packages
- **Modules:** Build and publish to the registry

## Project structure

```text
cmd/weft/          CLI entry point
internal/
  parse/           Lexer + parser → AST
  compile/         AST → bytecode
  vm/              Stack VM execution
  runtime/         Values, types, env
  stdlib/          All 76 stdlib packages (one .go file each)
  pkgman/          Package manager, registry client/server, signing
  types/           Gradual type checker
  lsp/             Language server
  diag/            Diagnostics
pkg/weft/          Public API (run, test, build, lint, etc.)
packages/          Optional modules (telecom, mold, ml, etc.)
docs/              All documentation (markdown)
examples/          Example scripts
editors/           VS Code + JetBrains plugins
```

## Adding a stdlib package

1. Create `internal/stdlib/mypkg.go`
2. Implement `func packageMyPkg(env *runtime.Env) runtime.Value`
3. Register in `internal/stdlib/pkg.go`
4. If it accesses network/fs/secrets, add to `RestrictedByDefault` in `internal/pkgman/capabilities.go`
5. Add docs in `docs/STDLIB.md`
6. Run `make ci`

## Adding a registry module

1. `weft new module mymod && cd mymod`
2. Write `lib.weft` with `pub fn` exports
3. Add `weft.json` with name, version, exports, capabilities
4. `weft mod check --tests`
5. `weft publish --key <keyname>`

## Running tests

```bash
make test              # all tests
go test ./internal/stdlib/ -run TestMyPkg -v   # specific package
weft test examples/cookbook -q                   # weft-level tests
weft lint .                                     # static analysis
weft check --types .                            # type check
```

## Code style

- `gofmt` on all Go code (CI enforces this)
- `weft fmt` on all `.weft` code
- 4-space indentation in Weft
- No trailing whitespace (`weft lint` catches this)
- Tests for non-trivial logic

## Commit messages

- Start with what changed: `Add`, `Fix`, `Update`, `Remove`
- Keep the first line under 72 characters
- Reference issues: `Fix #123: handle nil in json.get`

## Pull requests

1. Fork and branch from `main`
2. Make your changes
3. Run `make ci` (must pass)
4. Open a PR with a clear description
5. One PR per feature/fix

## Community

- Issues: [github.com/loreste32/weft/issues](https://github.com/loreste32/weft/issues)
- Discussions: [github.com/loreste32/weft/discussions](https://github.com/loreste32/weft/discussions)

## License

Apache-2.0. By contributing, you agree your contributions are licensed under the same terms.
