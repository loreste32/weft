# Expanding Weft with modules

Third parties grow Weft’s surface with **pure `.weft` packages** — not plugins, not Python wheels.

```text
examples/modules/
  packages/
    mathx/       # multi-file library (lib.weft + helpers.weft)
    resultx/     # depends on mathx (transitive install)
  demo/          # app: weft install → vendor/ → use mathx / resultx
```

## Run the demo

```bash
cd examples/modules/demo
weft install          # vendors resultx + transitive mathx
weft run main.weft
weft mod check ../packages/mathx
weft mod check ../packages/resultx
```

## Author your own

```bash
weft new module coolkit
# edit lib.weft — mark API with `pub fn`
weft mod check coolkit
# consumers:
weft get coolkit ./coolkit   # or github.com/you/coolkit@v0.1.0
weft install
```

```weft
use coolkit
fn main { say(coolkit.hello("weft")) }
```

See [docs/modules.md](../../docs/modules.md).
