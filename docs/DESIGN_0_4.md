# Weft 0.4.x–0.6.x Design — Type System, Wasm, DAP

Design decisions and implementation plans for the three deep language features.

---

## 1. Type System Evolution

### Current state

- **Gradual typing** via `infer.go` (865 lines) — infers types from usage, reports mismatches as warnings
- **14 type kinds:** any, unit, null, bool, int, float, str, list, map, result, optional, fn, named, channel
- **AST type nodes** exist: `NamedType`, `ListType`, `MapType`, `ResultType`, `OptionalType`, `StructType`
- **`-> Result` on functions** — already parsed and enforced
- **No type annotations on parameters or bindings** — everything is inferred

### Design: optional annotations

Keep the language dynamically typed at runtime. Add **optional** type annotations that the checker validates statically. Untyped code keeps working — annotations are hints, not requirements.

**Syntax (proposed):**

```weft
// annotated parameters
fn add(a: int, b: int) -> int {
    a + b
}

// annotated bindings
name: str := "weft"
count: int := 0

// struct-like type declarations
type User {
    name: str
    email: str
    age: int?        // optional int
    role: str = "user"  // default value
}

// function types
handler: fn(str) -> Result := lookup_user

// generic-ish list/map
items: [int] := [1, 2, 3]
config: Map[str, any] := {"key": "value"}
```

**Key rules:**
1. Annotations are **optional everywhere** — omitting them means `any`
2. The checker **warns**, never errors — existing code keeps working
3. Runtime behavior **never changes** based on annotations — they're erased
4. `-> Result` remains the only annotation that affects runtime (enables `?`)

### Implementation plan

| Step | What | Files | Effort |
|------|------|-------|--------|
| 1 | Add `TypeAnnotation` to AST for params and let bindings | `ast/ast.go` | Small |
| 2 | Parse `: type` after param names and `:=` bindings | `parse/parse.go` | Medium |
| 3 | Use annotations in type inference (constrain instead of infer) | `types/infer.go` | Medium |
| 4 | Parse `type Name { fields }` declarations | `parse/parse.go`, `ast/ast.go` | Medium |
| 5 | Validate struct field access against declared types | `types/infer.go` | Medium |
| 6 | LSP: use annotations for better hover/completion | `lsp/server.go` | Small |

**What NOT to do:**
- No generics (type parameters) — `[any]` is fine
- No type aliases — just `type` declarations
- No interfaces/traits — structural typing via `any` is enough
- No runtime type guards — keep `match` and `.is_err` as-is

### Backwards compatibility

All existing code works unchanged. The parser accepts `fn add(a, b)` and `fn add(a: int, b: int)` — the annotation is optional. The checker treats unannotated parameters as `any`.

---

## 2. Wasm Target

### Historical proposal (superseded)

The original proposal below is retained for design history. The shipped
implementation uses stock Go WASM, not TinyGo or a second JavaScript VM; see
[`wasm/README.md`](../wasm/README.md) for the current contract and tests.

- Weft compiles to **bytecode** (`compile.go` → `opcode.go`) run by a **stack VM** (`vm.go`)
- The VM is a Go `switch` loop over opcodes — not directly compilable to Wasm
- Stdlib is implemented in Go (`internal/stdlib/*.go`) — needs Wasm-compatible reimplementation or host bindings

### Design: two approaches

**Approach A: Compile Go VM to Wasm (TinyGo)**

Compile the entire Weft interpreter to Wasm using TinyGo. The Weft binary runs in the browser, interprets `.weft` code the same way it does natively.

| Pro | Con |
|-----|-----|
| All language features work immediately | Binary is large (~15-30MB Wasm) |
| Stdlib mostly works (Go stdlib compiles to Wasm) | Network/fs/db stdlib won't work in browser |
| No compiler changes needed | Slow startup (Wasm load + init) |

**Approach B: Bytecode interpreter in JS/TS**

Rewrite the VM in TypeScript. The compiler stays in Go (server-side), generates bytecode that a browser-side TS runtime executes.

| Pro | Con |
|-----|-----|
| Small download (~100KB JS) | Must maintain two VM implementations |
| Fast startup | Behavior divergence risk |
| Browser-native APIs available | Huge effort to replicate all stdlib |

**Historical recommendation: Approach A (TinyGo)**

