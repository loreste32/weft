# ml — Weft ML module

`ml` covers vector/embedding workflows and deterministic classical model
training without leaving the Weft program. It includes minibatch linear and
logistic regression, predictions, scores, standardization, metrics, train/test
splitting, embeddings, and a small JSON vector index.

```weft
use ml

fn main -> Result {
    model := ml.linear_fit([[0.0], [1.0], [2.0]], [1.0, 3.0, 5.0], {
        "epochs": 300, "learning_rate": 0.05, "batch_size": 64,
    })?
    say(ml.predict(model, [[4.0]])?)
}
```

## Training API

- `linear_fit(features, targets, opts)` trains a minibatch least-squares
  models. Features may be nested numeric lists or a packed 2-D Warp array;
  targets may be a list or packed 1-D Warp array.
- `logistic_fit(features, targets, opts)` trains binary logistic regression;
  targets are `0`/`1` and accept the same list/Warp forms.
- `fit(kind, features, targets, opts)` and `train(...)` dispatch either model.
- `predict(model, features)` returns values or probabilities.
- `predict_classes(model, features, threshold?)` returns binary classes.
- `score(model, features, targets)` returns MSE for linear models and accuracy
  for logistic models.
- `standardize(features)` returns standardized values plus mean and scale.

Options are validated and bounded: `epochs`, `batch_size`, `learning_rate`,
and `l2`. The implementation uses minibatches, has a regression test that
trains over 100,000 rows, and can consume the explicit DataFrame → Warp →
model boundary. Packed inputs are materialized once at the trainer boundary;
this remains a classical CPU model layer, separate from the differentiable
array API below.

## Other APIs

`cosine`, `dot`, `norm`, `topk`, `accuracy`, `mse`, `split`, embedding
providers, and the JSON vector index are available from the package entry.
Embedding calls use the configured OpenAI-compatible/Ollama provider and may
require network capabilities.

## Autodiff

The package provides a portable reverse-mode tape over scalars and `warp`
arrays. In addition to `variable`, `constant`, `backward(node, create_graph?)`,
`grad`, `grad_fn`, `grad_value`, and `zero_grad`, the differentiable operations
are `add`, `sub`, `mul`, `div`, `neg`, `exp`, `log`, `relu`, `pow`, `sum`,
`mean`, and two-dimensional `matmul`. Elementwise broadcasting is supported and
gradients are reduced back to each parent shape. Gradients accumulate across
backward calls; clear a graph with `zero_grad` or `zero_grad_many` before an
independent pass. Pass `create_graph` as `true`/`1` for scalar nested AD, or
`null`/`false` for ordinary numeric grads.

`sgd_step(parameters, learning_rate, weight_decay?)` updates trainable scalar
or Warp-array variables. `adam_state(parameters)` creates moment state and
`adam_step(...)` performs validated bias-corrected Adam updates. Optimizer
steps mutate parameter nodes explicitly; ordinary array operations remain
immutable, and gradients remain accumulated until cleared.

## Modules and checkpoints

`linear_module(in_features, out_features)` creates a trainable linear module
with Warp-array weight and bias nodes. `linear_forward(module, input)` builds
an autograd-compatible two-dimensional forward pass, and
`module_parameters(module)` returns the parameter nodes for an optimizer.

`state_dict(parameters)` extracts values without graph links.
`save_checkpoint(path, parameters, optimizer_state, metadata)` writes a
versioned JSON checkpoint, and `load_checkpoint(path)` validates and returns
it for `load_state_dict`. Checkpoints are intentionally value-only and do not
serialize executable functions or autograd graph links.

## Higher-order derivatives

`gradcheck` compares reverse-mode grads to finite differences.

**Scalar nested reverse-mode** (create_graph): `backward(node, true)` builds the
VJP with autograd ops for scalar ops (`add`/`sub`/`mul`/`div`/`neg`/`exp`/
`log`/`relu`/`pow`/`sum`/`mean`). Afterward `x.grad` is itself a node, so
`backward(x.grad)` yields second derivatives. `grad_fn(output, inputs)` returns
those gradient nodes. Array/matmul paths stay numeric (scalar nested first).

```weft
x := ml.variable(2.0)?
y := ml.pow(x, 3.0)?          // f = x^3
ml.backward(y, true)?         // x.grad = 3x^2 (node, value 12)
g := ml.grad(x)?
ml.zero_grad(g)?
ml.backward(g, null)?         // x.grad = 6x = 12
```

`second_derivative(scalar_fn, x, eps?)` uses nested double-backward when the
first gradient is a graph node; otherwise falls back to centered finite
differences of analytical first gradients. `hvp` remains finite-diff only.

- `second_derivative`: scalar variable; e.g. \(f(x)=x^3\) yields \(f''=6x\).
- `hvp`: Hessian-vector product \(Hv \approx (\nabla L(\theta+\varepsilon v)-\nabla L(\theta-\varepsilon v))/(2\varepsilon)\).

## Device placement (advisory)

`device(name)` returns {"_device": true, "name": "cpu"|"cuda"|"rocm"|"mlx"}.
`to_device(value, device)` returns a distinct warp array or node with a
`_device` field. It does not mutate the caller's value or share a releasable
native tensor handle. CPU always succeeds and returns the tagged value.
Non-CPU requests are accepted honestly with a fallback wrapper
`{"value", "fallback": true, "requested"}` — pure Weft has no GPU, so compute
remains on the host. `device_of(value)` reports the logical name (`"cpu"` when
unset). Placement is metadata until accelerator providers are bound; there is
no fake GPU.

## Status

This is a tested numerical-autodiff foundation with SGD/Adam, linear/relu/
sequential modules, checkpoints, deterministic seeds, **scalar nested
reverse-mode** (`create_graph` / double backward), finite-diff HVP, and advisory
device tags — not yet a complete PyTorch/JAX replacement. Sparse/complex
autodiff, array-level nested higher-order reverse mode, bound accelerator
execution, async pools, schedulers, and full NumPy dispatch remain separate work.

## GPU and native training

For GPU tensor operations or deep models, use `warp` with an explicit native
CUDA, ROCm/HIP, or Apple MLX provider through the versioned accelerator ABI.
The provider owns device memory and kernels; `ml` owns model/data contracts and
validation. Real vendor providers must publish supported dtypes, layouts,
operations, limits, and numerical checks. No package claims that its pure
Weft CPU implementation is equivalent to vendor libraries.
