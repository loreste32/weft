# Weft for JetBrains

Uses the same **`weft lsp`** stdio server as VS Code. No custom marketplace plugin JAR required — wire via **LSP4IJ**.

## Requirements

1. IntelliJ IDEA / GoLand / WebStorm / PyCharm (recent)
2. Plugin: **[LSP4IJ](https://plugins.jetbrains.com/plugin/23257-lsp4ij)**
3. `weft` on `PATH` (`go build -o weft ./cmd/weft`)

## Setup (LSP4IJ)

1. **Settings → Languages & Frameworks → Language Servers** (LSP4IJ)
2. **+** add server
3. Fill from this table (or import `lsp4ij-weft.json`):

| Field | Value |
|-------|--------|
| Name | Weft |
| Command | `weft` |
| Arguments | `lsp` |
| Language mappings | `weft` → `*.weft`, `*.loom` |

4. Apply → open a `.weft` file → diagnostics/completion should work.

### Import mapping file

Copy [`lsp4ij-weft.json`](lsp4ij-weft.json) into your project as `.lsp4ij/weft.json` if your LSP4IJ build supports project-level servers, **or** paste the command/args manually.

## File type association

If `.weft` is not recognized:

1. **Settings → Editor → File Types**
2. Add type **Weft** with patterns `*.weft`, `*.loom`
3. Optionally set language id / TextMate bundle

A TextMate grammar lives at [`../vscode/syntaxes/weft.tmLanguage.json`](../vscode/syntaxes/weft.tmLanguage.json) — point a TextMate bundle at it for highlighting without LSP.

## Features (from `weft lsp`)

Diagnostics · completion · hover · definition · document symbols · signature help

## Commands outside the IDE

```bash
weft check path/to/file.weft
weft run path/to/file.weft
weft packages list
```

## VS Code

Ready extension: [`../vscode/`](../vscode/) (`npm install` · `npx @vscode/vsce package`).
