package runtime

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// CallFunc invokes a Weft function or builtin from host code (agents/tools).
type CallFunc func(fn Value, args []Value) (Value, error)

// ToolCall is an OpenAI-style tool invocation from the model.
type ToolCall struct {
	ID       string
	Name     string
	ArgsJSON string
}

// Env is the global environment for a Weft program.
type Env struct {
	Globals map[string]Value
	Types   map[string]*TypeInfo
	Stdout  io.Writer
	Stderr  io.Writer

	// HostShared names are inherited by path/package modules (prelude + stdlib).
	// User/script bindings are not shared — only symbols marked via SetShared
	// or MarkShared (registration time), never a hardcoded name list.
	HostShared map[string]bool

	// Host hooks (stdlib / embed)
	HTTPClient *http.Client
	Environ    map[string]string // if non-nil, overrides os.Getenv
	LLMBaseURL string
	Call       CallFunc
	// LLMDo short-circuits HTTP for chat completions (tests / custom providers).
	LLMDo func(reqBody []byte) (content string, calls []ToolCall, err error)

	// Ctx is the active execution context (cancel aborts I/O that respects it).
	// Nil means context.Background().
	Ctx context.Context
	// BrowserAsync is true when a js/wasm host has yielded back to the browser
	// event loop. Browser-backed async stdlib operations reject synchronous
	// calls because waiting for a JS Promise would deadlock the main thread.
	BrowserAsync bool
	// BrowserWasm enables direct deadline checks needed by the single-threaded
	// browser VM. Native execution relies on context cancellation notifications.
	BrowserWasm bool

	// Modules
	ModuleCache map[string]Value // abs path -> module map
	ImportStack []string         // cycle detection
	ProjectDir  string           // project root (weft.json / vendor)
	// PackageRoot, when non-empty, confines relative path imports to this tree
	// (installed / named packages). Empty for app scripts (path imports free).
	PackageRoot string

	// Coverage tracking (weft test --coverage)
	// When non-nil, VM records "file:func" keys on each call.
	Coverage map[string]bool

	// Race detection (weft test --race)
	RaceDetect bool
	RaceLog    []string // "file:line: concurrent write to X"
}

// Context returns env.Ctx or background.
func (e *Env) Context() context.Context {
	if e == nil || e.Ctx == nil {
		return context.Background()
	}
	return e.Ctx
}

// ContextErr also observes an expired deadline directly. In js/wasm a tight
// CPU loop can prevent the event loop from delivering the timer notification
// that normally updates context.WithTimeout, so checking the deadline itself
// is required for bounded execution there.
func (e *Env) ContextErr() error {
	ctx := e.Context()
	if err := ctx.Err(); err != nil {
		return err
	}
	if e != nil && e.BrowserWasm {
		if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
			return context.DeadlineExceeded
		}
	}
	return nil
}

