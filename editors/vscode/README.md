# Weft for VS Code

Official language support for **[Weft](https://weftproject.dev)** — agent scripts, telecom, HTTP glue, and ops tooling.

| Feature | Details |
|---------|---------|
| **Syntax** | TextMate grammar (`.weft` / `.loom`) |
| **Snippets** | `main`, `fn`, `use`, `agent`, … |
| **LSP** | Diagnostics, completion, hover (types), go-to definition, symbols, format, rename/refs |
| **Types** | Optional annotations surface in hover/completion; type warnings as diagnostics |
| **Debug** | DAP via `weft debug --dap` (breakpoints, step, locals, stack) |

## Requirements

- VS Code **1.85+**
- **`weft` on `PATH`** (or set `weft.lspPath` / launch `weftPath`)

```bash
# install Weft: https://weftproject.dev
weft version
```

## Install

### Marketplace

Search **“Weft”** in Extensions, or:

```bash
code --install-extension weft.weft
```

### From VSIX (dev / offline)

```bash
# from Weft repo
./scripts/package-vscode.sh
code --install-extension editors/vscode/weft-*.vsix --force
```

## Debug

Create `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "type": "weft",
      "request": "launch",
      "name": "Debug Weft",
      "program": "${file}",
      "weftPath": "weft",
      "stopOnEntry": false
    }
  ]
}
```

## Settings

| Setting | Default | Meaning |
|---------|---------|---------|
| `weft.lspPath` | `weft` | Binary for LSP (and default for DAP) |
| `weft.lspArgs` | `["lsp"]` | Usually leave as-is |
| `weft.trace.server` | `off` | LSP wire log |

Commands: **Weft: Restart Language Server**, **Weft: Show Language Server Output**.

## Publish (maintainers)

```bash
# package (also: bash scripts/package-vscode.sh)
cd editors/vscode && npm install && npx @vscode/vsce package

# publish — Azure DevOps PAT with Marketplace (Acquire) scope:
export VSCE_PAT=...   # create at https://dev.azure.com → User settings → Personal access tokens
npx @vscode/vsce publish
```

Publisher id: `weft` · extension id: `weft.weft` · current VSIX: `weft-0.4.2.vsix`.

## JetBrains

See [`../jetbrains/`](../jetbrains/).
