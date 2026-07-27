# mold — pour data into shape

Pure Weft module for agent work: define **molds**, validate JSON, emit **JSON Schema** / **tool parameters**.

Name: you pour LLM or API JSON into a mold and get clean maps out.

```bash
weft get mold ./packages/mold
weft install
```

```weft
use mold

Person := mold.model({
    "name": "str!",                          // required string
    "age": mold.int({"min": 0, "max": 150})?,
    "email": "str?",                         // optional
    "tags": ["str"],
    "active": mold.bool({"default": true})?,
})?

fn main -> Result {
    // parse LLM / API JSON
    p := mold.parse(Person, "{\"name\":\"Ada\",\"age\":36,\"tags\":[\"math\"]}")?
    say(p["name"], p["age"])

    // JSON Schema (structured outputs)
    say(json.stringify(mold.json_schema(Person, "Person")))

    // OpenAI-style tool parameters
    say(json.stringify(mold.tool_params(Person)))
}
```

## API

| Call | Role |
|------|------|
| `mold.model(fields)` | build a mold |
| `mold.str/int/float/bool/list/object/any(opts?)` | field builders |
| `mold.parse(model, json\|map)` | `Result` cleaned map |
| `mold.validate(model, map)` | same without JSON parse |
| `mold.extract(model, text)` | strip \`\`\` fences + parse (LLM output) |
| `mold.json_schema(model, title?)` | JSON Schema object |
| `mold.tool_params(model)` | `{type,properties,required}` for tools |
| `mold.errors(result)` | list of `{path,msg}` |

### Shorthand field types

| Spec | Meaning |
|------|---------|
| `"str"` / `"str!"` | string (required) |
| `"str?"` | optional string |
| `"int"` / `"float"` / `"bool"` | scalars |
| `["str"]` | list of strings |
| `mold.int({"min":0,"desc":"…"})` | long form |

## With `llm`

```weft
use mold

Args := mold.model({
    "city": mold.str({"desc": "city name", "required": true})?,
})?

fn weather(city) { "clear in $city" }

fn main -> Result {
    raw := "{\"city\":\"Paris\"}"
    a := mold.parse(Args, raw)?
    say(weather(a["city"]))

    params := mold.tool_params(Args)
    say(json.stringify(params))
}
```

No host capabilities required (pure logic + `json`).

## Tests

```bash
weft mod check packages/mold --tests
```
