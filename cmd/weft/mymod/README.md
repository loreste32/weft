# mymod

Weft **module** — installable library for other Weft apps.

## Use it

```bash
# from a path (local development)
weft get mymod /Users/loreste/weft/cmd/weft/mymod

# from git (after you push + tag)
weft get mymod github.com/you/mymod@v0.1.0
weft install
```

```weft
use mymod

fn main {
    say(mymod.hello("weft"))
    say(mymod.greet("devs"))
}
```

## Author checklist

1. Edit `lib.weft` — mark public API with `pub fn`
2. Multi-file: `use "./util.weft" as util` (only entry exports are the package surface)
3. Depend on other modules: add `deps` in `weft.json` (consumers get them transitively)
4. Set `name` / `version` / `exports` in `weft.json`
5. `weft mod check` — validate (parses all .weft files)
6. `weft mod check --tests` — validate + run tests
7. `weft mod pack` — zip for distribution
8. Tag a release and share the git URL (or add to monorepo packages/index.json)

Modules **expand Weft** for your users: they `use mymod` and call your API — no binary plugins.

See [docs/modules.md](https://github.com/loreste/weft/blob/main/docs/modules.md).
