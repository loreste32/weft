# schema — structured models for AI work

Pure Weft module: define models, validate JSON, emit **JSON Schema** / **tool parameters** for agents.

```bash
weft get schema ./packages/schema
weft install
```

```weft
use schema

Person := schema.model({
    "name": "str!",                          // required string
    "age": schema.int({"min": 0, "max": 150})?,
    "email": "str?",                         // optional
    "tags": ["str"],
    "active": schema.bool({"default": true})?,
})?

fn main -> Result {
    // parse LLM / API JSON
    p := schema.parse(Person, "{\"name\":\"Ada\",\"age\":36,\"tags\":[\"math\"]}")?
    say(p["name"], p["age"])

    // JSON Schema (structured outputs)
    say(json.stringify(schema.json_schema(Person, "Person")))

    // OpenAI-style tool parameters
    say(json.stringify(schema.tool_params(Person)))
}
```

## API

| Call | Role |
|------|------|
| `schema.model(fields)` | build a model |
| `schema.str/int/float/bool/list/object/any(opts?)` | field builders |
| `schema.parse(model, json\|map)` | `Result` cleaned map |
| `schema.validate(model, map)` | same without JSON parse |
| `schema.extract(model, text)` | strip \`\`\` fences + parse (LLM output) |
| `schema.json_schema(model, title?)` | JSON Schema object |
| `schema.tool_params(model)` | `{type,properties,required}` for tools |
| `schema.errors(result)` | list of `{path,msg}` |

### Shorthand field types

| Spec | Meaning |
|------|---------|
| `"str"` / `"str!"` | string (required) |
| `"str?"` | optional string |
| `"int"` / `"float"` / `"bool"` | scalars |
| `["str"]` | list of strings |
| `schema.int({"min":0,"desc":"…"})` | long form |

## With `llm`

```weft
use schema

Args := schema.model({
    "city": schema.str({"desc": "city name", "required": true})?,
})?

fn weather(city) { "clear in $city" }

fn main -> Result {
    // validate tool args the model returned as JSON
    raw := "{\"city\":\"Paris\"}"
    a := schema.parse(Args, raw)?
    say(weather(a["city"]))

    // describe tools for prompts / providers
    params := schema.tool_params(Args)
    say(json.stringify(params))
}
```

No host capabilities required (pure logic + `json`).

## Tests

```bash
weft mod check packages/schema --tests
```
