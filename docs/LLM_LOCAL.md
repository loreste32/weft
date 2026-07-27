# Ollama & vLLM (local models)

Weft talks to **Ollama** and **vLLM** over their OpenAI-compatible HTTP APIs — same `llm.chat` / `weft gen` / `train chat` paths, no third-party client SDKs.

Cloud Anthropic + multi-provider overview: [`docs/LLM_PROVIDERS.md`](LLM_PROVIDERS.md).

| | Ollama | vLLM |
|--|--------|------|
| Default base | `http://127.0.0.1:11434/v1` | `http://127.0.0.1:8000/v1` |
| Env host | `OLLAMA_HOST` | `VLLM_BASE_URL` |
| Env model | `OLLAMA_MODEL` | `VLLM_MODEL` |
| API key | optional (`ollama`) | optional (`EMPTY`) |
| Provider flag | `WEFT_PROVIDER=ollama` | `WEFT_PROVIDER=vllm` |

## Quick start — Ollama

```bash
# install & run Ollama, then:
ollama pull llama3.2

export WEFT_PROVIDER=ollama
export OLLAMA_HOST=http://127.0.0.1:11434
export OLLAMA_MODEL=llama3.2

weft ollama list
weft ollama chat "Write a one-line hello"
weft gen "print hello weft" -o hello_gen.weft

# in scripts:
weft run examples/ollama_chat.weft
```

```weft
fn main -> Result {
    models := ollama.list()?
    say(models)
    reply := ollama.chat("Explain Result types in one sentence")?
    say(reply)
    // or force model:
    reply2 := ollama.chat({
        "model": "llama3.2",
        "prompt": "hi",
        "system": "You write terse answers.",
    })?
}
```

Native generate (non-chat):

```weft
text := ollama.generate("llama3.2", "Write a haiku about weaving")?
```

Pull (long-running):

```weft
ollama.pull("llama3.2")?
// or: weft ollama pull llama3.2  (shells out to ollama binary)
```

## Quick start — vLLM

```bash
# start an OpenAI-compatible vLLM server, e.g.:
#   vllm serve meta-llama/Meta-Llama-3-8B-Instruct --port 8000

export WEFT_PROVIDER=vllm
export VLLM_BASE_URL=http://127.0.0.1:8000/v1
export VLLM_MODEL=meta-llama/Meta-Llama-3-8B-Instruct
# optional auth:
# export VLLM_API_KEY=sk-local

weft vllm health
weft vllm list
weft vllm chat "Summarize Weft in 10 words"
weft gen "sum 1..5" -o sum.weft
```

```weft
fn main -> Result {
    vllm.health()?
    models := vllm.list()?
    say(models)
    reply := vllm.chat({
        "model": "meta-llama/Meta-Llama-3-8B-Instruct",
        "prompt": "List three uses for a scripting language.",
    })?
    say(reply)
}
```

## Unified `llm.*` surface

With provider env set, **all** of these hit Ollama/vLLM automatically:

```bash
export WEFT_PROVIDER=ollama
export OLLAMA_MODEL=qwen2.5-coder:7b

weft gen "read a JSON file and print keys"
weft train chat "write hello weft"
```

```weft
// uses defaultLLMOpts → ollama/vllm when WEFT_PROVIDER is set
reply := llm.chat("hi")?
agent := llm.agent([llm.tool("echo", echo_fn)])
```

Or pass explicitly:

```weft
llm.chat({
    "base_url": ollama.openai_base(),
    "api_key": "ollama",
    "model": "llama3.2",
    "prompt": "hello",
})
```

## Detection order

1. `WEFT_PROVIDER` / `LLM_PROVIDER` (`openai` | `ollama` | `vllm`)  
2. Else if `OLLAMA_HOST` / `OLLAMA_BASE_URL` → ollama  
3. Else if `VLLM_BASE_URL` / `VLLM_HOST` → vllm  
4. Else OpenAI (requires API key)

`weft doctor` shows provider reachability.

## Env reference

| Variable | Role |
|----------|------|
| `WEFT_PROVIDER` | `ollama` / `vllm` / `openai` |
| `OLLAMA_HOST` | e.g. `http://127.0.0.1:11434` |
| `OLLAMA_MODEL` | default chat model |
| `VLLM_BASE_URL` | e.g. `http://127.0.0.1:8000/v1` |
| `VLLM_MODEL` | served model id |
| `VLLM_API_KEY` | optional bearer token |
| `WEFT_MODEL` | generic model override |
| `OPENAI_BASE_URL` | any OpenAI-compatible host (also works) |

## Tips

| Topic | |
|-------|--|
| Slow local gen | Client timeout is 300s for local providers |
| Tools / agents | Same OpenAI tool-calling path if the model/server supports it |
| Privacy | Data stays on your GPU box — see `docs/FINETUNE.md` private train |
| OpenAI-compat only | Point `OPENAI_BASE_URL` at any server (LM Studio, LocalAI, …) |

## CLI

```text
weft ollama list | chat | ps | pull <model>
weft vllm list | chat | health
weft doctor
```
