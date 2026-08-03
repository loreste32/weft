//go:build js

package stdlib

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

type browserHTTPResponse struct {
	status  int64
	body    string
	headers map[string]string
}

const maxBrowserHTTPBodyBytes = 32 << 20

type browserHTTPResult struct {
	response browserHTTPResponse
	err      error
}

func packageHTTP(env *runtime.Env) runtime.Value {
	p := pkg()
	set(p, "get", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("http.get(url, opts?)", "http"), nil
		}
		return browserHTTPDo(env, "GET", args[0].String(), "", optArg(args, 1)), nil
	}, 2)
	set(p, "post", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("http.post(url, body?, opts?)", "http"), nil
		}
		body := ""
		if len(args) > 1 {
			body = args[1].String()
		}
		return browserHTTPDo(env, "POST", args[0].String(), body, optArg(args, 2)), nil
	}, 3)
	set(p, "put", func(args []runtime.Value) (runtime.Value, error) {
		return browserHTTPMethod(env, "PUT", args), nil
	}, 3)
	set(p, "patch", func(args []runtime.Value) (runtime.Value, error) {
		return browserHTTPMethod(env, "PATCH", args), nil
	}, 3)
	set(p, "delete", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("http.delete(url, opts?)", "http"), nil
		}
		return browserHTTPDo(env, "DELETE", args[0].String(), "", optArg(args, 1)), nil
	}, 2)
	set(p, "request", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("http.request(method, url, opts?)", "http"), nil
		}
		opts := optArg(args, 2)
		return browserHTTPDo(env, args[0].String(), args[1].String(), mapGetStr(opts, "body", ""), opts), nil
	}, 3)
	set(p, "fetch", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindMap {
			return errRes("http.fetch(opts)", "http"), nil
		}
		opts := args[0]
		url := mapGetStr(opts, "url", "")
		if url == "" {
			return errRes("http.fetch: url required", "http"), nil
		}
		return browserHTTPDo(env, mapGetStr(opts, "method", "GET"), url, mapGetStr(opts, "body", ""), opts), nil
	}, 1)
	set(p, "get_json", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("http.get_json(url, opts?)", "http"), nil
		}
		response := browserHTTPDo(env, "GET", args[0].String(), "", optArg(args, 1))
		if !resultOK(response) {
			return response, nil
		}
		ro := response.Obj.(*runtime.ResultObj)
		if status, ok := mapGet(ro.Val, "status"); ok {
			if code, err := runtime.AsInt(status); err == nil && (code < 200 || code >= 300) {
				return errRes(fmt.Sprintf("http.get_json: HTTP %d", code), "http"), nil
			}
		}
		body := mapGetStr(ro.Val, "body", "")
		var value any
		decoder := json.NewDecoder(stringsReader(body))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return errRes("http.get_json: "+err.Error(), "http"), nil
		}
		return runtime.Ok(goToValue(value)), nil
	}, 2)
	set(p, "text", func(args []runtime.Value) (runtime.Value, error) {
		status, body := int64(200), ""
		if len(args) > 0 {
			if value, err := runtime.AsInt(args[0]); err == nil {
				status = value
			} else {
				body = args[0].String()
			}
		}
		if len(args) > 1 {
			body = args[1].String()
		}
		return browserResponseValue(browserHTTPResponse{status: status, body: body, headers: map[string]string{"content-type": "text/plain; charset=utf-8"}}), nil
	}, 2)
	set(p, "json", func(args []runtime.Value) (runtime.Value, error) {
		status, value := int64(200), runtime.Null()
		if len(args) == 1 {
			value = args[0]
		} else if len(args) >= 2 {
			if code, err := runtime.AsInt(args[0]); err == nil {
				status, value = code, args[1]
			} else {
				value = args[0]
			}
		}
		body, err := jsonMarshal(value)
		if err != nil {
			return errRes(err.Error(), "http"), nil
		}
		return browserResponseValue(browserHTTPResponse{status: status, body: body, headers: map[string]string{"content-type": "application/json"}}), nil
	}, 2)
	set(p, "serve", func(args []runtime.Value) (runtime.Value, error) {
		return errRes("http.serve is not available in browser Wasm; use a browser fetch handler", "http"), nil
	}, 2)
	set(p, "post_form", func(args []runtime.Value) (runtime.Value, error) {
		return errRes("http.post_form is not available in browser Wasm; use http.fetch with a serialized body", "http"), nil
	}, 3)
	return p
}

func browserHTTPMethod(env *runtime.Env, method string, args []runtime.Value) runtime.Value {
	if len(args) < 1 {
		return errRes("http."+method+"(url, body?, opts?)", "http")
	}
	body := ""
	if len(args) > 1 {
		body = args[1].String()
	}
	return browserHTTPDo(env, method, args[0].String(), body, optArg(args, 2))
}

