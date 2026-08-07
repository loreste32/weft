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
- `batches(features, targets, batch_size, opts)` materializes one epoch of
  minibatches as `{"x", "y", "indices"}` maps. Options: `shuffle` (seeded
  Fisher-Yates over sample order), `seed`, and `drop_last`. Every sample
  appears in exactly one batch per call, shuffled or not.

Options are validated and bounded: `epochs`, `batch_size`, `learning_rate`,
and `l2`; `linear_fit`/`logistic_fit` additionally accept `shuffle` and
`seed` (defaults preserve the original sequential order — same seed and data
give bit-identical parameters). The implementation uses minibatches, has a
regression test that trains over 100,000 rows, and can consume the explicit
DataFrame → Warp → model boundary. Packed inputs are materialized once at the
trainer boundary; this remains a classical CPU model layer, separate from the
differentiable array API below.

## Learning-rate schedules

Pure, deterministic functions of the zero-based epoch:

- `step_lr(base_lr, epoch, step_size, gamma)` — `base_lr * gamma^⌊epoch/step_size⌋`.
- `exponential_lr(base_lr, epoch, gamma)` — `base_lr * gamma^epoch`.
- `cosine_lr(base_lr, epoch, total_epochs, min_lr?)` — cosine annealing from
  `base_lr` to `min_lr` (default 0), clamped past `total_epochs`.

## Differentiable losses and activations

Losses (in `losses.weft`) are built on the autograd tape and return scalar
loss nodes, so `backward` differentiates through every trainable input:

- `mse_loss(pred, target)` — mean squared error.
- `binary_cross_entropy(pred, target, eps?)` — log-stability epsilon
  (default 1e-12) keeps exact 0/1 predictions finite.
- `cross_entropy(logits, targets)` — mean softmax cross-entropy over 2-D
  logits `[batch, classes]` with integer class targets; log-sum-exp
  stabilization keeps extreme logits overflow-free.
- `huber_loss(pred, target, delta?)` — quadratic inside `|r| ≤ delta`
  (default 1), linear outside.

Differentiable activation ops mirror the existing `relu`: `sigmoid`, `tanh`,
`gelu` (tanh approximation), and axis-aware `softmax(node, axis?)` with
max-subtraction stabilization. Each also has a parameter-free module wrapper
(`sigmoid_module`, `tanh_module`, `gelu_module`, `softmax_module`) usable in
`sequential` exactly like `relu_module`.

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
`mean`, and two-dimensional `matmul`, plus `sigmoid`, `tanh`, `gelu`,
axis-aware `softmax`, elementwise `huber`, and 2-D `cross_entropy`. The newer
ops use numeric VJPs (under `create_graph` they fall back to numeric
propagation, like array ops). Elementwise broadcasting is supported and
gradients are reduced back to each parent shape. Gradients accumulate across
backward calls; clear a graph with `zero_grad` or `zero_grad_many` before an
independent pass. Pass `create_graph` as `true`/`1` for scalar nested AD, or
`null`/`false` for ordinary numeric grads.

`sgd_step(parameters, learning_rate, weight_decay?)` updates trainable scalar
or Warp-array variables. `adam_state(parameters)` creates moment state and
`adam_step(...)` performs validated bias-corrected Adam updates. Optimizer
steps mutate parameter nodes explicitly; ordinary array operations remain
immutable, and gradients remain accumulated until cleared.

`clip_grad_norm(parameters, max_norm)` rescales all trainable gradients when
their global L2 norm exceeds `max_norm` (PyTorch formula: coefficient
`max_norm / (total_norm + 1e-6)`, applied only when below 1) and returns the
pre-clip norm; zero gradients are a no-op. `clip_grad_value(parameters,
max_abs)` clamps every gradient element to `[-max_abs, max_abs]`.

## Forward-mode autodiff (dual numbers)

`forward.weft` complements the reverse-mode tape with exact forward-mode
differentiation. A dual number pairs a value with its tangent; every
operation applies the chain rule to the tangent, so evaluating `f` once on
`dual(x, v)` yields the exact Jacobian-vector product `J(x) v` — no
finite-difference error and no tape.

- `dual(value, derivative)` constructs a dual over a finite scalar or a
  `warp` array (array tangents are elementwise, same shape; a numeric tangent
  broadcasts). `dual_value` / `dual_derivative` / `is_dual` inspect duals.
- `jvp(f, x, v)` returns `{"value": f(x), "jvp": J(x) v}`. `x` may be a
  scalar, a list of scalars (`f` receives a list of duals), or a warp array;
  `v` must mirror `x`. A constant (non-dual) return from `f` means zero jvp.
- `jacobian(f, x)` returns the gradient of a scalar-output `f` at a list of
  `n` scalars via `n` unit-vector JVPs — one full `f` evaluation per input,
  so keep `n` small. Forward mode is the cheap direction for few-input
  functions; reverse mode stays cheap for few-output losses.
