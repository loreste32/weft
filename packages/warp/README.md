# warp

N-dimensional array math for Weft. Pure `.weft` — no native deps, no Go, no C.

The warp threads of a loom: structured, tensioned, ready for work.

## Install

```bash
weft registry install warp
# or from git:
# weft registry install warp
```

## Quick start

```weft
use warp

fn main {
    a := warp.array([1, 2, 3, 4, 5, 6], [2, 3])
    b := warp.ones([2, 3])
    c := warp.add(a, b)
    warp.print_(c)
    // [2, 3, 4]
    // [5, 6, 7]

    say("sum:", warp.sum(c))
    say("mean:", warp.mean(c))

    // matrix multiply
    x := warp.array([1, 2, 3, 4], [2, 2])
    y := warp.eye(2)
    say(warp.to_list(warp.matmul(x, y)))
}
```

## API

### Constructors

| Function | Description |
|----------|-------------|
| `array(data, shape?)` | Create from flat list + shape |
| `zeros(shape)` | All zeros |
| `ones(shape)` | All ones |
| `full(shape, value)` | Fill with value |
| `arange(start, stop?, step?)` | Range like `0, 1, 2, ...` |
| `linspace(start, stop, n)` | n evenly spaced values |
| `eye(n)` | n×n identity matrix |
| `diag(array)` | Diagonal matrix from 1d array |

### Properties

| Function | Returns |
|----------|---------|
| `shape(a)` | Dimension list `[rows, cols, ...]` |
| `size(a)` | Total element count |
| `ndim(a)` | Number of dimensions |

### Arithmetic (element-wise, scalar broadcast)

`add`, `sub`, `mul`, `div`, `neg`, `abs_`, `sqrt`, `exp`, `log_`, `pow_`, `round_`, `clip`

```weft
warp.add(a, b)       // element-wise
warp.mul(a, 2)       // scalar broadcast
warp.clip(a, 0, 1)   // clamp values
```

### Reductions

`sum`, `mean`, `var_`, `std_`, `min_`, `max_`, `argmin`, `argmax`

### Linear algebra

| Function | Description |
|----------|-------------|
| `dot(a, b)` | Dot product (1d) |
| `matmul(a, b)` | Matrix multiply (2d × 2d) |
| `T(a)` | Transpose (2d) |

### Comparisons

`equal`, `less`, `greater` — return 0/1 arrays.

`where(cond, x, y)` — element-wise ternary.

### Shape ops

`reshape`, `flatten`, `T`

### Utility

`to_list`, `print_`

## Design

- Arrays are `{_warp: true, data: [...], shape: [...]}` — plain Weft maps
- Flat data + shape metadata (row-major)
- Scalar broadcast on binary ops
- No mutation — all ops return new arrays
- Trailing `_` on names that clash with Weft builtins (`abs_`, `min_`, `var_`)

## Limits

This is a scripting-weight array library, not BLAS. Good for:
- Small data transforms in agent scripts
- Prototyping math before calling real services
- Learning linear algebra interactively

Not good for: large datasets, GPU compute, training neural networks. For those, orchestrate external tools from Weft.