Reason: Weft's VM is ~700 lines of Go. The bytecode, compiler, and type checker are pure Go with no CGo dependencies (SQLite is the only CGo dep). TinyGo can compile it all. We stub out network/db stdlib for the browser build and expose a `say()` → console.log bridge.

### Implementation plan

| Step | What | Effort |
|------|------|--------|
| 1 | Build tag `//go:build !wasm` for network/db stdlib | Small |
| 2 | Create `cmd/weft-wasm/main.go` — Wasm entry point (receives source, returns output) | Small |
| 3 | TinyGo compile: `tinygo build -o weft.wasm -target wasm ./cmd/weft-wasm` | Try it |
| 4 | JS loader: `weft.js` that loads the Wasm, provides `runWeft(code) -> output` | Small |
| 5 | Update playground to use Wasm instead of server API | Small |
| 6 | Stub network/db stdlib to return errors in Wasm mode | Medium |

**Status:** this plan was superseded. The browser target is now built with
`GOOS=js GOARCH=wasm`, has browser Fetch and virtual-fs implementations, and
returns explicit errors for host-only packages.

---

## 3. DAP (Debug Adapter Protocol)

### Current state

- `weft debug` exists (173 lines) — breakpoint-based debugger, but uses a custom CLI protocol
- No IDE integration — VS Code can't connect to it
- The debugger can set breakpoints, step, inspect variables

### Design: DAP over stdio

DAP is JSON-RPC over stdio (like LSP). VS Code, JetBrains, and Neovim all speak it. We wrap the existing debugger with a DAP protocol layer.

**DAP flow:**

```
IDE                       weft debug --dap
 │                            │
 │── initialize ──────────────│
 │── launch (program) ────────│
 │── setBreakpoints ──────────│
 │── configurationDone ───────│
 │                            │── stopped (breakpoint)
 │── threads ─────────────────│
 │── stackTrace ──────────────│
 │── scopes ──────────────────│
 │── variables ───────────────│
 │── continue ────────────────│
 │                            │── terminated
 │── disconnect ──────────────│
```

### Implementation plan

| Step | What | Files | Effort |
|------|------|-------|--------|
| 1 | Create `internal/dap/dap.go` — DAP JSON-RPC protocol handler | New | Medium |
| 2 | Map DAP requests to existing debugger API | `dap.go` → `debug.go` | Medium |
| 3 | Add `--dap` flag to `weft debug` | `cmd/weft/main.go` | Small |
| 4 | Implement: initialize, launch, setBreakpoints, continue, next, stepIn, stepOut | `dap.go` | Medium |
| 5 | Implement: threads, stackTrace, scopes, variables (inspect VM state) | `dap.go` | Medium |
| 6 | VS Code launch.json config for Weft DAP | `editors/vscode/` | Small |
| 7 | Test with VS Code | Manual | Small |

**DAP events to support:**
- `stopped` (breakpoint, step, exception)
- `terminated` (script finished)
- `output` (say/println → debug console)

**Minimum viable DAP:**
- Set breakpoints by line
- Continue, step over, step in, step out
- Inspect local variables at breakpoint
- See call stack

### VS Code integration

```json
// .vscode/launch.json
{
  "type": "weft",
  "request": "launch",
  "name": "Debug Weft",
  "program": "${file}",
  "weftPath": "weft"
}
```

The VS Code extension registers a debug adapter that spawns `weft debug --dap <file>`.

---

## Priority order

1. **Type annotations** — highest impact for developer experience, catches bugs before runtime
2. **DAP** — IDE debugging unlocks serious development workflows
3. **Wasm** — shipped; maintain browser capability parity and integration coverage

## File map

```text
internal/types/type.go     — Type kinds (add struct fields, annotation support)
internal/types/infer.go    — Type inference (use annotations as constraints)
internal/ast/ast.go        — AST nodes (add TypeAnnotation, StructDecl)
internal/parse/parse.go    — Parser (parse : type after params/bindings)
internal/compile/compile.go — Compiler (pass through annotations)
internal/dap/dap.go        — NEW: DAP protocol handler
internal/lsp/server.go     — LSP (use annotations for hover/completion)
pkg/weft/debug.go          — Debugger (expose API for DAP)
cmd/weft-wasm/main.go      — NEW: Wasm entry point
editors/vscode/            — VS Code DAP launch config
```
