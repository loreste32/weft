# ML in Weft

Weft now has two explicit ML layers: `packages/ml` for portable classical
models and `packages/warp` for validated numerical arrays. Native CUDA,
ROCm/HIP, and Apple MLX providers are loaded through the capability-gated
`accelerator` ABI in [`native/accelerator`](../native/accelerator).

## Package layers

| Layer | Role |
|---|---|
| `packages/ml` | vectors, embeddings, metrics, standardization, minibatch linear/logistic training |
| `packages/warp` | N-dimensional CPU arrays, broadcasting, reductions, linear algebra, native dispatch |
| `accelerator` | explicit shared-library loading and bounded JSON operation dispatch |
| `mlinfer` | remote ONNX Runtime, Triton, HuggingFace, and custom HTTP inference |
| `weft train` | fine-tune orchestration for external/private training workflows |

## Classical training

```weft
use ml

fn main -> Result {
    model := ml.fit("linear", [[0.0], [1.0], [2.0]], [1.0, 3.0, 5.0], {
        "epochs": 300,
        "learning_rate": 0.05,
        "batch_size": 64,
    })?
    say(ml.predict(model, [[4.0]])?)
    say(ml.score(model, [[4.0]], [9.0])?)
}
```

`linear_fit` uses minibatch least squares. `logistic_fit` uses minibatch
binary cross-entropy. Options are bounded, input matrices are validated, and
the test suite trains a model over 100,000 rows. This is a deterministic
classical CPU implementation. `ml` also exposes a tested reverse-mode tape
for scalars and `warp` arrays, including elementwise broadcasting, reductions,
and two-dimensional matrix multiplication, plus a forward-mode dual-number
layer (`dual`, `jvp`, `jacobian`, `derivative`, `fwd_*` ops) that computes
exact Jacobian-vector products at one function evaluation per input direction
— cheap for few-input functions, where reverse mode is cheap for few-output
losses. The two modes are cross-checked against each other and against
finite-difference `gradcheck` in the test suite.
`sgd_step` and stateful `adam_step`
provide validated optimizer updates for scalar and Warp-array parameters.
`linear_module` and `linear_forward` provide a tested trainable two-dimensional
module. `state_dict`, `save_checkpoint`, and `load_checkpoint` persist values,
optimizer state, and metadata without serializing executable graph links.
`step_lr`, `exponential_lr`, and `cosine_lr` learning-rate schedulers are
available; sparse/complex autodiff, exact array-level higher-order reverse
mode, and a complete deep-learning framework remain future work; numerical
array HVP/Hessian helpers are available.

## Classical algorithms

`kmeans(features, k, opts)` provides deterministic dense numeric Lloyd iterations and returns centers, labels, inertia, and iteration count. `knn_predict(features, targets, queries, k, weighted?)` and its `knn` alias provide dense numeric nearest-neighbor classification with deterministic tie-breaking; labels may be arbitrary comparable values. `decision_tree_fit`/`decision_tree_predict` provide bounded numeric classification trees (up to 4,096 rows and 65,536 input cells). These APIs do not claim sparse matrices, categorical encoders, regression trees, or accelerator-resident execution.

## Native GPU/provider execution

```weft
use warp

fn main -> Result {
    plugin := warp.accelerator_load("/opt/weft/libweft_accel_rocm.so")?
    result := warp.accelerator_run(plugin, "matmul", {
        "a": [1.0, 2.0, 3.0, 4.0], "a_shape": [2, 2],
        "b": [5.0, 6.0, 7.0, 8.0], "b_shape": [2, 2],
    })?
    say(result)
    warp.accelerator_close(plugin)?
}
```

The host ABI is vendor-neutral and does not link vendor SDKs into the main
binary. A CUDA provider uses the CUDA Runtime API; a ROCm provider may use HIP
or rocBLAS; an Apple provider may use MLX. They are separate binaries and must
declare their actual vendor, supported operations, dtypes, layouts, device
constraints, and numerical guarantees. The checked-in example provider is a
loader smoke test only.

Native loading is explicit because shared libraries execute host code and may
access device memory, drivers, or files. The `accelerator` capability must be
granted in a package manifest, and browser WASM reports native loading as
unavailable.

## Remote inference

For models already served by ONNX Runtime or Triton, use `mlinfer` and keep
weights/drivers outside Weft:

```weft
use mlinfer

fn main -> Result {
    result := mlinfer.triton("http://gpu-box:8000", "model", {
        "inputs": [{"name": "input", "shape": [1, 3], "datatype": "FP32", "data": [1, 2, 3]}],
    })?
    say(result)
}
```

## Claims and boundaries

Weft can replace Python for the supported classical training, tabular, array,
and provider-dispatch workflows. It is not yet a drop-in replacement for all
NumPy/pandas APIs, dtype systems, sparse arrays, FFTs, the full breadth of
deep-learning module/scheduler APIs (ml covers linear/activation modules and
step/exponential/cosine LR schedules), exact array-level higher-order autodiff,
or Python ecosystem libraries. Numerical array HVP/Hessian helpers do not
claim exact nested reverse-mode or device-resident Hessians. Each native provider must be benchmarked and numerically
validated independently; an ABI load success is not evidence that a GPU
operation is correct.