func browserHTTPDo(env *runtime.Env, method, url, body string, opts runtime.Value) runtime.Value {
	if !env.BrowserAsync {
		return errRes("browser HTTP requires runAsync(); synchronous Wasm calls cannot wait for fetch", "http")
	}
	if url == "" {
		return errRes("http: url required", "http")
	}
	if len([]byte(body)) > maxBrowserHTTPBodyBytes {
		return errRes(fmt.Sprintf("http request body exceeds %d MiB limit", maxBrowserHTTPBodyBytes>>20), "http")
	}
	headers := map[string]string{}
	if value, ok := mapGet(opts, "headers"); ok {
		values, err := asMap(value)
		if err != nil {
			return errRes("http: headers must be a map", "http")
		}
		for key, value := range values {
			headers[key] = fmt.Sprint(value)
		}
	}
	if body != "" && method != "GET" && method != "HEAD" {
		if _, ok := headers["Content-Type"]; !ok {
			headers["Content-Type"] = "application/json"
		}
	}
	ctx := env.Context()
	if milliseconds := mapGetInt(opts, "timeout_ms", 0); milliseconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(milliseconds)*time.Millisecond)
		defer cancel()
	} else if seconds := mapGetInt(opts, "timeout", 0); seconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
		defer cancel()
	}
	response, err := browserFetch(ctx, method, url, body, headers)
	if err != nil {
		return errRes(err.Error(), "http")
	}
	return runtime.Ok(browserResponseValue(response))
}

