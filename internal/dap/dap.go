// Package dap implements a minimal Debug Adapter Protocol (JSON-RPC over stdio)
// for Weft, wrapping the VM's breakpoint/step hooks.
package dap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/loreste/weft/internal/compile"
	"github.com/loreste/weft/internal/jsonrpc"
	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/internal/stdlib"
	"github.com/loreste/weft/internal/vm"
)

// Run starts the DAP session on r/w (usually stdin/stdout).
// program may be empty — then launch request must supply it.
func Run(r io.Reader, w io.Writer, program string) error {
	s := &session{
		w:          w,
		program:    program,
		threads:    []map[string]any{{"id": 1, "name": "main"}},
		cont:       make(chan contAction, 1),
		done:       make(chan struct{}),
		varByRef:   map[int][]variable{},
		nextVarRef: 2, // 1 reserved for Locals scope
	}
	br := bufio.NewReader(r)
	for {
		msg, err := readMessage(br)
		if err != nil {
			if err == io.EOF {
				s.shutdownVM()
				return nil
			}
			return err
		}
		if err := s.handle(msg); err != nil {
			return err
		}
		if s.exit {
			s.shutdownVM()
			return nil
		}
	}
}

type contAction int

const (
	contContinue contAction = iota
	contStep
	contDisconnect
)

type variable struct {
	Name  string
	Value string
	Type  string
}

type session struct {
	w       io.Writer
	mu      sync.Mutex
	seq     int64
	exit    bool
	program string

	// runtime
	ds          *vm.DebugState
	machine     *vm.VM
	prog        *compile.Program
	running     bool
	finished    bool
	stopOnEntry bool

	// pause snapshot
	paused     bool
	lastLoc    vm.FrameLoc
	lastStack  []vm.FrameLoc
	locals     map[string]runtime.Value
	varByRef   map[int][]variable
	nextVarRef int

	// coordination
	cont chan contAction
	done chan struct{} // closed when program finishes

	threads []map[string]any
}

func (s *session) handle(raw []byte) error {
	var env struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil
	}
	// Events from client (no id) or requests
	cmd := env.Method
	switch cmd {
	case "initialize":
		if err := s.replyCmd(env.ID, cmd, map[string]any{
			"supportsConfigurationDoneRequest": true,
			"supportsStepInTargetsRequest":     false,
			"supportsStepBack":                 false,
			"supportsRestartRequest":           false,
			"supportsSetVariable":              false,
			"supportsGotoTargetsRequest":       false,
			"supportTerminateDebuggee":         true,
			"supportsCancelRequest":            false,
			"supportsEvaluateForHovers":        false,
		}); err != nil {
			return err
		}
		// DAP: after successful initialize, adapter sends initialized event
		return s.event("initialized", map[string]any{})
	case "initialized":
		// not a request we receive; client sends this as event sometimes — ignore
		return nil
	case "launch":
		return s.onLaunch(env.ID, env.Params)
	case "setBreakpoints":
		return s.onSetBreakpoints(env.ID, env.Params)
	case "configurationDone":
		return s.onConfigurationDone(env.ID)
	case "threads":
		return s.replyCmd(env.ID, cmd, map[string]any{"threads": s.threads})
	case "stackTrace":
		return s.onStackTrace(env.ID)
	case "scopes":
		return s.onScopes(env.ID)
	case "variables":
		return s.onVariables(env.ID, env.Params)
	case "continue":
		return s.onContinue(env.ID)
	case "next", "stepIn", "stepOut":
		return s.onStep(env.ID, env.Method)
	case "disconnect", "terminate":
		return s.onDisconnect(env.ID)
	case "evaluate":
		return s.onEvaluate(env.ID, env.Params)
	default:
		// unknown request with id → empty success
		if len(env.ID) > 0 && string(env.ID) != "null" {
			return s.replyCmd(env.ID, cmd, map[string]any{})
		}
		return nil
	}
}

func (s *session) onLaunch(id json.RawMessage, params json.RawMessage) error {
	var p struct {
		Program     string `json:"program"`
		StopOnEntry bool   `json:"stopOnEntry"`
		NoDebug     bool   `json:"noDebug"`
		Args        []any  `json:"args"`
		Cwd         string `json:"cwd"`
	}
	_ = json.Unmarshal(params, &p)
	if p.Program != "" {
		s.program = p.Program
	}
	s.stopOnEntry = p.StopOnEntry
	if s.program == "" {
		return s.replyError(id, 1, "launch: missing program")
	}
	abs, err := filepath.Abs(s.program)
	if err != nil {
		return s.replyError(id, 1, err.Error())
	}
	s.program = abs
	if err := s.prepare(abs); err != nil {
		return s.replyError(id, 1, err.Error())
	}
	// Client follows with setBreakpoints + configurationDone
	return s.replyCmd(id, "launch", map[string]any{})
}

