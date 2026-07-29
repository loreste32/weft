//go:build !js

package stdlib

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// packageMCP — Model Context Protocol client and server helpers.
// MCP lets Weft scripts connect to AI tool servers or expose functions as MCP tools.
func packageMCP(env *runtime.Env) runtime.Value {
	p := pkg()

	// ─── client: connect to MCP servers ───────────────────────────

	// mcp.connect(command, args?) -> Result[map]
	// Spawn an MCP server process (stdio transport) and return a client handle.
	set(p, "connect", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("mcp.connect(command, args?)", "mcp"), nil
		}
		cmd := args[0].String()
		var cmdArgs []string
		if len(args) >= 2 && args[1].Kind == runtime.KindList {
			lo := args[1].Obj.(*runtime.ListObj)
			for _, a := range lo.Items {
				cmdArgs = append(cmdArgs, a.String())
			}
		}
		client, err := newMCPStdioClient(cmd, cmdArgs)
		if err != nil {
			return errRes(err.Error(), "mcp"), nil
		}
		// initialize
		if err := client.initialize(); err != nil {
			client.close()
			return errRes("mcp init: "+err.Error(), "mcp"), nil
		}
		return runtime.Ok(wrapMCPClient(client)), nil
	}, 2)

	// mcp.connect_sse(url) -> Result[map]
	// Connect to an MCP server via HTTP+SSE transport.
	set(p, "connect_sse", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("mcp.connect_sse(url)", "mcp"), nil
		}
		url := args[0].String()
		client, err := newMCPSSEClient(url)
		if err != nil {
			return errRes(err.Error(), "mcp"), nil
		}
		if err := client.initialize(); err != nil {
			return errRes("mcp init: "+err.Error(), "mcp"), nil
		}
		return runtime.Ok(wrapMCPClient(client)), nil
	}, 1)

	// ─── server: expose weft functions as MCP tools ───────────────

	// mcp.tool(name, description, fn, schema?) -> map
	// Define an MCP tool from a Weft function.
	set(p, "tool", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 {
			return errRes("mcp.tool(name, description, fn, schema?)", "mcp"), nil
		}
		name := args[0].String()
		desc := args[1].String()
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		putMap := func(k string, v runtime.Value) {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = v
		}
		putMap("name", runtime.Str(name))
		putMap("description", runtime.Str(desc))
		putMap("fn", args[2])
		if len(args) >= 4 {
			putMap("schema", args[3])
		}
		return m, nil
	}, 4)

	// mcp.serve_stdio(tools) -> unit
	// Run an MCP server on stdio (for use with AI assistants).
	// tools: [mcp.tool(...), ...]
	set(p, "serve_stdio", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return errRes("mcp.serve_stdio([tools])", "mcp"), nil
		}
		lo := args[0].Obj.(*runtime.ListObj)
		tools := make([]mcpToolDef, 0, len(lo.Items))
		for _, item := range lo.Items {
			if item.Kind != runtime.KindMap {
				continue
			}
			imo := item.Obj.(*runtime.MapObj)
			td := mcpToolDef{
				Name: imo.Vals["name"].String(),
				Desc: imo.Vals["description"].String(),
				Fn:   imo.Vals["fn"],
			}
			if schema, ok := imo.Vals["schema"]; ok && schema.Kind != runtime.KindNull {
				td.Schema = schema
			}
			tools = append(tools, td)
		}
		srv := &mcpStdioServer{
			tools: tools,
			env:   env,
		}
		srv.run()
		return runtime.Unit(), nil
	}, 1)

	return p
}

// ─── MCP types ────────────────────────────────────────────────────

// mcpClient is the interface both stdio and SSE clients implement.
type mcpClient interface {
	call(method string, params any) (json.RawMessage, error)
	initialize() error
	close()
}

type mcpToolDef struct {
	Name   string
	Desc   string
	Fn     runtime.Value
	Schema runtime.Value
}

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ─── stdio MCP client ─────────────────────────────────────────────

type mcpStdioClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	mu     sync.Mutex
	nextID atomic.Int64
}

func newMCPStdioClient(command string, args []string) (*mcpStdioClient, error) {
	cmd := exec.Command(command, args...)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", command, err)
	}
	return &mcpStdioClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
	}, nil
}

func (c *mcpStdioClient) call(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID.Add(1)
	reqBytes, _ := json.Marshal(params)
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  reqBytes,
	}
	data, _ := json.Marshal(req)
	data = append(data, '\n')
	if _, err := c.stdin.Write(data); err != nil {
		return nil, err
	}

	// read response line
	line, err := c.stdout.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	var resp jsonRPCResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("mcp error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	result, _ := json.Marshal(resp.Result)
	return result, nil
}

func (c *mcpStdioClient) initialize() error {
	_, err := c.call("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "weft", "version": "0.3.34"},
	})
	if err != nil {
		return err
	}
	// send initialized notification
	notif, _ := json.Marshal(jsonRPCRequest{JSONRPC: "2.0", Method: "notifications/initialized"})
	notif = append(notif, '\n')
	c.mu.Lock()
	c.stdin.Write(notif)
	c.mu.Unlock()
	return nil
}

func (c *mcpStdioClient) close() {
	c.stdin.Close()
	c.cmd.Process.Kill()
	c.cmd.Wait()
}

