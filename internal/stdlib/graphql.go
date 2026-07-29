//go:build !js

package stdlib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// packageGraphQL — GraphQL HTTP client (queries & mutations).
//
//	res := graphql.query(url, query, variables?, opts?)?
//	// res.data, res.errors
func packageGraphQL(env *runtime.Env) runtime.Value {
	p := pkg()

	do := func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("graphql.query(url, query, variables?, opts?)", "graphql"), nil
		}
		url := args[0].String()
		query := args[1].String()
		var variables any
		if len(args) >= 3 && args[2].Kind != runtime.KindNull {
			variables = valueToGo(args[2])
		}
		opts := runtime.Null()
		if len(args) >= 4 {
			opts = args[3]
		} else if len(args) >= 3 && (args[2].Kind == runtime.KindMap) {
			// allow (url, query, opts) when third has headers/operation
			if _, ok := mapGet(args[2], "headers"); ok {
				opts = args[2]
				variables = nil
			}
			if _, ok := mapGet(args[2], "operation"); ok && variables == nil {
				opts = args[2]
			}
		}
		opName := ""
		headers := map[string]string{"Content-Type": "application/json"}
		if opts.Kind == runtime.KindMap || opts.Kind == runtime.KindStruct {
			opName = mapGetStr(opts, "operation", "")
			if opName == "" {
				opName = mapGetStr(opts, "operationName", "")
			}
			if h, ok := mapGet(opts, "headers"); ok {
				if hm, err := asMap(h); err == nil {
					for k, v := range hm {
						headers[k] = fmt.Sprint(v)
					}
				}
			}
			if t := mapGetStr(opts, "token", ""); t != "" {
				headers["Authorization"] = "Bearer " + t
			}
		}

		body := map[string]any{"query": query}
		if variables != nil {
			body["variables"] = variables
		}
		if opName != "" {
			body["operationName"] = opName
		}
		raw, _ := json.Marshal(body)

		client := env.HTTPClient
		if client == nil {
			client = &http.Client{Timeout: 60 * time.Second}
		}
		req, err := http.NewRequestWithContext(env.Context(), "POST", url, bytes.NewReader(raw))
		if err != nil {
			return errRes(err.Error(), "graphql"), nil
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if client == nil {
			client = DefaultHTTPClient()
		}
		resp, err := client.Do(req)
		if err != nil {
			return errRes(err.Error(), "graphql"), nil
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
		if err != nil {
			return errRes(err.Error(), "graphql"), nil
		}
		if resp.StatusCode >= 300 {
			return errRes(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncStr(string(b), 200)), "graphql"), nil
		}
		var parsed map[string]any
		if err := json.Unmarshal(b, &parsed); err != nil {
			return errRes(err.Error(), "graphql"), nil
		}
		// GraphQL errors
		if errs, ok := parsed["errors"]; ok && errs != nil {
			// still return data if partial, but mark ok=false via Result Err if no data
			if parsed["data"] == nil {
				return errRes(fmt.Sprint(errs), "graphql"), nil
			}
		}
		return runtime.Ok(goToValue(parsed)), nil
	}

	set(p, "query", do, 4)
	set(p, "mutation", do, 4) // alias — same HTTP shape
	set(p, "request", do, 4)

	return p
}

func truncStr(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
