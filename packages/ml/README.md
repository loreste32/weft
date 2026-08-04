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
  model.
- `logistic_fit(features, targets, opts)` trains binary logistic regression;
  targets are `0`/`1`.
- `fit(kind, features, targets, opts)` and `train(...)` dispatch either model.
- `predict(model, features)` returns values or probabilities.
- `predict_classes(model, features, threshold?)` returns binary classes.
- `score(model, features, targets)` returns MSE for linear models and accuracy
  for logistic models.
- `standardize(features)` returns standardized values plus mean and scale.

Options are validated and bounded: `epochs`, `batch_size`, `learning_rate`,
and `l2`. The implementation uses minibatches and has a regression test that
trains over 100,000 rows. It is a classical CPU model layer; the differentiable
array API below is separate from this trainer.

## Other APIs

`cosine`, `dot`, `norm`, `topk`, `accuracy`, `mse`, `split`, embedding
providers, and the JSON vector index are available from the package entry.
Embedding calls use the configured OpenAI-compatible/Ollama provider and may
require network capabilities.

## Autodiff

The package provides a portable reverse-mode tape over scalars and `warp`
arrays. In addition to `variable`, `constant`, `backward`, `grad`, and
`zero_grad`, the differentiable operations are `add`, `sub`, `mul`, `div`,
`neg`, `exp`, `log`, `relu`, `pow`, `sum`, `mean`, and two-dimensional `matmul`.
Elementwise broadcasting is supported and gradients are reduced back to each
parent shape. Gradients accumulate across backward calls; clear a graph with
`zero_grad` or `zero_grad_many` before an independent pass.

`sgd_step(parameters, learning_rate, weight_decay?)` updates trainable scalar
or Warp-array variables. `adam_state(parameters)` creates moment state and
`adam_step(...)` performs validated bias-corrected Adam updates. Optimizer
steps mutate parameter nodes explicitly; ordinary array operations remain
immutable, and gradients remain accumulated until cleared.

This is a tested numerical-autodiff and optimizer foundation, not yet a
complete PyTorch/JAX replacement: modules, serialization, sparse/complex
autodiff, higher-order derivatives, schedulers, and full NumPy dispatch remain
separate work.

## GPU and native training

For GPU tensor operations or deep models, use `warp` with an explicit native
CUDA, ROCm/HIP, or Apple MLX provider through the versioned accelerator ABI.
The provider owns device memory and kernels; `ml` owns model/data contracts and
validation. Real vendor providers must publish supported dtypes, layouts,
operations, limits, and numerical checks. No package claims that its pure
Weft CPU implementation is equivalent to vendor libraries.
