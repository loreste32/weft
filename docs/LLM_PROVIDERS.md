# LLM providers

Weft talks HTTP only — no vendor SDKs. One env switch, same `llm.*` / `weft gen` / `weft train chat`.

**Stack map** (`llm` + optional `mold` / `tokensave` / `ml`): [ECOSYSTEM.md](ECOSYSTEM.md).

| Provider | Flag | Key / host | Protocol |
|----------|------|------------|----------|
| **OpenAI** | `WEFT_PROVIDER=openai` (default) | `OPENAI_API_KEY` · `OPENAI_BASE_URL` | OpenAI chat/completions |
| **Ollama** | `WEFT_PROVIDER=ollama` | `OLLAMA_HOST` | OpenAI-compat on local host |
| **vLLM** | `WEFT_PROVIDER=vllm` | `VLLM_BASE_URL` | OpenAI-compat |
| **Anthropic** | `WEFT_PROVIDER=anthropic` | `ANTHROPIC_API_KEY` | Anthropic **Messages** API |

Infer when unset: `ANTHROPIC_API_KEY` alone → anthropic; `OLLAMA_HOST` → ollama; etc.

## Anthropic

```bash
export WEFT_PROVIDER=anthropic
export ANTHROPIC_API_KEY=sk-ant-…
# optional:
# export ANTHROPIC_MODEL=claude-sonnet-4-20250514
# export ANTHROPIC_BASE_URL=https://api.anthropic.com

weft gen "print hello weft" -o hi.weft
weft train chat "Write a Weft hello"
```

```weft
fn main -> Result {
    // uses WEFT_PROVIDER / keys from env
    say(llm.chat("one word: weft")?)
    say(llm.chat("hi", {"system": "Be terse."})?)
    say(llm.chat([
        {"role": "user", "content": "ping"},
    ])?)
}
```

**Agents / tools:** `llm.ask` and `llm.agent` work on Anthropic via **tool_use** / **tool_result** (mapped from the same `llm.tool` bindings). OpenAI-compat hosts still use `tools` + `tool_calls`.

```weft
fn add(a, b) { a + b }

fn main -> Result {
    say(llm.ask("2+3?", [llm.tool("add", add)], {
        "system": "Use tools for math.",
        "max_steps": 6,
    })?)
    say(llm.stream_text("one word: weft")?)
}
```

| Call | Role |
|------|------|
| `llm.chat` | One-shot or multi-turn messages |
| `llm.ask` | Tool-using agent (`tools?`, `opts?`) |
| `llm.agent([tools]).run(prompt)` | Same loop, reusable agent |
| `llm.stream` / `llm.stream_text` | SSE tokens / collected string |
| `llm.extract` | JSON object from the model |
| `llm.tool` | Bind a Weft `fn` as a tool |

**Optional modules** (not in the binary — install only what you need):

| Module | When | Doc |
|--------|------|-----|
| [mold](MOLD.md) | Validate tool/model JSON; emit tool params / JSON Schema | `weft registry install mold` |
| [tokensave](../packages/tokensave/) | Thrift context, memory, teach → train | `weft registry install tokensave` |
| [ml](ML.md) | Embeddings / RAG vectors | `weft registry install ml` |

See [ECOSYSTEM.md](ECOSYSTEM.md#agent-path-cohesive-recipe) for the full agent path. Cookbook: `examples/cookbook/13_agent.weft`, `14_mold.weft`.

### Env keys and custom `base_url`

Process env API keys (`OPENAI_API_KEY`, …) are only sent to **trusted hostnames** (OpenAI/Anthropic, localhost/loopback, plus `WEFT_LLM_TRUST_HOSTS`). Path/substring spoofs are rejected. Prefer `secrets` + explicit keys for non-standard hosts. See [SECURITY.md](../SECURITY.md).

## OpenAI-compat (any host)

```bash
export OPENAI_BASE_URL=https://my-proxy/v1
export OPENAI_API_KEY=…
export WEFT_MODEL=…
```

Local detail: [`docs/LLM_LOCAL.md`](LLM_LOCAL.md).
