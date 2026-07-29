//go:build !js

package stdlib

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// packageMLInfer — ML inference clients for ONNX Runtime, Triton, HuggingFace, custom endpoints.
// Weft never loads models in-process. This talks to inference servers over HTTP.
func packageMLInfer(env *runtime.Env) runtime.Value {
	p := pkg()

	// ─── generic inference ────────────────────────────────────────

	// mlinfer.predict(url, input, opts?) -> Result[map]
	// Call any inference endpoint with JSON input.
	set(p, "predict", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("mlinfer.predict(url, input, opts?)", "mlinfer"), nil
		}
		url := args[0].String()
		input, err := asMap(args[1])
		if err != nil {
			input = map[string]any{"input": args[1].String()}
		}

		timeout := 30 * time.Second
		headers := map[string]string{"Content-Type": "application/json"}
		if len(args) >= 3 && args[2].Kind == runtime.KindMap {
			mo := args[2].Obj.(*runtime.MapObj)
			if v, ok := mo.Vals["timeout"]; ok {
				if n, e := runtime.AsInt(v); e == nil {
					timeout = time.Duration(n) * time.Second
				}
			}
			if v, ok := mo.Vals["headers"]; ok && v.Kind == runtime.KindMap {
				hmo := v.Obj.(*runtime.MapObj)
				for _, k := range hmo.Keys {
					headers[k] = hmo.Vals[k].String()
				}
			}
			if v, ok := mo.Vals["api_key"]; ok && v.Kind != runtime.KindNull {
				headers["Authorization"] = "Bearer " + v.String()
			}
		}

		return doInferenceRequest(url, input, headers, timeout)
	}, 3)

	// ─── ONNX Runtime Server ──────────────────────────────────────

	// mlinfer.onnx(base_url, model, input, opts?) -> Result[map]
	// ONNX Runtime Server: POST /v1/models/{model}/infer
	set(p, "onnx", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 {
			return errRes("mlinfer.onnx(base_url, model, input)", "mlinfer"), nil
		}
		base := strings.TrimRight(args[0].String(), "/")
		model := args[1].String()
		input, _ := asMap(args[2])
		url := fmt.Sprintf("%s/v1/models/%s/infer", base, model)
		return doInferenceRequest(url, input, map[string]string{"Content-Type": "application/json"}, 30*time.Second)
	}, 4)

	// mlinfer.onnx_health(base_url) -> Result[bool]
	set(p, "onnx_health", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("mlinfer.onnx_health(base_url)", "mlinfer"), nil
		}
		base := strings.TrimRight(args[0].String(), "/")
		return checkHealth(base + "/health")
	}, 1)

	// mlinfer.onnx_models(base_url) -> Result[[str]]
	set(p, "onnx_models", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("mlinfer.onnx_models(base_url)", "mlinfer"), nil
		}
		base := strings.TrimRight(args[0].String(), "/")
		return doGet(base + "/v1/models")
	}, 1)

	// ─── Triton Inference Server ──────────────────────────────────

	// mlinfer.triton(base_url, model, input, opts?) -> Result[map]
	// NVIDIA Triton: POST /v2/models/{model}/infer
	set(p, "triton", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 {
			return errRes("mlinfer.triton(base_url, model, input)", "mlinfer"), nil
		}
		base := strings.TrimRight(args[0].String(), "/")
		model := args[1].String()
		input, _ := asMap(args[2])

		// Triton expects {inputs: [{name, shape, datatype, data}]}
		body := input
		if _, ok := body["inputs"]; !ok {
			// wrap simple input into Triton format
			body = map[string]any{"inputs": []any{input}}
		}

		url := fmt.Sprintf("%s/v2/models/%s/infer", base, model)
		return doInferenceRequest(url, body, map[string]string{"Content-Type": "application/json"}, 30*time.Second)
	}, 4)

	// mlinfer.triton_health(base_url) -> Result[bool]
	set(p, "triton_health", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("mlinfer.triton_health(base_url)", "mlinfer"), nil
		}
		base := strings.TrimRight(args[0].String(), "/")
		return checkHealth(base + "/v2/health/ready")
	}, 1)

	// mlinfer.triton_models(base_url) -> Result[map]
	set(p, "triton_models", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("mlinfer.triton_models(base_url)", "mlinfer"), nil
		}
		base := strings.TrimRight(args[0].String(), "/")
		return doGet(base + "/v2/models")
	}, 1)

	// ─── HuggingFace Inference API ────────────────────────────────

	// mlinfer.hf(model, input, opts?) -> Result[map]
	// HuggingFace Inference API
	set(p, "hf", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("mlinfer.hf(model, input, opts?)", "mlinfer"), nil
		}
		model := args[0].String()
		apiKey := ""
		if k, ok := getenv(env, "HF_API_KEY"); ok {
			apiKey = k
		}
		if k, ok := getenv(env, "HUGGINGFACE_API_KEY"); ok {
			apiKey = k
		}
		if len(args) >= 3 && args[2].Kind == runtime.KindMap {
			mo := args[2].Obj.(*runtime.MapObj)
			if v, ok := mo.Vals["api_key"]; ok && v.Kind != runtime.KindNull {
				apiKey = v.String()
			}
		}

		url := fmt.Sprintf("https://api-inference.huggingface.co/models/%s", model)
		headers := map[string]string{"Content-Type": "application/json"}
		if apiKey != "" {
			headers["Authorization"] = "Bearer " + apiKey
		}

		var input map[string]any
		if args[1].Kind == runtime.KindMap {
			input, _ = asMap(args[1])
		} else {
			input = map[string]any{"inputs": args[1].String()}
		}

		return doInferenceRequest(url, input, headers, 60*time.Second)
	}, 3)

	// ─── classify / embed / detect shortcuts ──────────────────────

	// mlinfer.classify(url, text, opts?) -> Result[map]
	// Shortcut for text classification endpoints.
	set(p, "classify", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("mlinfer.classify(url, text)", "mlinfer"), nil
		}
		url := args[0].String()
		input := map[string]any{"text": args[1].String()}
		headers := map[string]string{"Content-Type": "application/json"}
		if len(args) >= 3 && args[2].Kind == runtime.KindMap {
			mo := args[2].Obj.(*runtime.MapObj)
			if v, ok := mo.Vals["api_key"]; ok && v.Kind != runtime.KindNull {
				headers["Authorization"] = "Bearer " + v.String()
			}
		}
		return doInferenceRequest(url, input, headers, 15*time.Second)
	}, 3)

	// mlinfer.embed(url, text, opts?) -> Result[[float]]
	// Shortcut for embedding endpoints. Returns vector.
	set(p, "embed", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("mlinfer.embed(url, text)", "mlinfer"), nil
		}
		url := args[0].String()
		input := map[string]any{"input": args[1].String()}
		headers := map[string]string{"Content-Type": "application/json"}
		if len(args) >= 3 && args[2].Kind == runtime.KindMap {
			mo := args[2].Obj.(*runtime.MapObj)
			if v, ok := mo.Vals["api_key"]; ok && v.Kind != runtime.KindNull {
				headers["Authorization"] = "Bearer " + v.String()
			}
			if v, ok := mo.Vals["model"]; ok && v.Kind != runtime.KindNull {
				input["model"] = v.String()
			}
		}
		return doInferenceRequest(url, input, headers, 15*time.Second)
	}, 3)

	// mlinfer.detect(url, image_url, opts?) -> Result[map]
	// Shortcut for object detection endpoints.
	set(p, "detect", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("mlinfer.detect(url, image_url)", "mlinfer"), nil
		}
		url := args[0].String()
		input := map[string]any{"image": args[1].String()}
		headers := map[string]string{"Content-Type": "application/json"}
		return doInferenceRequest(url, input, headers, 30*time.Second)
	}, 3)

	// ─── batch ────────────────────────────────────────────────────

	// mlinfer.batch(url, inputs, opts?) -> Result[[map]]
	// Send multiple inputs in one request.
	set(p, "batch", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("mlinfer.batch(url, [inputs])", "mlinfer"), nil
		}
		url := args[0].String()
		if args[1].Kind != runtime.KindList {
			return errRes("inputs must be a list", "mlinfer"), nil
		}
		lo := args[1].Obj.(*runtime.ListObj)
		inputs := make([]any, len(lo.Items))
		for i, item := range lo.Items {
			if m, err := asMap(item); err == nil {
				inputs[i] = m
			} else {
				inputs[i] = item.String()
			}
		}

		body := map[string]any{"inputs": inputs}
		headers := map[string]string{"Content-Type": "application/json"}
		return doInferenceRequest(url, body, headers, 60*time.Second)
	}, 3)

	return p
}

func doInferenceRequest(url string, body map[string]any, headers map[string]string, timeout time.Duration) (runtime.Value, error) {
	bodyJSON, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(bodyJSON)))
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return errRes("inference request: "+err.Error(), "mlinfer"), nil
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return errRes(fmt.Sprintf("inference HTTP %d: %s", resp.StatusCode, string(respBody)), "mlinfer"), nil
	}

	var result any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return runtime.Ok(runtime.Str(string(respBody))), nil
	}
	return runtime.Ok(goToValue(result)), nil
}

func checkHealth(url string) (runtime.Value, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return runtime.Ok(runtime.Bool(false)), nil
	}
	resp.Body.Close()
	return runtime.Ok(runtime.Bool(resp.StatusCode == 200)), nil
}

func doGet(url string) (runtime.Value, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return errRes(err.Error(), "mlinfer"), nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return runtime.Ok(runtime.Str(string(body))), nil
	}
	return runtime.Ok(goToValue(result)), nil
}
