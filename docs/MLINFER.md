# ML inference

Call ML models running on inference servers. Weft never loads models in-process — this package talks to ONNX Runtime, NVIDIA Triton, HuggingFace Inference API, or any custom HTTP endpoint.

Inference URLs must be absolute `http://` or `https://` URLs. Successful
responses must be JSON and use a 2xx status; transport errors, non-2xx status,
invalid JSON, and responses larger than 32 MiB return `Err`. `predict` preserves
JSON scalars, arrays, and objects in its request body.

```weft
use mlinfer
```

---

## Generic inference

Call any endpoint that accepts JSON:

```weft
fn main -> Result {
    result := mlinfer.predict("http://model-server:8080/predict", {
        "text": "This product is amazing",
    })?
    say(result)
}
```

With auth and timeout:

```weft
result := mlinfer.predict("https://api.example.com/v1/classify", {
    "text": "refund my order",
}, {
    "api_key": env.get("API_KEY"),
    "timeout": 10,
})?
```

---

## ONNX Runtime Server

```weft
fn main -> Result {
    base := "http://localhost:8001"

    // health check
    say(mlinfer.onnx_health(base)?)  // true

    // list models
    models := mlinfer.onnx_models(base)?
    say(models)

    // run inference
    result := mlinfer.onnx(base, "sentiment", {
        "text": "This is a great product",
    })?
    say(result)
}
```

---

## NVIDIA Triton

```weft
fn main -> Result {
    base := "http://gpu-box:8000"

    say(mlinfer.triton_health(base)?)
    say(mlinfer.triton_models(base)?)

    // Triton expects specific input format
    result := mlinfer.triton(base, "bert-base", {
        "inputs": [{
            "name": "input_ids",
            "shape": [1, 128],
            "datatype": "INT64",
            "data": tokens,
        }],
    })?
    say(result)
}
```

---

## HuggingFace Inference API

```weft
fn main -> Result {
    // text classification
    result := mlinfer.hf("facebook/bart-large-mnli", {
        "inputs": "This is urgent and needs immediate attention",
    })?
    say(result)

    // with API key
    result := mlinfer.hf("meta-llama/Llama-2-7b-chat-hf", {
        "inputs": "What is Weft?",
    }, {"api_key": env.get("HF_API_KEY")})?
    say(result)
}
```

Env: `HF_API_KEY` or `HUGGINGFACE_API_KEY`.

---

## Task shortcuts

```weft
fn main -> Result {
    // classify text
    label := mlinfer.classify("http://localhost:8080/classify", "refund my order")?
    say(label)

    // generate embeddings
    vec := mlinfer.embed("http://localhost:8080/embed", "search query")?
    say(len(vec))

    // object detection
    boxes := mlinfer.detect("http://localhost:8080/detect", "https://example.com/photo.jpg")?
    say(boxes)
}
```

---

## Batch inference

Send multiple inputs in one request:

```weft
fn main -> Result {
    results := mlinfer.batch("http://localhost:8080/classify", [
        "I love this product",
        "Terrible experience",
        "It's okay I guess",
    ])?
    for r in results { say(r) }
}
```

---

## Members

| Function | What it does |
|----------|-------------|
| `predict(url, input, opts?)` | Generic inference call |
| `onnx(base, model, input)` | ONNX Runtime Server |
| `onnx_health(base)` | ONNX health check |
| `onnx_models(base)` | List ONNX models |
| `triton(base, model, input)` | NVIDIA Triton |
| `triton_health(base)` | Triton health check |
| `triton_models(base)` | List Triton models |
| `hf(model, input, opts?)` | HuggingFace Inference |
| `classify(url, text)` | Text classification shortcut |
| `embed(url, text)` | Embedding shortcut |
| `detect(url, image_url)` | Object detection shortcut |
| `batch(url, [inputs])` | Batched inference |