func (s *session) prepare(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	file, perrs := parse.ParseFile(path, string(src))
	if perrs.HasErrors() {
		return fmt.Errorf("parse: %v", perrs)
	}
	env := runtime.NewEnv()
	stdlib.Register(env, stdlib.Options{})
	env.Set("__args", runtime.List(runtime.Str(path)))
	prog, cerrs := compile.CompileFile(file, env)
	if cerrs.HasErrors() {
		return fmt.Errorf("compile: %v", cerrs)
	}
	if prog.Main == nil {
		return fmt.Errorf("no main function")
	}
	s.prog = prog
	s.ds = vm.NewDebugState()
	s.ds.OnPause = s.onPause
	s.machine = vm.New(env)
	s.machine.Debug = s.ds
	return nil
}

func (s *session) onPause(loc vm.FrameLoc, locals map[string]runtime.Value) {
	s.mu.Lock()
	s.paused = true
	s.lastLoc = loc
	s.locals = locals
	if s.ds != nil {
		s.lastStack = append([]vm.FrameLoc(nil), s.ds.LastStack...)
	}
	s.rebuildVariables()
	s.mu.Unlock()

	reason := "breakpoint"
	if s.ds != nil && s.ds.StepMode {
		reason = "step"
	}
	_ = s.event("stopped", map[string]any{
		"reason":            reason,
		"threadId":          1,
		"allThreadsStopped": true,
	})

	// Block VM until client continues/steps/disconnects
	act := <-s.cont
	s.mu.Lock()
	s.paused = false
	s.mu.Unlock()
	if act == contDisconnect {
		// Leave step mode off; RunFunc will keep going unless we panic.
		// Force stop by setting a breakpoint-only path and clearing step — still runs.
		// Best effort: re-enter step forever until disconnect handled at top level.
		s.ds.StepMode = false
		// Close-ish: run to end without more pauses
		s.ds.OnPause = func(vm.FrameLoc, map[string]runtime.Value) {}
	}
}

func (s *session) rebuildVariables() {
	vars := make([]variable, 0, len(s.locals))
	for name, val := range s.locals {
		if strings.HasPrefix(name, "__") {
			continue
		}
		vars = append(vars, variable{
			Name:  name,
			Value: val.String(),
			Type:  val.KindName(),
		})
	}
	// stable-ish order
	for i := 0; i < len(vars); i++ {
		for j := i + 1; j < len(vars); j++ {
			if vars[j].Name < vars[i].Name {
				vars[i], vars[j] = vars[j], vars[i]
			}
		}
	}
	s.varByRef = map[int][]variable{1: vars}
	s.nextVarRef = 2
}

