# Weft editor packaging

Both IDEs use the **same** language server: `weft lsp` (stdio JSON-RPC).

| IDE | Path | Install |
|-----|------|---------|
| **VS Code** | [`vscode/`](vscode/) | `npm install` · `npx @vscode/vsce package` · install `.vsix` |
| **JetBrains** | [`jetbrains/`](jetbrains/) | LSP4IJ + command `weft` args `lsp` |

## Prerequisite

```bash
go build -o weft ./cmd/weft
export PATH="$PWD:$PATH"   # or install to ~/go/bin
weft lsp                   # should wait on stdin (Ctrl-C to stop)
```

## Quick VS Code VSIX

```bash
./scripts/package-vscode.sh
# → editors/vscode/weft-*.vsix
code --install-extension editors/vscode/weft-*.vsix --force
```

## Language surface

See [`docs/SYNTAX.md`](../docs/SYNTAX.md). Snippets live in the VS Code pack; JetBrains gets them via LSP completion later.
