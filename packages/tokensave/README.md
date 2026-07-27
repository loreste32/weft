# tokensave — model brain with memory (external)

**Optional module** — not core stdlib, not in the `weft` binary.  
Stack map (with `llm` / `mold` / `ml`): [docs/ECOSYSTEM.md](../../docs/ECOSYSTEM.md).

A thrift **brain** for **local and paid** models:

| Step | What happens |
|------|----------------|
| **Clarify** | Fuzzy user ask → Goal / Must / Avoid / Output |
| **Recall** | Pull past successful context for similar asks |
| **Pick** | Keep only relevant knowledge (drop noise) |
| **Pack** | Short system + framed user under a token budget |
| **Remember** | Store what worked so the next call is smarter |
| **Train** | Export memory → `weft train --from` gold JSONL |

```bash
weft get tokensave ./packages/tokensave
weft install
```

```weft
use tokensave

fn main -> Result {
    knowledge := [/* your docs */]
    mem := ".weft/tokensave-memory.json"

    p := tokensave.brain("yo print hi weft", knowledge, {
        "memory_path": mem,
        "k": 3,
    })
    // p.user is clarified + relevant only
    // p.recalled = past wins for similar asks

    // after a good answer — one call updates memory + private gold:
    tokensave.teach(mem, "domain-gold.jsonl", p.ask, "fn main { say(\"hi\") }", "weft")?

    // later: weft train prepare --from domain-gold.jsonl --expand
    //        weft train finetune --private --preset qwen-1.5b
    //        tokensave.export_train(mem, "from-memory.jsonl")?
}
```

## Getting smarter with use

| Call | Role |
|------|------|
| `brain(..., {memory_path})` | auto-recall + auto-remember each pack |
| `teach(mem, gold, ask, answer, kind?)?` | **one call**: memory feedback + gold JSONL |
| `feedback` / `remember` | lower-level memory updates |
| `recall(path, ask, k)?` | past docs + known-good answers |
| `stats(path)?` | `{n, ok, fail}` |
| `export_train(mem, out.jsonl)?` | successful episodes → train rows |
| `append_gold(...)?` | direct private SFT line |

Each **successful** episode stores: clarified goal, kind, docs used, optional answer.  
Next similar ask gets those docs first → better accuracy, fewer tokens, better local fine-tunes.

## Clarify messy asks

```weft
c := tokensave.clarify("fix the json thing", null)
// c.goal, c.kind, c.clear  — no model call, offline
```

## Train loop (local models especially)

```bash
# 1) use the brain day-to-day with memory_path
# 2) export what worked
weft run export.weft   # calls tokensave.export_train / append_gold

# 3) fine-tune privately
weft train prepare -o weft-sft --expand --from domain-gold.jsonl
weft train finetune --private --preset qwen-1.5b
```

Training data is **already clarified + context-relevant** — not raw chat dumps.

## API sketch

`clarify` · `detect_kind` · `pick` · `frame` · `brain` · `ask` ·  
`remember` · `recall` · `feedback` · `stats` ·  
`export_train` · `append_gold` · `estimate` · `pack` · `contract`

Demo: `examples/tokensave_demo/`.