- `derivative(f, x)` is the scalar convenience `f'(x)`.
- The `fwd_*` op set mirrors the tape where practical: `fwd_add`, `fwd_sub`,
  `fwd_mul`, `fwd_div`, `fwd_neg`, `fwd_exp`, `fwd_log`, `fwd_sin`,
  `fwd_cos`, `fwd_sqrt`, `fwd_tanh`, `fwd_sigmoid`, `fwd_relu`, `fwd_pow`,
  `fwd_sum`, `fwd_mean`, and 2-D `fwd_matmul`. Plain numbers/arrays mix with
  duals as constants.

Higher order works through nested dual numbers: `derivative` always seeds a
fresh unit tangent, so `derivative(fn(y) { derivative(f, y) }, x)` evaluates
`f` on duals of duals and yields the exact second derivative (scalar-first,
matching the nested reverse-mode scope).

The forward tests cross-validate three ways — forward jvp/derivative, the
reverse-mode tape gradient, and `gradcheck` finite differences — over a
scalar battery, array functions, and a small linear→tanh→linear→mse MLP, so a
wrong chain-rule term in either mode fails the suite.

`freeze(module, names?)` / `unfreeze(module, names?)` toggle gradient
accumulation on parameters selected by name (see `named_parameters`: a linear
module exposes `weight`/`bias`, a sequential prefixes child indices, e.g.
`0.weight`, `2.bias`; a trailing `*` makes a name a prefix pattern, and `null`
selects all). Unknown names are an error and change nothing. Frozen
parameters receive no gradient from `backward` and are skipped entirely by
`sgd_step`/`adam_step` — values and Adam moments stay bit-identical, so
unfreezing later resumes cleanly.

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

Checkpoint format version 2 adds a top-level `epoch` (taken from
`metadata.epoch` when saving, else 0); version 1 files still load with
`epoch` defaulting to 0. `resume(path, module_or_parameters)` restores the
stored weights and returns `{"epoch", "optimizer", "metadata",
"parameters"}`: pass `optimizer` straight back to `adam_step` and continued
training is bit-identical to an uninterrupted run (JSON round-trips the Adam
moments exactly). When the checkpoint carries no optimizer state, `optimizer`
is `null` — create fresh state then; a checkpoint whose optimizer state does
not match the parameter count is an error.

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

## Device placement and plugin dispatch

`device(name)` returns {"_device": true, "name": "cpu"|"cuda"|"rocm"|"mlx"}.
`to_device(value, device)` returns a distinct warp array or node with a
`_device` field. It does not mutate the caller's value or share a releasable
native tensor handle. CPU always succeeds and returns the tagged value.
Non-CPU requests without a plugin are accepted honestly with a fallback
wrapper `{"value", "fallback": true, "requested", ...}` — compute remains on
the host. `device_of(value)` reports the logical name (`"cpu"` when unset).
There is no fake GPU.

`device_with_plugin(name, path)` additionally loads a native accelerator
provider (see `native/accelerator/`) through the accelerator stdlib. The load
outcome is recorded on the handle (`plugin_available`, `plugin_error`) and
never hidden. With a bound plugin, `to_device` retains the provider handle on
placed arrays and reports the provider probe. `ml.matmul` dispatches through
the binary `tensor_matmul` ABI when both operands use the same plugin; the
returned array is marked as native-dispatched and `exec_info(device)` exposes
the provider's report. Other ML operations and gradients remain explicit host
fallbacks until their ABI operations are implemented.
`exec_info(device)` returns the bound plugin's last execution report, or an
explicit `{"status": "unavailable"}` map when no plugin ran.

## Status

This is a tested numerical-autodiff foundation with SGD/Adam, differentiable
losses (MSE/BCE/cross-entropy/Huber) and activations (sigmoid/tanh/gelu/
softmax), linear/activation/sequential modules, checkpoints, deterministic
seeds, seeded shuffled batching, LR schedules, **scalar nested reverse-mode**
(`create_graph` / double backward), **forward-mode dual numbers** (exact JVP /
jacobian / nested-dual second derivatives), finite-diff HVP, and one tested
plugin-backed tensor operation — not yet a complete PyTorch/JAX replacement.
Sparse/complex autodiff, array-level nested higher-order reverse mode,
broader accelerator operation coverage, async pools, and full NumPy dispatch
remain separate work.

## GPU and native training

For GPU tensor operations or deep models, use `warp` with an explicit native
CUDA, ROCm/HIP, or Apple MLX provider through the versioned accelerator ABI.
The provider owns device memory and kernels; `ml` owns model/data contracts and
validation. Real vendor providers must publish supported dtypes, layouts,
operations, limits, and numerical checks. No package claims that its pure
Weft CPU implementation is equivalent to vendor libraries.
