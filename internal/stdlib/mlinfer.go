//go:build !js

package stdlib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
		input := valueToGo(args[1])

		timeout := 30 * time.Second
		headers := map[string]string{"Content-Type": "application/json"}
		if len(args) >= 3 && args[2].Kind == runtime.KindMap {
			mo := args[2].Obj.(*runtime.MapObj)
			if v, ok := mo.Vals["timeout"]; ok {
				if n, e := runtime.AsInt(v); e == nil {
					if n <= 0 || n > 3600 {
						return errRes("timeout must be between 1 and 3600 seconds", "mlinfer"), nil
					}
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
		if model == "" {
			return errRes("mlinfer.onnx requires a model", "mlinfer"), nil
		}
		input, err := asMap(args[2])
		if err != nil {
			return errRes("mlinfer.onnx input must be a map", "mlinfer"), nil
		}
		url := fmt.Sprintf("%s/v1/models/%s/infer", base, url.PathEscape(model))
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
		if model == "" {
			return errRes("mlinfer.triton requires a model", "mlinfer"), nil
		}
		input, err := asMap(args[2])
		if err != nil {
			return errRes("mlinfer.triton input must be a map", "mlinfer"), nil
		}

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
		if model == "" {
			return errRes("mlinfer.hf requires a model", "mlinfer"), nil
		}
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
			inputs[i] = valueToGo(item)
		}

		body := map[string]any{"inputs": inputs}
		headers := map[string]string{"Content-Type": "application/json"}
		return doInferenceRequest(url, body, headers, 60*time.Second)
	}, 3)

	return p
}

func doInferenceRequest(rawURL string, body any, headers map[string]string, timeout time.Duration) (runtime.Value, error) {
	if err := validateInferenceURL(rawURL); err != nil {
		return errRes(err.Error(), "mlinfer"), nil
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return errRes("inference request JSON: "+err.Error(), "mlinfer"), nil
	}
	req, err := http.NewRequest(http.MethodPost, rawURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return errRes("inference request: "+err.Error(), "mlinfer"), nil
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return errRes("inference request: "+err.Error(), "mlinfer"), nil
	}
	defer resp.Body.Close()
	respBody, err := readInferenceBody(resp.Body)
	if err != nil {
		return errRes("inference response: "+err.Error(), "mlinfer"), nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return errRes(fmt.Sprintf("inference HTTP %d: %s", resp.StatusCode, responsePreview(respBody)), "mlinfer"), nil
	}

	var result any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return errRes("inference response was not valid JSON: "+err.Error(), "mlinfer"), nil
	}
	return runtime.Ok(goToValue(result)), nil
}

const maxInferenceBody = 32 << 20

func validateInferenceURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("inference URL must be an absolute HTTP(S) URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("inference URL must use http or https")
	}
	return nil
}

func readInferenceBody(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxInferenceBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxInferenceBody {
		return nil, fmt.Errorf("response exceeds %d MiB", maxInferenceBody/(1<<20))
	}
	return body, nil
}

func responsePreview(body []byte) string {
	const maxPreview = 4096
	if len(body) <= maxPreview {
		return string(body)
	}
	return string(body[:maxPreview]) + "…"
}

func checkHealth(url string) (runtime.Value, error) {
	if err := validateInferenceURL(url); err != nil {
		return runtime.Ok(runtime.Bool(false)), nil
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return runtime.Ok(runtime.Bool(false)), nil
	}
	resp.Body.Close()
	return runtime.Ok(runtime.Bool(resp.StatusCode == 200)), nil
}

func doGet(url string) (runtime.Value, error) {
	if err := validateInferenceURL(url); err != nil {
		return errRes(err.Error(), "mlinfer"), nil
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return errRes(err.Error(), "mlinfer"), nil
	}
	defer resp.Body.Close()
	body, err := readInferenceBody(resp.Body)
	if err != nil {
		return errRes("inference response: "+err.Error(), "mlinfer"), nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return errRes(fmt.Sprintf("inference HTTP %d: %s", resp.StatusCode, responsePreview(body)), "mlinfer"), nil
	}
	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return errRes("inference response was not valid JSON: "+err.Error(), "mlinfer"), nil
	}
	return runtime.Ok(goToValue(result)), nil
}
