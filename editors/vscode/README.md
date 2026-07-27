# Weft for VS Code

**Syntax** + **Language Server** (`weft lsp`) for the Weft language.

| Feature | Source |
|---------|--------|
| Highlighting | TextMate grammar (`fn`, `:=`, `$name`, `?`, …) |
| Snippets | `main`, `fn`, `use`, `ask`, `brain`, … |
| Diagnostics | `weft lsp` |
| Completion / hover / go-to / symbols / signature | `weft lsp` |

## Requirements

- [VS Code](https://code.visualstudio.com/) 1.85+
- **`weft` on `PATH`** (or set `weft.lspPath`)

```bash
# from the Weft repo
go build -o weft ./cmd/weft
export PATH="$PWD:$PATH"
weft version
```

## Install (development)

```bash
cd editors/vscode
npm install
# F5 in VS Code with this folder open, or:
code --install-extension . 
# better: package then install VSIX
npm run package
code --install-extension weft-*.vsix --force
```

## Settings

| Setting | Default | Meaning |
|---------|---------|---------|
| `weft.lspPath` | `weft` | Binary path |
| `weft.lspArgs` | `["lsp"]` | Args (usually leave as-is) |
| `weft.trace.server` | `off` | LSP wire log |

Commands: **Weft: Restart Language Server**, **Weft: Show Language Server Output**.

## Package for distribution

```bash
cd editors/vscode
npm install
npx @vscode/vsce package
# → weft-0.3.1.vsix
```

Marketplace publish (when ready): `npx @vscode/vsce publish` (needs publisher token).

## JetBrains

See [`../jetbrains/`](../jetbrains/).
