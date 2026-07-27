# Agent stack demo (offline)

Cohesive sample for the three optional modules + stdlib mental model:

```text
tokensave  →  thrift / clarify knowledge
mold       →  structure tool JSON + tool_spec
ml         →  local vector topk (RAG-scale)
```

No network or API keys. See [docs/ECOSYSTEM.md](../../docs/ECOSYSTEM.md).

```bash
cd examples/agent_stack
weft install    # if vendor/ missing
weft run main.weft
```