func wrapMCPClient(c mcpClient) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	putFn := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("mcp.client."+name, arity, fn)
	}

	// client.list_tools() -> Result[[map]]
	putFn("list_tools", 0, func(args []runtime.Value) (runtime.Value, error) {
		raw, err := c.call("tools/list", map[string]any{})
		if err != nil {
			return errRes(err.Error(), "mcp"), nil
		}
		var result struct {
			Tools []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"tools"`
		}
		json.Unmarshal(raw, &result)
		items := make([]runtime.Value, 0, len(result.Tools))
		for _, t := range result.Tools {
			tm := runtime.NewMap()
			tmo := tm.Obj.(*runtime.MapObj)
			tmo.Keys = append(tmo.Keys, "name", "description")
			tmo.Vals["name"] = runtime.Str(t.Name)
			tmo.Vals["description"] = runtime.Str(t.Description)
			items = append(items, tm)
		}
		return runtime.Ok(runtime.List(items...)), nil
	})

	// client.call_tool(name, args?) -> Result[map]
	putFn("call_tool", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("client.call_tool(name, args?)", "mcp"), nil
		}
		name := args[0].String()
		toolArgs := map[string]any{}
		if len(args) >= 2 && args[1].Kind == runtime.KindMap {
			toolArgs, _ = asMap(args[1])
		}
		raw, err := c.call("tools/call", map[string]any{
			"name":      name,
			"arguments": toolArgs,
		})
		if err != nil {
			return errRes(err.Error(), "mcp"), nil
		}
		v, err := jsonToValue(raw)
		if err != nil {
			return errRes(err.Error(), "mcp"), nil
		}
		return runtime.Ok(v), nil
	})

	// client.list_resources() -> Result[[map]]
	putFn("list_resources", 0, func(args []runtime.Value) (runtime.Value, error) {
		raw, err := c.call("resources/list", map[string]any{})
		if err != nil {
			return errRes(err.Error(), "mcp"), nil
		}
		v, err := jsonToValue(raw)
		if err != nil {
			return errRes(err.Error(), "mcp"), nil
		}
		return runtime.Ok(v), nil
	})

	// client.read_resource(uri) -> Result[map]
	putFn("read_resource", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("client.read_resource(uri)", "mcp"), nil
		}
		raw, err := c.call("resources/read", map[string]any{"uri": args[0].String()})
		if err != nil {
			return errRes(err.Error(), "mcp"), nil
		}
		v, err := jsonToValue(raw)
		if err != nil {
			return errRes(err.Error(), "mcp"), nil
		}
		return runtime.Ok(v), nil
	})

	// client.close()
	putFn("close", 0, func(args []runtime.Value) (runtime.Value, error) {
		c.close()
		return runtime.Unit(), nil
	})

	return m
}

// ─── SSE MCP client ───────────────────────────────────────────────

type mcpSSEClient struct {
	baseURL    string
	httpClient *http.Client
	mu         sync.Mutex
	nextID     atomic.Int64
}

func newMCPSSEClient(url string) (*mcpSSEClient, error) {
	return &mcpSSEClient{
		baseURL:    strings.TrimRight(url, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *mcpSSEClient) call(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID.Add(1)
	reqBytes, _ := json.Marshal(params)
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  reqBytes,
	}
	data, _ := json.Marshal(req)

	resp, err := c.httpClient.Post(c.baseURL+"/mcp", "application/json", strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, err
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("mcp error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	result, _ := json.Marshal(rpcResp.Result)
	return result, nil
}

func (c *mcpSSEClient) close() {}

func (c *mcpSSEClient) initialize() error {
	_, err := c.call("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "weft", "version": "0.3.34"},
	})
	return err
}

// ─── stdio MCP server ─────────────────────────────────────────────

type mcpStdioServer struct {
	tools []mcpToolDef
	env   *runtime.Env
}

func (s *mcpStdioServer) run() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		resp := s.handle(req)
		if resp != nil {
			data, _ := json.Marshal(resp)
			fmt.Println(string(data))
		}
	}
}

func (s *mcpStdioServer) handle(req jsonRPCRequest) *jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    "weft",
					"version": "0.3.34",
				},
			},
		}
	case "notifications/initialized":
		return nil // notification, no response
	case "tools/list":
		tools := make([]map[string]any, 0, len(s.tools))
		for _, t := range s.tools {
			tool := map[string]any{
				"name":        t.Name,
				"description": t.Desc,
			}
			if t.Schema.Kind == runtime.KindMap {
				schema, _ := asMap(t.Schema)
				tool["inputSchema"] = schema
			} else {
				tool["inputSchema"] = map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				}
			}
			tools = append(tools, tool)
		}
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{"tools": tools},
		}
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		json.Unmarshal(req.Params, &params)

		// find tool
		var tool *mcpToolDef
		for i := range s.tools {
			if s.tools[i].Name == params.Name {
				tool = &s.tools[i]
				break
			}
		}
		if tool == nil {
			return &jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &rpcError{Code: -32601, Message: "unknown tool: " + params.Name},
			}
		}

		// call the Weft function
		fnArgs := goToValue(params.Arguments)
		result, err := callWeftFn(s.env, tool.Fn, []runtime.Value{fnArgs})
		if err != nil {
			return &jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"content": []map[string]any{
						{"type": "text", "text": "error: " + err.Error()},
					},
					"isError": true,
				},
			}
		}
		text := result.String()
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": text},
				},
			},
		}
	default:
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: "method not found: " + req.Method},
		}
	}
}

// ─── helpers ──────────────────────────────────────────────────────

func jsonToValue(raw json.RawMessage) (runtime.Value, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return runtime.Null(), err
	}
	return goToValue(v), nil
}

// goToValue is in helpers.go

func callWeftFn(env *runtime.Env, fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
	switch fn.Kind {
	case runtime.KindBuiltin:
		bo := fn.Obj.(*runtime.BuiltinObj)
		return bo.Fn(args)
	default:
		return runtime.Null(), fmt.Errorf("not a callable function")
	}
}
