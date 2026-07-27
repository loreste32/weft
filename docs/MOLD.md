# mold — structured models for agents

**Optional module** (not stdlib). Pour LLM/API JSON into clean maps; emit JSON Schema and tool parameters.

| | |
|--|--|
| **Location** | [`packages/mold`](../packages/mold/) |
| **Install** | `weft get mold ./packages/mold` · `weft packages get mold` |
| **Capabilities** | none (pure `.weft` + `json`) |
| **Cookbook** | [`examples/cookbook/14_mold.weft`](../examples/cookbook/14_mold.weft) |

Why a module, not core: structured validation is useful for agents, but not every script needs it. Core stays lean; apps install when they need molds.

## Install

```bash
# monorepo
weft get mold ./packages/mold
# or catalog
weft packages get mold
weft install
```

```weft
use mold
```

## Quick model

```weft
use mold

fn main -> Result {
    Person := mold.model({
        "name": "str!",
        "age": mold.integer({"min": 0, "max": 150})?,
        "email": "str?",
        "tags": ["str"],
        "active": mold.boolean({"default": true})?,
    })?

    p := mold.parse(Person, "{\"name\":\"Ada\",\"age\":36,\"tags\":[\"math\"]}")?
    say(p["name"], p["age"])   // maps: use p["key"] after parse

    // fenced model output
    b := mold.extract(Person, "```json\n{\"name\":\"Bob\",\"age\":20,\"tags\":[]}\n```")?

    // provider wire shapes
    say(json.stringify(mold.json_schema(Person, "Person")))
    say(json.stringify(mold.tool_params(Person)))
}
```

## API

| Call | Role |
|------|------|
| `mold.model(fields)` | build a mold (`Result`) |
| `mold.str` / `int` / `float` / `bool` / `list` / `object` / `any` | field builders (`string` / `integer` / `boolean` aliases) |
| `mold.parse(model, json\|map)` | coerce + validate → `Result` map |
| `mold.validate(model, map)` | same without JSON parse |
| `mold.extract(model, text)` | strip \`\`\` fences + parse (LLM output) |
| `mold.json_schema(model, title?)` | JSON Schema object |
| `mold.tool_params(model)` | `{type, properties, required, …}` for tools |
| `mold.errors(result)` / `mold.ok(result)` | validation error list / ok flag |

### Field shorthands

| Spec | Meaning |
|------|---------|
| `"str"` / `"str!"` | string (required) |
| `"str?"` | optional string |
| `"int"` / `"float"` / `"bool"` | scalars |
| `["str"]` | list of strings |
| `mold.int({"min":0,"desc":"…"})?` | long form (`?` unwraps `Result`) |

## With tools / agents

Validate arguments the model returned (or describe shapes for prompts/providers):

```weft
use mold

Args := mold.model({
    "city": mold.str({"desc": "city name", "required": true})?,
})?

fn weather(city) { "clear in $city" }

fn main -> Result {
    raw := "{\"city\":\"Paris\"}"   // e.g. tool call JSON
    a := mold.parse(Args, raw)?
    say(weather(a["city"]))
    say(json.stringify(mold.tool_params(Args)))
}
```

See also: [LLM_PROVIDERS.md](LLM_PROVIDERS.md) · [COOKBOOK.md](COOKBOOK.md) (structured models) · [modules.md](modules.md).

## Limits (safety)

Hostile or huge model JSON is capped:

| Cap | Value |
|-----|--------|
| Nest depth | 32 |
| List length | 10 000 |

## Tests

```bash
weft mod check packages/mold --tests
```

## Related

| Doc | |
|-----|--|
| Package README | [`packages/mold/README.md`](../packages/mold/README.md) |
| Optional packages | [packages.md](packages.md) · [`packages/README.md`](../packages/README.md) |
| ML / embeddings module | [ML.md](ML.md) |
| Security (vendor trust) | [../SECURITY.md](../SECURITY.md) |