// NewEnv creates an environment with prelude builtins only.
// Call stdlib.Register (from embed/CLI) to install packages.
func NewEnv() *Env {
	e := &Env{
		Globals:    make(map[string]Value),
		HostShared: make(map[string]bool),
		Types: map[string]*TypeInfo{
			"str": TypeStr, "int": TypeInt, "float": TypeFloat,
			"bool": TypeBool, "unit": TypeUnit, "any": TypeAny,
		},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	e.RegisterPrelude()
	return e
}

// RegisterPrelude installs intrinsics.
func (e *Env) RegisterPrelude() {
	printlnFn := MakeBuiltin("println", -1, func(args []Value) (Value, error) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = a.String()
		}
		fmt.Fprintln(e.Stdout, strings.Join(parts, " "))
		return Unit(), nil
	})
	e.Globals["println"] = printlnFn
	e.Globals["say"] = printlnFn
	e.Globals["print"] = MakeBuiltin("print", -1, func(args []Value) (Value, error) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = a.String()
		}
		fmt.Fprint(e.Stdout, strings.Join(parts, " "))
		return Unit(), nil
	})
	e.Globals["Ok"] = MakeBuiltin("Ok", 1, func(args []Value) (Value, error) {
		if len(args) != 1 {
			return Null(), fmt.Errorf("Ok expects 1 arg")
		}
		return Ok(args[0]), nil
	})
	e.Globals["Err"] = MakeBuiltin("Err", -1, func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Null(), fmt.Errorf("Err(error|message, kind?)")
		}
		// Err("msg") / Err("msg", "kind") / Err(Error)
		if args[0].Kind == KindStr {
			kind := "user"
			if len(args) >= 2 && args[1].String() != "" {
				kind = args[1].String()
			}
			return Err(NewError(args[0].S, kind)), nil
		}
		if IsError(args[0]) {
			return Err(args[0]), nil
		}
		// wrap anything else as message
		return Err(NewError(args[0].String(), "user")), nil
	})

	// Error constructors
	errPkg := NewMap()
	mo := errPkg.Obj.(*MapObj)
	mo.Keys = []string{"new", "wrap", "with"}
	// Error.new(message, kind?)
	mo.Vals["new"] = MakeBuiltin("Error.new", -1, func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Null(), fmt.Errorf("Error.new(message, kind?)")
		}
		kind := "user"
		if len(args) >= 2 && args[1].String() != "" {
			kind = args[1].String()
		}
		return NewError(args[0].String(), kind), nil
	})
	// Error.wrap(cause, context_msg) -> Error
	mo.Vals["wrap"] = MakeBuiltin("Error.wrap", 2, func(args []Value) (Value, error) {
		if len(args) < 2 {
			return Null(), fmt.Errorf("Error.wrap(cause, message)")
		}
		return WrapError(args[0], args[1].String()), nil
	})
	// Error.with(message, opts_map?) — kind/code/cause from map
	mo.Vals["with"] = MakeBuiltin("Error.with", -1, func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Null(), fmt.Errorf("Error.with(message, {kind, code, cause}?)")
		}
		msg := args[0].String()
		kind := "user"
		var code, cause Value = Null(), Null()
		if len(args) >= 2 && args[1].Kind == KindMap {
			mo := args[1].Obj.(*MapObj)
			if v, ok := mo.Vals["kind"]; ok {
				kind = v.String()
			}
			if v, ok := mo.Vals["code"]; ok {
				code = v
			}
			if v, ok := mo.Vals["cause"]; ok {
				cause = v
			}
			if v, ok := mo.Vals["message"]; ok && msg == "" {
				msg = v.String()
			}
		}
		e := NewErrorCode(msg, kind, code)
		if cause.Kind != KindNull {
			so := e.Obj.(*StructObj)
			so.Fields["cause"] = cause
		}
		return e, nil
	})
	e.Globals["Error"] = errPkg

	// ensure(cond, msg?) -> Result  — assert precondition; use with ?
	e.Globals["ensure"] = MakeBuiltin("ensure", -1, func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Err(NewError("ensure(cond, msg?)", "ensure")), nil
		}
		if args[0].IsTruthy() {
			return Ok(Unit()), nil
		}
		msg := "ensure failed"
		if len(args) >= 2 && args[1].String() != "" {
			msg = args[1].String()
		}
		return Err(NewError(msg, "ensure")), nil
	})
	// bail(msg, kind?) -> Result Err  — early failure without ceremony
	e.Globals["bail"] = MakeBuiltin("bail", -1, func(args []Value) (Value, error) {
		msg := "bail"
		kind := "user"
		if len(args) >= 1 && args[0].String() != "" {
			msg = args[0].String()
		}
		if len(args) >= 2 && args[1].String() != "" {
			kind = args[1].String()
		}
		return Err(NewError(msg, kind)), nil
	})
	// is_err(v) / is_ok(v) — work on Result or Error
	e.Globals["is_ok"] = MakeBuiltin("is_ok", 1, func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Bool(false), nil
		}
		v := args[0]
		if v.Kind == KindResult {
			return Bool(v.Obj.(*ResultObj).Ok), nil
		}
		return Bool(false), nil
	})
	e.Globals["is_err"] = MakeBuiltin("is_err", 1, func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Bool(false), nil
		}
		v := args[0]
		if v.Kind == KindResult {
			return Bool(!v.Obj.(*ResultObj).Ok), nil
		}
		return Bool(IsError(v)), nil
	})

	// int.parse
	intPkg := NewMap()
	imo := intPkg.Obj.(*MapObj)
	imo.Keys = []string{"parse"}
	imo.Vals["parse"] = MakeBuiltin("int.parse", 1, func(args []Value) (Value, error) {
		if len(args) != 1 {
			return Null(), fmt.Errorf("int.parse expects 1 arg")
		}
		n, err := AsInt(args[0])
		if err != nil {
			return Err(NewError(err.Error(), "validation")), nil
		}
		return Ok(Int(n)), nil
	})
	e.Globals["int"] = intPkg

	// len(x) — list, map, str, or bytes-as-str length
	e.Globals["len"] = MakeBuiltin("len", 1, func(args []Value) (Value, error) {
		if len(args) != 1 {
			return Null(), fmt.Errorf("len expects 1 arg")
		}
		switch args[0].Kind {
		case KindList:
			return Int(int64(len(args[0].Obj.(*ListObj).Items))), nil
		case KindMap:
			return Int(int64(len(args[0].Obj.(*MapObj).Vals))), nil
		case KindStr:
			return Int(int64(len([]rune(args[0].S)))), nil
		case KindNull:
			return Int(0), nil
		default:
			return Null(), fmt.Errorf("len: unsupported %s", args[0].KindName())
		}
	})

	// push(list, item) — mutate list in place, return list (data pipelines)
	e.Globals["push"] = MakeBuiltin("push", 2, func(args []Value) (Value, error) {
		if len(args) < 2 || args[0].Kind != KindList {
			return Null(), fmt.Errorf("push(list, item)")
		}
		lo := args[0].Obj.(*ListObj)
		lo.Items = append(lo.Items, args[1])
		return args[0], nil
	})
	// concat(a, b) -> new list
	e.Globals["concat"] = MakeBuiltin("concat", 2, func(args []Value) (Value, error) {
		if len(args) < 2 || args[0].Kind != KindList || args[1].Kind != KindList {
			return Null(), fmt.Errorf("concat(list, list)")
		}
		a := args[0].Obj.(*ListObj).Items
		b := args[1].Obj.(*ListObj).Items
		out := make([]Value, 0, len(a)+len(b))
		out = append(out, a...)
		out = append(out, b...)
		return List(out...), nil
	})
	// slice(list, start, end?) — end exclusive; negative from end
	e.Globals["slice"] = MakeBuiltin("slice", -1, func(args []Value) (Value, error) {
		if len(args) < 2 || args[0].Kind != KindList {
			return Null(), fmt.Errorf("slice(list, start, end?)")
		}
		items := args[0].Obj.(*ListObj).Items
		n := len(items)
		start, err := AsInt(args[1])
		if err != nil {
			return Null(), err
		}
		end := int64(n)
		if len(args) >= 3 {
			end, err = AsInt(args[2])
			if err != nil {
				return Null(), err
			}
		}
		si, ei := int(start), int(end)
		if si < 0 {
			si = n + si
		}
		if ei < 0 {
			ei = n + ei
		}
		if si < 0 {
			si = 0
		}
		if ei > n {
			ei = n
		}
		if si > ei {
			si = ei
		}
		return List(items[si:ei]...), nil
	})
	// range(n) or range(start, end) — list of ints [start, end)
	e.Globals["range"] = MakeBuiltin("range", -1, func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Null(), fmt.Errorf("range(n) or range(start, end)")
		}
		var start, end int64
		if len(args) == 1 {
			start = 0
			var err error
			end, err = AsInt(args[0])
			if err != nil {
				return Null(), err
			}
		} else {
			var err error
			start, err = AsInt(args[0])
			if err != nil {
				return Null(), err
			}
			end, err = AsInt(args[1])
			if err != nil {
				return Null(), err
			}
		}
		if end < start {
			return List(), nil
		}
		const maxRange = 10_000_000
		if end-start > maxRange {
			return Null(), fmt.Errorf("range too large (%d elements, max %d)", end-start, maxRange)
		}
		items := make([]Value, 0, end-start)
		for i := start; i < end; i++ {
			items = append(items, Int(i))
		}
		return List(items...), nil
	})
	// pop(list) -> last element (mutates list)
	e.Globals["pop"] = MakeBuiltin("pop", 1, func(args []Value) (Value, error) {
		if len(args) < 1 || args[0].Kind != KindList {
			return Null(), fmt.Errorf("pop(list)")
		}
		lo := args[0].Obj.(*ListObj)
		if len(lo.Items) == 0 {
			return Null(), nil
		}
		v := lo.Items[len(lo.Items)-1]
		lo.Items = lo.Items[:len(lo.Items)-1]
		return v, nil
	})
	// contains(list|str, item) -> bool
	e.Globals["contains"] = MakeBuiltin("contains", 2, func(args []Value) (Value, error) {
		if len(args) < 2 {
			return Bool(false), nil
		}
		switch args[0].Kind {
		case KindStr:
			return Bool(strings.Contains(args[0].S, args[1].String())), nil
		case KindList:
			for _, it := range args[0].Obj.(*ListObj).Items {
				if Equal(it, args[1]) {
					return Bool(true), nil
				}
			}
			return Bool(false), nil
		default:
			return Bool(false), nil
		}
	})
	// keys(map) -> list
	e.Globals["keys"] = MakeBuiltin("keys", 1, func(args []Value) (Value, error) {
		if len(args) < 1 || args[0].Kind != KindMap {
			return List(), nil
		}
		mo := args[0].Obj.(*MapObj)
		items := make([]Value, len(mo.Keys))
		for i, k := range mo.Keys {
			items[i] = Str(k)
		}
		return List(items...), nil
	})
	// values(map) -> list
	e.Globals["values"] = MakeBuiltin("values", 1, func(args []Value) (Value, error) {
		if len(args) < 1 || args[0].Kind != KindMap {
			return List(), nil
		}
		mo := args[0].Obj.(*MapObj)
		items := make([]Value, 0, len(mo.Keys))
		for _, k := range mo.Keys {
			items = append(items, mo.Vals[k])
		}
		return List(items...), nil
	})
	// delete(map, key) -> map (mutates)
	e.Globals["delete"] = MakeBuiltin("delete", 2, func(args []Value) (Value, error) {
		if len(args) < 2 || args[0].Kind != KindMap {
			return Null(), fmt.Errorf("delete(map, key)")
		}
		mo := args[0].Obj.(*MapObj)
		k := args[1].String()
		delete(mo.Vals, k)
		nk := mo.Keys[:0]
		for _, x := range mo.Keys {
			if x != k {
				nk = append(nk, x)
			}
		}
		mo.Keys = nk
		return args[0], nil
	})

	e.Globals["unit"] = Struct("unit", map[string]Value{}, nil)
	e.Types["Error"] = &TypeInfo{
		Name: "Error",
		Kind: "struct",
		Fields: []FieldInfo{
			{Name: "message", Type: TypeStr},
			{Name: "kind", Type: TypeStr},
			{Name: "code", Type: &TypeInfo{Name: "str?", Kind: "optional", Elem: TypeStr}, Optional: true},
			{Name: "cause", Type: &TypeInfo{Name: "Error?", Kind: "optional"}, Optional: true},
		},
	}

	// Concurrency (non-verbose)
	// spawn(fn) or spawn(fn, arg1, arg2, ...) — args deep-copied (no shared mut)
	e.Globals["spawn"] = MakeBuiltin("spawn", -1, func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Null(), fmt.Errorf("spawn(fn, args...)")
		}
		var callArgs []Value
		if len(args) > 1 {
			callArgs = args[1:]
		}
		return spawnTask(e, args[0], callArgs)
	})
	e.Globals["parallel"] = MakeBuiltin("parallel", 1, func(args []Value) (Value, error) {
		if len(args) < 1 || args[0].Kind != KindList {
			return Err(NewError("parallel([fn, ...])", "task")), nil
		}
		return parallelTasks(e, args[0].Obj.(*ListObj).Items)
	})
	// gather = parallel (concurrent fan-out; not asyncio — listops already uses `all` for any/all)
	e.Globals["gather"] = e.Globals["parallel"]
	// race([fn, ...]) — first completed wins
	e.Globals["race"] = MakeBuiltin("race", 1, func(args []Value) (Value, error) {
		if len(args) < 1 || args[0].Kind != KindList {
			return Err(NewError("race([fn, ...])", "task")), nil
		}
		return raceTasks(e, args[0].Obj.(*ListObj).Items)
	})
	// timeout(seconds, fn) -> Result  (concurrent deadline; no async/await)
	e.Globals["timeout"] = MakeBuiltin("timeout", 2, func(args []Value) (Value, error) {
		if len(args) < 2 {
			return Err(NewError("timeout(seconds, fn)", "task")), nil
		}
		var d time.Duration
		switch args[0].Kind {
		case KindInt:
			d = time.Duration(args[0].I) * time.Second
		case KindFloat:
			d = time.Duration(args[0].F * float64(time.Second))
		default:
			if n, err := AsInt(args[0]); err == nil {
				d = time.Duration(n) * time.Second
			}
		}
		return timeoutTask(e, d, args[1])
	})
	// channel(buf?) / send / recv / close / select_recv
	e.Globals["channel"] = MakeBuiltin("channel", -1, func(args []Value) (Value, error) {
		buf := 0
		if len(args) > 0 {
			n, err := AsInt(args[0])
			if err != nil {
				return Null(), err
			}
			buf = int(n)
		}
		return MakeChannel(buf), nil
	})
	e.Globals["send"] = MakeBuiltin("send", 2, func(args []Value) (Value, error) {
		if len(args) < 2 {
			return Err(NewError("send(ch, value)", "chan")), nil
		}
		if err := ChannelSend(args[0], args[1]); err != nil {
			return Err(NewError(err.Error(), "chan")), nil
		}
		return Ok(Unit()), nil
	})
	e.Globals["recv"] = MakeBuiltin("recv", 1, func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Err(NewError("recv(ch)", "chan")), nil
		}
		v, err := ChannelRecv(args[0])
		if err != nil {
			return Err(NewError(err.Error(), "chan")), nil
		}
		return Ok(v), nil
	})
	// try_recv(ch) -> Result[{ok, value}]  non-blocking; ok=false if empty/closed empty
	e.Globals["try_recv"] = MakeBuiltin("try_recv", 1, func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Err(NewError("try_recv(ch)", "chan")), nil
		}
		v, ok, err := ChannelTryRecv(args[0])
		if err != nil {
			return Err(NewError(err.Error(), "chan")), nil
		}
		m := NewMap()
		mo := m.Obj.(*MapObj)
		mo.Keys = []string{"ok", "value"}
		mo.Vals["ok"] = Bool(ok)
		mo.Vals["value"] = v
		return Ok(m), nil
	})
	e.Globals["close"] = MakeBuiltin("close", 1, func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Err(NewError("close(ch)", "chan")), nil
		}
		if err := ChannelClose(args[0]); err != nil {
			return Err(NewError(err.Error(), "chan")), nil
		}
		return Ok(Unit()), nil
	})
	// select_recv([ch, ...], timeout_ms?) -> Result[{i, value}]
	e.Globals["select_recv"] = MakeBuiltin("select_recv", -1, func(args []Value) (Value, error) {
		if len(args) < 1 || args[0].Kind != KindList {
			return Err(NewError("select_recv([ch, ...], timeout_ms?)", "chan")), nil
		}
		timeout := int64(0)
		if len(args) > 1 {
			n, _ := AsInt(args[1])
			timeout = n
		}
		chs := args[0].Obj.(*ListObj).Items
		i, v, err := SelectRecv(chs, timeout)
		if err != nil {
			return Err(NewError(err.Error(), "chan")), nil
		}
		m := NewMap()
		mo := m.Obj.(*MapObj)
		mo.Keys = []string{"i", "value"}
		mo.Vals["i"] = Int(int64(i))
		mo.Vals["value"] = v
		return Ok(m), nil
	})
	// group() -> TaskGroup with .go(fn) and .wait()
	e.Globals["group"] = MakeBuiltin("group", 0, func(args []Value) (Value, error) {
		return newTaskGroup(e), nil
	})

	// deepcopy(v) — structural copy (lists/maps/structs); for spawn-safe data
	e.Globals["deepcopy"] = MakeBuiltin("deepcopy", 1, func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Null(), nil
		}
		return DeepCopy(args[0]), nil
	})

	// All prelude symbols are host-shared with modules (no name list to maintain).
	e.MarkAllGlobalsShared()
}

