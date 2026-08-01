# warp — numpy-style array math for Weft

Pure `.weft` module, no native dependencies. Flat storage with shape metadata, element-wise operations, linear algebra, statistics, and sorting.

```bash
weft get warp
```

```weft
use warp

fn main {
    a := warp.array([1, 2, 3, 4, 5, 6], [2, 3])
    warp.print_(a)
    // [1, 2, 3]
    // [4, 5, 6]

    say("mean: " + str(warp.mean(a)))        // 3.5
    say("sum:  " + str(warp.sum(a)))          // 21
    say("std:  " + str(warp.std_(a)))         // 1.707...
}
```

## Creation

| Function | What it does |
|----------|-------------|
| `array(data, shape)` | Create from flat list + shape |
| `from_list([[1,2],[3,4]])` | Create from nested lists (2D) |
| `zeros([rows, cols])` | All zeros |
| `ones([n])` | All ones |
| `full([n], value)` | Fill with a value |
| `eye(n)` | Identity matrix |
| `diag(vec)` | Diagonal matrix from vector |
| `arange(start, stop, step?)` | Range (like Python's) |
| `linspace(start, stop, n)` | Evenly spaced points |
| `rand([rows, cols])` | Uniform random [0, 1) |
| `randn([rows, cols])` | Normal distribution (Box-Muller) |
| `randint(low, high, [n])` | Random integers in [low, high) |

```weft
a := warp.zeros([3, 3])       // 3x3 zeros
b := warp.eye(4)              // 4x4 identity
c := warp.arange(0, 10, 2)    // [0, 2, 4, 6, 8]
d := warp.linspace(0, 1, 5)   // [0, 0.25, 0.5, 0.75, 1]
r := warp.randn([100])        // 100 normal-distributed values
```

## Shape & indexing

| Function | What it does |
|----------|-------------|
| `shape(a)` | Shape as list (e.g., `[2, 3]`) |
| `size(a)` | Total elements |
| `ndim(a)` | Number of dimensions |
| `reshape(a, [new_shape])` | Change shape (same data) |
| `flatten(a)` | Collapse to 1D |
| `T(a)` | Transpose (2D) |
| `get(a, [row, col])` | Element by indices |
| `set(a, [row, col], val)` | Set element (returns new array) |
| `row(a, i)` | Extract row i |
| `col(a, j)` | Extract column j |
| `slice(a, start, stop?)` | Slice flat data |

```weft
m := warp.array([1,2,3,4,5,6], [2, 3])
say(warp.get(m, [0, 2]))     // 3
say(warp.to_list(warp.row(m, 1)))  // [4, 5, 6]
t := warp.T(m)                // [3, 2] shape
```

## Arithmetic (element-wise)

All support array-array and array-scalar:

| Function | Operation |
|----------|-----------|
| `add(a, b)` | a + b |
| `sub(a, b)` | a - b |
| `mul(a, b)` | a * b |
| `div(a, b)` | a / b |
| `mod_(a, b)` | a % b |
| `pow_(a, p)` | a ** p |
| `neg(a)` | -a |
| `abs_(a)` | \|a\| |
| `round_(a, decimals?)` | Round |
| `floor_(a)` / `ceil_(a)` | Floor / ceil |
| `clip(a, lo, hi)` | Clamp values |
| `sign(a)` | -1, 0, or 1 |

```weft
a := warp.array([1, 2, 3], [3])
b := warp.mul(a, 10)            // [10, 20, 30]
c := warp.add(a, warp.ones([3])) // [2, 3, 4]
d := warp.clip(b, 5, 25)       // [10, 20, 25]
```

## Math functions (element-wise)

| Function | |
|----------|--|
| `sqrt`, `exp`, `log_`, `log2`, `log10` | Standard math |
| `sin`, `cos`, `tan`, `asin`, `acos`, `atan`, `atan2` | Trigonometry |
| `sinh`, `cosh`, `tanh` | Hyperbolic |

```weft
angles := warp.linspace(0, 3.14159, 100)
sines := warp.sin(angles)
```

## Comparisons

Return arrays of 0/1:

| Function | |
|----------|--|
| `equal`, `not_equal` | == / != |
| `less`, `less_equal` | < / <= |
| `greater`, `greater_equal` | > / >= |
| `where(cond, x, y)` | Conditional select |
| `all_(a)` / `any_(a)` | Boolean reduction |

```weft
a := warp.array([1, 5, 3, 8], [4])
mask := warp.greater(a, 4)       // [0, 1, 0, 1]
big := warp.mask(a, mask)        // [5, 8]
```

## Reductions

| Function | |
|----------|--|
| `sum`, `prod` | Sum / product of all elements |
| `mean`, `var_`, `std_` | Statistics |
| `min_`, `max_` | Extremes |
| `argmin`, `argmax` | Index of extreme |
| `median`, `percentile(a, p)` | Order statistics |
| `cumsum`, `cumprod` | Cumulative |
| `sum_axis(a, axis)` | Sum along axis (0=cols, 1=rows) |
| `mean_axis`, `min_axis`, `max_axis` | Axis reductions |

```weft
a := warp.array([1,2,3,4,5,6], [2, 3])
say(warp.sum(a))                  // 21
say(warp.to_list(warp.sum_axis(a, 0)))  // [5, 7, 9] (column sums)
say(warp.to_list(warp.sum_axis(a, 1)))  // [6, 15] (row sums)
say(warp.median(a))               // 3.5
say(warp.percentile(a, 75))       // ~4.75
```

## Linear algebra

| Function | |
|----------|--|
| `dot(a, b)` | Dot product (vectors) |
| `matmul(a, b)` | Matrix multiply |
| `norm(a)` / `norm_l1(a)` | L2 / L1 norm |
| `normalize(a)` | Unit vector |
| `trace(a)` | Trace (sum of diagonal) |
| `det(a)` | Determinant (any size via LU decomposition) → Result |
| `inv(a)` | Matrix inverse (any size via LU) → Result |
| `solve(a, b)` | Solve Ax = b (any size via LU) → Result |
| `outer(a, b)` | Outer product |
| `cross(a, b)` | 3D cross product |

```weft
a := warp.array([2, 1, 1, 3], [2, 2])
b := warp.array([5, 11], [2, 1])
x := warp.solve(a, b)?
warp.print_(x)    // [[1], [3]]  (2x+y=5, x+3y=11)
```

## Manipulation

| Function | |
|----------|--|
| `concat(a, b)` | Concatenate |
| `vstack([a, b, …])` | Stack vertically (rows) |
| `hstack([a, b, …])` | Stack horizontally |
| `tile(a, n)` | Repeat array n times |
| `repeat(a, n)` | Repeat each element n times |
| `flip(a)` | Reverse |
| `sort(a)` | Sort ascending |
| `argsort(a)` | Indices that would sort |
| `unique(a)` | Remove duplicates |
| `mask(a, cond)` | Filter by 0/1 array |

## Utility

| Function | |
|----------|--|
| `apply(a, fn)` | Map function over elements |
| `apply2(a, b, fn)` | Map binary function |
| `describe(a)` | Stats summary (shape, min, max, mean, std, median) |
| `allclose(a, b, atol?)` | Approximate equality (default 1e-8) |
| `isnan(a)` / `isinf(a)` | NaN / Inf detection |
| `nan_to_num(a, nan?, posinf?, neginf?)` | Replace NaN/Inf |
| `count_nonzero(a)` | Count non-zero elements |
| `to_list(a)` | Flat data as list |
| `print_(a)` | Pretty-print (rows for 2D) |

```weft
a := warp.rand([1000])
say(warp.describe(a))
// {shape: [1000], size: 1000, min: 0.001..., max: 0.999..., mean: ~0.5, std: ~0.29, median: ~0.5}
```

## Limitations

- Pure Weft — no SIMD, no BLAS. Fine for arrays up to ~10k elements.
- `det`, `inv`, `solve` use LU decomposition with partial pivoting — O(n³), practical up to ~50x50.
- Sort uses insertion sort — O(n²) for large arrays.
- No sparse arrays or complex numbers.
- For heavy numeric work, use `mlinfer` to call ONNX/Triton/HuggingFace.

## API count

93 exported functions across creation (12), shape/indexing (11), arithmetic (14), math (17), comparisons (9), reductions (18), linear algebra (11), manipulation (10), utility (11).