func browserFetch(ctx context.Context, method, url, body string, headers map[string]string) (result browserHTTPResponse, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("browser fetch: %v", recovered)
		}
	}()
	fetch := js.Global().Get("fetch")
	if fetch.Type() != js.TypeFunction {
		return result, fmt.Errorf("browser fetch API is unavailable")
	}

	options := js.Global().Get("Object").New()
	options.Set("method", method)
	if body != "" && method != "GET" && method != "HEAD" {
		options.Set("body", body)
	}
	headerObject := js.Global().Get("Object").New()
	for key, value := range headers {
		if !browserHTTPHeaderSafe(key) || !browserHTTPHeaderSafe(value) {
			return result, fmt.Errorf("invalid HTTP header")
		}
		headerObject.Set(key, value)
	}
	options.Set("headers", headerObject)

	var controller js.Value
	if ctor := js.Global().Get("AbortController"); ctor.Type() == js.TypeFunction {
		controller = ctor.New()
		options.Set("signal", controller.Get("signal"))
	}

	done := make(chan browserHTTPResult, 1)
	send := func(value browserHTTPResult) {
		select {
		case done <- value:
		default:
		}
	}
	var timeoutTimer js.Value
	var timeoutFunc js.Func
	hasTimeoutFunc := false
	timedOut := false
	var textResolve *js.Func
	var bodyRead *js.Func
	var resolve js.Func
	var reject js.Func
	cleanup := func() {
		if timeoutTimer.Type() != js.TypeUndefined && js.Global().Get("clearTimeout").Type() == js.TypeFunction {
			js.Global().Call("clearTimeout", timeoutTimer)
		}
		// Promise callbacks may still be unwinding when the Go waiter receives
		// the result. Release them on the next event-loop turn.
		var release js.Func
		release = js.FuncOf(func(this js.Value, args []js.Value) any {
			resolve.Release()
			reject.Release()
			if textResolve != nil {
				textResolve.Release()
			}
			if bodyRead != nil {
				bodyRead.Release()
			}
			if hasTimeoutFunc {
				timeoutFunc.Release()
			}
			release.Release()
			return nil
		})
		if js.Global().Get("setTimeout").Type() == js.TypeFunction {
			js.Global().Call("setTimeout", release, 0)
		} else {
			release.Invoke()
			release.Release()
		}
	}
	reject = js.FuncOf(func(this js.Value, args []js.Value) any {
		if timedOut {
			send(browserHTTPResult{err: context.DeadlineExceeded})
			return nil
		}
		message := "fetch failed"
		if len(args) > 0 && args[0].Truthy() {
			message = args[0].String()
		}
		send(browserHTTPResult{err: fmt.Errorf("%s", message)})
		return nil
	})
	resolve = js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) < 1 {
			send(browserHTTPResult{err: fmt.Errorf("fetch returned no response")})
			return nil
		}
		response := args[0]
		headersOut := map[string]string{}
		jsHeaders := response.Get("headers")
		if jsHeaders.Type() != js.TypeUndefined && jsHeaders.Get("forEach").Type() == js.TypeFunction {
			headerFn := js.FuncOf(func(this js.Value, values []js.Value) any {
				if len(values) >= 2 {
					headersOut[values[1].String()] = values[0].String()
				}
				return nil
			})
			jsHeaders.Call("forEach", headerFn)
			headerFn.Release()
		}
		if jsHeaders.Type() != js.TypeUndefined && jsHeaders.Get("get").Type() == js.TypeFunction {
			contentLength := strings.TrimSpace(jsHeaders.Call("get", "content-length").String())
			if contentLength != "" {
				if size, parseErr := strconv.ParseInt(contentLength, 10, 64); parseErr == nil && size >= 0 {
					if size > maxBrowserHTTPBodyBytes {
						send(browserHTTPResult{err: fmt.Errorf("http response body exceeds %d MiB limit", maxBrowserHTTPBodyBytes>>20)})
						return nil
					}
				}
			}
		}
		readableBody := response.Get("body")
		textDecoder := js.Global().Get("TextDecoder")
		if readableBody.Type() == js.TypeObject && readableBody.Get("getReader").Type() == js.TypeFunction && textDecoder.Type() == js.TypeFunction {
			reader := readableBody.Call("getReader")
			decoder := textDecoder.New()
			decodeOptions := js.Global().Get("Object").New()
			decodeOptions.Set("stream", true)
			var bodyText strings.Builder
			var bytesRead int64
			var readFn js.Func
			readFn = js.FuncOf(func(this js.Value, values []js.Value) any {
				if len(values) < 1 {
					send(browserHTTPResult{err: fmt.Errorf("http response body read returned no result")})
					return nil
				}
				readResult := values[0]
				if readResult.Get("done").Bool() {
					bodyText.WriteString(decoder.Call("decode").String())
					send(browserHTTPResult{response: browserHTTPResponse{
						status:  int64(response.Get("status").Int()),
						body:    bodyText.String(),
						headers: headersOut,
					}})
					return nil
				}
				chunk := readResult.Get("value")
				chunkBytes := int64(chunk.Get("byteLength").Int())
				if chunkBytes < 0 || bytesRead > maxBrowserHTTPBodyBytes-chunkBytes {
					_ = reader.Call("cancel")
					send(browserHTTPResult{err: fmt.Errorf("http response body exceeds %d MiB limit", maxBrowserHTTPBodyBytes>>20)})
					return nil
				}
				bytesRead += chunkBytes
				bodyText.WriteString(decoder.Call("decode", chunk, decodeOptions).String())
				reader.Call("read").Call("then", readFn).Call("catch", reject)
				return nil
			})
			bodyRead = &readFn
			reader.Call("read").Call("then", readFn).Call("catch", reject)
			return nil
		}
		status := response.Get("status").Int()
		if method != "HEAD" && status != 204 && status != 205 && status != 304 {
			send(browserHTTPResult{err: fmt.Errorf("http response body size cannot be bounded without a readable stream")})
			return nil
		}
		textPromise := response.Call("text")
		textFn := js.FuncOf(func(this js.Value, values []js.Value) any {
			bodyText := ""
			if len(values) > 0 {
				bodyText = values[0].String()
			}
			if len([]byte(bodyText)) > maxBrowserHTTPBodyBytes {
				send(browserHTTPResult{err: fmt.Errorf("http response body exceeds %d MiB limit", maxBrowserHTTPBodyBytes>>20)})
				return nil
			}
			status := int64(response.Get("status").Int())
			send(browserHTTPResult{response: browserHTTPResponse{status: status, body: bodyText, headers: headersOut}})
			return nil
		})
		textResolve = &textFn
		textPromise.Call("then", textFn).Call("catch", reject)
		return nil
	})
	promise := fetch.Invoke(js.ValueOf(url), options)
	promise.Call("then", resolve).Call("catch", reject)
	if deadline, ok := ctx.Deadline(); ok && controller.Type() != js.TypeUndefined {
		milliseconds := time.Until(deadline) / time.Millisecond
		if milliseconds < 1 {
			milliseconds = 1
		}
		timeoutFunc = js.FuncOf(func(this js.Value, args []js.Value) any {
			timedOut = true
			controller.Call("abort")
			return nil
		})
		hasTimeoutFunc = true
		timeoutTimer = js.Global().Call("setTimeout", timeoutFunc, int(milliseconds))
	}

	select {
	case value := <-done:
		cleanup()
		return value.response, value.err
	case <-ctx.Done():
		timedOut = true
		if controller.Type() != js.TypeUndefined {
			controller.Call("abort")
		}
		cleanup()
		return result, ctx.Err()
	}
}

func browserResponseValue(response browserHTTPResponse) runtime.Value {
	value := runtime.NewMap()
	mo := value.Obj.(*runtime.MapObj)
	mo.Keys = append(mo.Keys, "status", "body", "headers")
	mo.Vals["status"] = runtime.Int(response.status)
	mo.Vals["body"] = runtime.Str(response.body)
	headers := runtime.NewMap()
	hm := headers.Obj.(*runtime.MapObj)
	keys := make([]string, 0, len(response.headers))
	for key := range response.headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		hm.Keys = append(hm.Keys, key)
		hm.Vals[key] = runtime.Str(response.headers[key])
	}
	mo.Vals["headers"] = headers
	return value
}

func resultOK(value runtime.Value) bool {
	return value.Kind == runtime.KindResult && value.Obj.(*runtime.ResultObj).Ok
}
func stringsReader(value string) *strings.Reader { return strings.NewReader(value) }
func browserHTTPHeaderSafe(value string) bool {
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return false
		}
	}
	return true
}