// Get looks up a global.
func (e *Env) Get(name string) (Value, bool) {
	v, ok := e.Globals[name]
	return v, ok
}

// Set sets a global (script/user binding — not auto-shared into modules).
func (e *Env) Set(name string, v Value) {
	e.Globals[name] = v
}

// SetShared sets a host symbol inherited by every loaded module.
func (e *Env) SetShared(name string, v Value) {
	e.Globals[name] = v
	e.MarkShared(name)
}

// MarkShared flags existing (or future) global names as host-shared for modules.
func (e *Env) MarkShared(names ...string) {
	if e.HostShared == nil {
		e.HostShared = make(map[string]bool)
	}
	for _, n := range names {
		if n != "" {
			e.HostShared[n] = true
		}
	}
}

// MarkAllGlobalsShared marks every current global as host-shared.
func (e *Env) MarkAllGlobalsShared() {
	if e.HostShared == nil {
		e.HostShared = make(map[string]bool)
	}
	for k := range e.Globals {
		e.HostShared[k] = true
	}
}

// IsHostShared reports whether name is inherited by modules.
func (e *Env) IsHostShared(name string) bool {
	return e != nil && e.HostShared != nil && e.HostShared[name]
}

// CopyHostSharedInto copies prelude/stdlib symbols into dst (module env).
func (e *Env) CopyHostSharedInto(dst *Env) {
	if e == nil || dst == nil {
		return
	}
	for name := range e.HostShared {
		if v, ok := e.Globals[name]; ok {
			dst.Globals[name] = v
			if dst.HostShared == nil {
				dst.HostShared = make(map[string]bool)
			}
			dst.HostShared[name] = true
		}
	}
}

// RevokeHost removes a host-shared package/symbol from this env (capabilities).
func (e *Env) RevokeHost(name string) {
	if e == nil {
		return
	}
	delete(e.Globals, name)
	if e.HostShared != nil {
		delete(e.HostShared, name)
	}
}