func (s *session) onSetBreakpoints(id json.RawMessage, params json.RawMessage) error {
	var p struct {
		Source struct {
			Path string `json:"path"`
		} `json:"source"`
		Breakpoints []struct {
			Line int `json:"line"`
		} `json:"breakpoints"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return s.replyError(id, 1, "bad setBreakpoints params")
	}
	path := p.Source.Path
	if path != "" {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	} else {
		path = s.program
	}
	// Clear prior BPs for this file
	if s.ds != nil {
		prefix := path + ":"
		for k := range s.ds.Breakpoints {
			if strings.HasPrefix(k, prefix) {
				delete(s.ds.Breakpoints, k)
			}
		}
		// also clear basename keys for this file
		base := filepath.Base(path) + ":"
		for k := range s.ds.Breakpoints {
			if strings.HasPrefix(k, base) && !strings.Contains(k, string(filepath.Separator)) {
				// keep absolute keys only for other files
			}
		}
	}
	out := make([]map[string]any, 0, len(p.Breakpoints))
	for i, bp := range p.Breakpoints {
		key := fmt.Sprintf("%s:%d", path, bp.Line)
		if s.ds != nil {
			s.ds.Breakpoints[key] = true
			// Also register basename form if chunk uses relative paths
			s.ds.Breakpoints[fmt.Sprintf("%s:%d", filepath.Base(path), bp.Line)] = true
		}
		out = append(out, map[string]any{
			"id":       i + 1,
			"verified": true,
			"line":     bp.Line,
		})
	}
	return s.replyCmd(id, "setBreakpoints", map[string]any{"breakpoints": out})
}

func (s *session) onConfigurationDone(id json.RawMessage) error {
	if err := s.replyCmd(id, "configurationDone", map[string]any{}); err != nil {
		return err
	}
	// Start program
	if s.prog == nil || s.running {
		return nil
	}
	s.running = true
	if s.stopOnEntry {
		s.ds.StepMode = true
	} else {
		s.ds.StepMode = false
	}
	go s.runProgram()
	return nil
}

func (s *session) runProgram() {
	defer close(s.done)
	_, err := s.machine.RunFunc(s.prog.Main, nil)
	s.mu.Lock()
	s.finished = true
	s.running = false
	s.paused = false
	s.mu.Unlock()
	if err != nil {
		_ = s.event("output", map[string]any{
			"category": "stderr",
			"output":   err.Error() + "\n",
		})
		_ = s.event("stopped", map[string]any{
			"reason":   "exception",
			"threadId": 1,
			"text":     err.Error(),
		})
	}
	_ = s.event("terminated", map[string]any{})
	_ = s.event("exited", map[string]any{"exitCode": 0})
}

func (s *session) onStackTrace(id json.RawMessage) error {
	s.mu.Lock()
	stack := append([]vm.FrameLoc(nil), s.lastStack...)
	loc := s.lastLoc
	s.mu.Unlock()
	if len(stack) == 0 && loc.Line > 0 {
		stack = []vm.FrameLoc{loc}
	}
	frames := make([]map[string]any, 0, len(stack))
	for i, f := range stack {
		path := f.File
		if path == "" {
			path = s.program
		}
		frames = append(frames, map[string]any{
			"id":     i + 1,
			"name":   f.Func,
			"line":   f.Line,
			"column": 1,
			"source": map[string]any{
				"name": filepath.Base(path),
				"path": path,
			},
		})
	}
	return s.replyCmd(id, "stackTrace", map[string]any{
		"stackFrames": frames,
		"totalFrames": len(frames),
	})
}

func (s *session) onScopes(id json.RawMessage) error {
	return s.replyCmd(id, "scopes", map[string]any{
		"scopes": []map[string]any{
			{
				"name":               "Locals",
				"variablesReference": 1,
				"expensive":          false,
			},
		},
	})
}

func (s *session) onVariables(id json.RawMessage, params json.RawMessage) error {
	var p struct {
		VariablesReference int `json:"variablesReference"`
	}
	_ = json.Unmarshal(params, &p)
	s.mu.Lock()
	vars := s.varByRef[p.VariablesReference]
	s.mu.Unlock()
	out := make([]map[string]any, 0, len(vars))
	for _, v := range vars {
		out = append(out, map[string]any{
			"name":               v.Name,
			"value":              v.Value,
			"type":               v.Type,
			"variablesReference": 0,
		})
	}
	return s.replyCmd(id, "variables", map[string]any{"variables": out})
}

func (s *session) onContinue(id json.RawMessage) error {
	if s.ds != nil {
		s.ds.StepMode = false
		// Skip re-hitting the current line once (set in VM on pause; reinforce here).
		if s.lastLoc.File != "" && s.lastLoc.Line > 0 {
			s.ds.SkipBP = fmt.Sprintf("%s:%d", s.lastLoc.File, s.lastLoc.Line)
		}
	}
	s.signal(contContinue)
	return s.replyCmd(id, "continue", map[string]any{"allThreadsContinued": true})
}

func (s *session) onStep(id json.RawMessage, method string) error {
	if s.ds != nil {
		s.ds.StepMode = true
	}
	s.signal(contStep)
	return s.replyCmd(id, method, map[string]any{})
}

func (s *session) onDisconnect(id json.RawMessage) error {
	s.signal(contDisconnect)
	s.exit = true
	return s.replyCmd(id, "disconnect", map[string]any{})
}

func (s *session) onEvaluate(id json.RawMessage, params json.RawMessage) error {
	var p struct {
		Expression string `json:"expression"`
	}
	_ = json.Unmarshal(params, &p)
	expr := strings.TrimSpace(p.Expression)

	// Try direct local lookup first
	s.mu.Lock()
	v, ok := s.locals[expr]
	s.mu.Unlock()
	if ok {
		return s.replyCmd(id, "evaluate", map[string]any{
			"result":             v.String(),
			"type":               v.KindName(),
			"variablesReference": 0,
		})
	}

	// Try field access: "x.y" → lookup x, then access field y
	if dot := strings.Index(expr, "."); dot > 0 {
		root := expr[:dot]
		field := expr[dot+1:]
		s.mu.Lock()
		rv, rok := s.locals[root]
		s.mu.Unlock()
		if rok && rv.Kind == runtime.KindMap {
			if mo, mok := rv.Obj.(*runtime.MapObj); mok {
				if fv, fok := mo.Vals[field]; fok {
					return s.replyCmd(id, "evaluate", map[string]any{
						"result":             fv.String(),
						"type":               fv.KindName(),
						"variablesReference": 0,
					})
				}
			}
		}
		if rok && rv.Kind == runtime.KindStruct {
			if so, sok := rv.Obj.(*runtime.StructObj); sok {
				if fv, fok := so.Fields[field]; fok {
					return s.replyCmd(id, "evaluate", map[string]any{
						"result":             fv.String(),
						"type":               fv.KindName(),
						"variablesReference": 0,
					})
				}
			}
		}
	}

	// Try bracket access: "x[0]" or "x["key"]"
	if bracket := strings.Index(expr, "["); bracket > 0 {
		root := expr[:bracket]
		s.mu.Lock()
		rv, rok := s.locals[root]
		s.mu.Unlock()
		if rok && rv.Kind == runtime.KindList {
			idxStr := strings.Trim(expr[bracket+1:], "[] ")
			if idx, err := strconv.Atoi(idxStr); err == nil {
				if lo, lok := rv.Obj.(*runtime.ListObj); lok && idx >= 0 && idx < len(lo.Items) {
					return s.replyCmd(id, "evaluate", map[string]any{
						"result":             lo.Items[idx].String(),
						"type":               lo.Items[idx].KindName(),
						"variablesReference": 0,
					})
				}
			}
		}
	}

	return s.replyCmd(id, "evaluate", map[string]any{
		"result":             fmt.Sprintf("undefined: %s", expr),
		"variablesReference": 0,
	})
}

func (s *session) signal(a contAction) {
	select {
	case s.cont <- a:
	default:
		// drop if VM not waiting
		select {
		case <-s.cont:
		default:
		}
		s.cont <- a
	}
}

func (s *session) shutdownVM() {
	s.signal(contDisconnect)
	// don't block forever if program already done
	select {
	case <-s.done:
	default:
	}
}

func (s *session) nextSeq() int {
	return int(atomic.AddInt64(&s.seq, 1))
}

func (s *session) replyCmd(id json.RawMessage, command string, body any) error {
	msg := map[string]any{
		"jsonrpc":     "2.0",
		"type":        "response",
		"request_seq": jsonSeq(id),
		"success":     true,
		"command":     command,
		"seq":         s.nextSeq(),
		"body":        body,
	}
	if len(id) > 0 && string(id) != "null" {
		var raw any
		_ = json.Unmarshal(id, &raw)
		msg["id"] = raw
	}
	return writeMessage(s.w, msg)
}

func (s *session) replyError(id json.RawMessage, code int, message string) error {
	msg := map[string]any{
		"jsonrpc":     "2.0",
		"type":        "response",
		"request_seq": jsonSeq(id),
		"success":     false,
		"command":     "error",
		"seq":         s.nextSeq(),
		"message":     message,
		"body": map[string]any{
			"error": map[string]any{
				"id":       code,
				"format":   message,
				"showUser": true,
			},
		},
	}
	if len(id) > 0 && string(id) != "null" {
		var raw any
		_ = json.Unmarshal(id, &raw)
		msg["id"] = raw
	}
	return writeMessage(s.w, msg)
}

func (s *session) event(event string, body any) error {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"type":    "event",
		"seq":     s.nextSeq(),
		"event":   event,
		"body":    body,
	}
	return writeMessage(s.w, msg)
}

func jsonSeq(id json.RawMessage) int {
	if len(id) == 0 {
		return 0
	}
	var n int
	if err := json.Unmarshal(id, &n); err == nil {
		return n
	}
	// string id
	var s string
	if err := json.Unmarshal(id, &s); err == nil {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return 0
}

// readMessage and writeMessage delegate to the shared jsonrpc framing
// package, which enforces bounded header lines, header count, total
// header bytes, duplicate Content-Length rejection, and body size limits.

func readMessage(r *bufio.Reader) ([]byte, error) { return jsonrpc.ReadMessage(r) }
func writeMessage(w io.Writer, v any) error       { return jsonrpc.WriteMessage(w, v) }
