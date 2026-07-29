// Package vm implements the Weft stack bytecode virtual machine.
package vm

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/loreste/weft/internal/compile"
	"github.com/loreste/weft/internal/runtime"
)

// VM executes Weft bytecode.
type VM struct {
	Env         *runtime.Env
	stack       []runtime.Value
	frames      []frame
	calledFrame bool // last OpCall installed a new frame
	// fatal is set on stack underflow / bad operands; run() returns it instead of panicking.
	fatal error
	// Debug hooks (nil = no debugging)
	Debug *DebugState
}

// DebugState holds debugger state for step/breakpoint execution.
type DebugState struct {
	Breakpoints map[string]bool // "file:line" → true
	StepMode    bool            // pause after every statement
	Paused      bool
	// SkipBP is a "file:line" key ignored once after continue (avoids re-hit).
	SkipBP string
	// LastStack is filled before OnPause (deepest frame first).
	LastStack []FrameLoc
	OnPause   func(loc FrameLoc, locals map[string]runtime.Value) // callback when paused
}

type deferredCall struct {
	callee runtime.Value
	args   []runtime.Value
}

type frame struct {
	fn     *runtime.FuncObj
	chunk  *compile.Chunk
	ip     int
	slots  []runtime.Value
	bp     int
	defers []deferredCall // LIFO on exit
}

// New creates a VM.
func New(env *runtime.Env) *VM {
	return &VM{Env: env, stack: make([]runtime.Value, 0, 256)}
}

// RunFunc runs a compiled function with args.
func (vm *VM) RunFunc(fn *runtime.FuncObj, args []runtime.Value) (runtime.Value, error) {
	chunk, ok := fn.Chunk.(*compile.Chunk)
	if !ok || chunk == nil {
		return runtime.Null(), fmt.Errorf("invalid function chunk")
	}
	if err := arityErr(fn, args); err != nil {
		return runtime.Null(), err
	}
	slots := make([]runtime.Value, chunk.NumLocs)
	for i := 0; i < fn.Arity && i < len(args); i++ {
		slots[i] = args[i]
	}
	for i, uv := range fn.Upvalues {
		slot := fn.Arity + i
		if slot < len(slots) {
			slots[slot] = uv
		}
	}
	vm.frames = append(vm.frames, frame{
		fn: fn, chunk: chunk, ip: 0, slots: slots, bp: len(vm.stack),
	})
	return vm.run()
}

func (vm *VM) push(v runtime.Value) { vm.stack = append(vm.stack, v) }

func (vm *VM) pop() runtime.Value {
	n := len(vm.stack)
	if n == 0 {
		if vm.fatal == nil {
			vm.fatal = vm.errf("stack underflow")
		}
		return runtime.Null()
	}
	v := vm.stack[n-1]
	vm.stack = vm.stack[:n-1]
	return v
}

func (vm *VM) peek() runtime.Value {
	n := len(vm.stack)
	if n == 0 {
		if vm.fatal == nil {
			vm.fatal = vm.errf("stack underflow")
		}
		return runtime.Null()
	}
	return vm.stack[n-1]
}

func (vm *VM) readU16(fr *frame) uint16 {
	ch := fr.chunk
	if fr.ip+1 >= len(ch.Code) {
		if vm.fatal == nil {
			vm.fatal = vm.errf("truncated operand at ip %d", fr.ip)
		}
		// advance past end so the run loop exits cleanly
		fr.ip = len(ch.Code)
		return 0
	}
	hi := ch.Code[fr.ip]
	lo := ch.Code[fr.ip+1]
	fr.ip += 2
	return uint16(hi)<<8 | uint16(lo)
}

func (vm *VM) run() (runtime.Value, error) {
	for len(vm.frames) > 0 {
		if vm.fatal != nil {
			return runtime.Null(), vm.wrapErr(vm.fatal)
		}
		fr := &vm.frames[len(vm.frames)-1]
		ch := fr.chunk
		if fr.ip >= len(ch.Code) {
			if err := vm.runDefers(fr); err != nil {
				return runtime.Null(), vm.wrapErr(err)
			}
			vm.frames = vm.frames[:len(vm.frames)-1]
			if len(vm.frames) == 0 {
				return runtime.Unit(), nil
			}
			vm.push(runtime.Unit())
			continue
		}
		// Debug hook: check breakpoints / step mode before each instruction
		if vm.Debug != nil {
			vm.debugCheck(fr)
		}
		op := compile.Op(ch.Code[fr.ip])
		fr.ip++

		switch op {
		case compile.OpLoadConst:
			idx := vm.readU16(fr)
			if vm.fatal != nil {
				return runtime.Null(), vm.wrapErr(vm.fatal)
			}
			if int(idx) >= len(ch.Consts) {
				return runtime.Null(), vm.errf("const index %d out of range", idx)
			}
			v, ok := ch.Consts[idx].(runtime.Value)
			if !ok {
				return runtime.Null(), vm.errf("const %d is not a Value", idx)
			}
			vm.push(v)
		case compile.OpLoadLocal:
			idx := vm.readU16(fr)
			if vm.fatal != nil {
				return runtime.Null(), vm.wrapErr(vm.fatal)
			}
			if int(idx) >= len(fr.slots) {
				return runtime.Null(), vm.errf("local index %d out of range", idx)
			}
			vm.push(fr.slots[idx])
		case compile.OpStoreLocal:
			idx := vm.readU16(fr)
			if vm.fatal != nil {
				return runtime.Null(), vm.wrapErr(vm.fatal)
			}
			if int(idx) >= len(fr.slots) {
				return runtime.Null(), vm.errf("local index %d out of range", idx)
			}
			fr.slots[idx] = vm.pop()
		case compile.OpLoadGlobal:
			name := vm.pop().S
			if v, ok := vm.Env.Get(name); ok {
				vm.push(v)
			} else if ti, ok := vm.Env.Types[name]; ok {
				vm.push(runtime.TypeInfoValue(ti))
			} else {
				return runtime.Null(), vm.errf("undefined name %q", name)
			}
		case compile.OpStoreGlobal:
			name := vm.pop().S
			vm.Env.Set(name, vm.pop())
		case compile.OpPop:
			vm.pop()
		case compile.OpTrue:
			vm.push(runtime.Bool(true))
		case compile.OpFalse:
			vm.push(runtime.Bool(false))
		case compile.OpNull:
			vm.push(runtime.Null())
		case compile.OpUnit:
			vm.push(runtime.Unit())
		case compile.OpAdd:
			b, a := vm.pop(), vm.pop()
			r, err := binAdd(a, b)
			if err != nil {
				return runtime.Null(), vm.wrapErr(err)
			}
			vm.push(r)
		case compile.OpSub:
			b, a := vm.pop(), vm.pop()
			r, err := binNum(a, b, func(x, y float64) float64 { return x - y })
			if err != nil {
				return runtime.Null(), vm.wrapErr(err)
			}
			vm.push(r)
		case compile.OpMul:
			b, a := vm.pop(), vm.pop()
			r, err := binNum(a, b, func(x, y float64) float64 { return x * y })
			if err != nil {
				return runtime.Null(), vm.wrapErr(err)
			}
			vm.push(r)
		case compile.OpDiv:
			b, a := vm.pop(), vm.pop()
			r, err := div(a, b)
			if err != nil {
				return runtime.Null(), vm.wrapErr(err)
			}
			vm.push(r)
		case compile.OpMod:
			b, a := vm.pop(), vm.pop()
			if a.Kind != runtime.KindInt || b.Kind != runtime.KindInt {
				return runtime.Null(), vm.errf("%% requires ints")
			}
			if b.I == 0 {
				return runtime.Null(), vm.errf("division by zero")
			}
			vm.push(runtime.Int(a.I % b.I))
		case compile.OpNeg:
			a := vm.pop()
			switch a.Kind {
			case runtime.KindInt:
				vm.push(runtime.Int(-a.I))
			case runtime.KindFloat:
				vm.push(runtime.Float(-a.F))
			default:
				return runtime.Null(), vm.errf("unary - on %s", a.KindName())
			}
		case compile.OpNot:
			vm.push(runtime.Bool(!vm.pop().IsTruthy()))
		case compile.OpEq:
			b, a := vm.pop(), vm.pop()
			vm.push(runtime.Bool(runtime.Equal(a, b)))
		case compile.OpNeq:
			b, a := vm.pop(), vm.pop()
			vm.push(runtime.Bool(!runtime.Equal(a, b)))
		case compile.OpLt, compile.OpLte, compile.OpGt, compile.OpGte:
			b, a := vm.pop(), vm.pop()
			cmp, err := compare(a, b)
			if err != nil {
				return runtime.Null(), vm.wrapErr(err)
			}
			switch op {
			case compile.OpLt:
				vm.push(runtime.Bool(cmp < 0))
			case compile.OpLte:
				vm.push(runtime.Bool(cmp <= 0))
			case compile.OpGt:
				vm.push(runtime.Bool(cmp > 0))
			case compile.OpGte:
				vm.push(runtime.Bool(cmp >= 0))
			}
		case compile.OpNullCoalesce:
			b, a := vm.pop(), vm.pop()
			if a.Kind == runtime.KindNull {
				vm.push(b)
			} else {
				vm.push(a)
			}
		case compile.OpJump:
			off := int(int16(vm.readU16(fr)))
			fr.ip += off
		case compile.OpJumpIfFalse:
			// peek — leave value for short-circuit &&
			off := int(int16(vm.readU16(fr)))
			if !vm.peek().IsTruthy() {
				fr.ip += off
			}
		case compile.OpJumpIfTrue:
			off := int(int16(vm.readU16(fr)))
			if vm.peek().IsTruthy() {
				fr.ip += off
			}
		case compile.OpJumpIfFalsePop:
			off := int(int16(vm.readU16(fr)))
			v := vm.pop()
			if !v.IsTruthy() {
				fr.ip += off
			}
		case compile.OpJumpIfTruePop:
			off := int(int16(vm.readU16(fr)))
			v := vm.pop()
			if v.IsTruthy() {
				fr.ip += off
			}
		case compile.OpCall:
			argc := int(vm.readU16(fr))
			args := make([]runtime.Value, argc)
			for i := argc - 1; i >= 0; i-- {
				args[i] = vm.pop()
			}
			callee := vm.pop()
			vm.calledFrame = false
			ret, err := vm.call(callee, args)
			if err != nil {
				return runtime.Null(), vm.wrapErr(err)
			}
			if !vm.calledFrame {
				vm.push(ret)
			}
		case compile.OpDefer:
			argc := int(vm.readU16(fr))
			args := make([]runtime.Value, argc)
			for i := argc - 1; i >= 0; i-- {
				args[i] = vm.pop()
			}
			callee := vm.pop()
			fr.defers = append(fr.defers, deferredCall{callee: callee, args: args})
		case compile.OpReturn:
			var ret runtime.Value
			if len(vm.stack) > fr.bp {
				ret = vm.pop()
			} else {
				ret = runtime.Unit()
			}
			if err := vm.runDefers(fr); err != nil {
				return runtime.Null(), vm.wrapErr(err)
			}
			vm.stack = vm.stack[:fr.bp]
			vm.frames = vm.frames[:len(vm.frames)-1]
			if len(vm.frames) == 0 {
				return ret, nil
			}
			vm.push(ret)
		case compile.OpMakeList:
			n := int(vm.readU16(fr))
			items := make([]runtime.Value, n)
			for i := n - 1; i >= 0; i-- {
				items[i] = vm.pop()
			}
			vm.push(runtime.List(items...))
		case compile.OpMakeMap:
			n := int(vm.readU16(fr))
			m := runtime.NewMap()
			mo := m.Obj.(*runtime.MapObj)
			pairs := make([]runtime.Value, n*2)
			for i := n*2 - 1; i >= 0; i-- {
				pairs[i] = vm.pop()
			}
			for i := 0; i < n; i++ {
				ks := pairs[i*2].String()
				v := pairs[i*2+1]
				if _, exists := mo.Vals[ks]; !exists {
					mo.Keys = append(mo.Keys, ks)
				}
				mo.Vals[ks] = v
			}
			vm.push(m)
		case compile.OpMakeStruct:
			n := int(vm.readU16(fr))
			name := vm.pop().S
			tmp := make([]runtime.Value, n*2)
			for i := n*2 - 1; i >= 0; i-- {
				tmp[i] = vm.pop()
			}
			fields := make(map[string]runtime.Value, n)
			order := make([]string, 0, n)
			for i := 0; i < n; i++ {
				fn := tmp[i*2].S
				fields[fn] = tmp[i*2+1]
				order = append(order, fn)
			}
			vm.push(runtime.Struct(name, fields, order))
		case compile.OpGetIndex:
			idx, x := vm.pop(), vm.pop()
			v, err := getIndex(x, idx)
			if err != nil {
				return runtime.Null(), vm.wrapErr(err)
			}
			vm.push(v)
		case compile.OpSetIndex:
			val, idx, x := vm.pop(), vm.pop(), vm.pop()
			if err := setIndex(x, idx, val); err != nil {
				return runtime.Null(), vm.wrapErr(err)
			}
		case compile.OpGetField:
			name := vm.pop().S
			x := vm.pop()
			v, err := getField(x, name)
			if err != nil {
				return runtime.Null(), vm.wrapErr(err)
			}
			vm.push(v)
		case compile.OpSetField:
			name := vm.pop().S
			val := vm.pop()
			x := vm.pop()
			if err := setField(x, name, val); err != nil {
				return runtime.Null(), vm.wrapErr(err)
			}
		case compile.OpTryQ:
			v := vm.pop()
			if v.Kind != runtime.KindResult {
				return runtime.Null(), vm.errf("? on non-Result (got %s) — use Ok/Err or a fallible call", v.KindName())
			}
			ro := v.Obj.(*runtime.ResultObj)
			if !ro.Ok {
				// Stamp source location onto Error for agent-friendly diagnostics.
				errPayload := ro.Err
				if loc := frameLoc(fr); loc.File != "" && loc.Line > 0 {
					at := fmt.Sprintf("%s:%d", loc.File, loc.Line)
					if loc.Func != "" {
						at = fmt.Sprintf("%s:%d in %s", loc.File, loc.Line, loc.Func)
					}
					errPayload = runtime.ErrorWithLocation(errPayload, at)
				}
				errVal := runtime.Err(errPayload)
				if err := vm.runDefers(fr); err != nil {
					return runtime.Null(), vm.wrapErr(err)
				}
				vm.stack = vm.stack[:fr.bp]
				vm.frames = vm.frames[:len(vm.frames)-1]
				if len(vm.frames) == 0 {
					return errVal, nil
				}
				vm.push(errVal)
				continue
			}
			vm.push(ro.Val)
		case compile.OpWrapResult:
			// Design §1.4: Result unchanged; Error → Err; bare T → Ok(T)
			v := vm.pop()
			switch v.Kind {
			case runtime.KindResult:
				vm.push(v)
			case runtime.KindStruct:
				if so, ok := v.Obj.(*runtime.StructObj); ok && so.TypeName == "Error" {
					vm.push(runtime.Err(v))
					break
				}
				vm.push(runtime.Ok(v))
			default:
				vm.push(runtime.Ok(v))
			}
		case compile.OpConcat:
			b, a := vm.pop(), vm.pop()
			vm.push(runtime.Str(a.String() + b.String()))
		case compile.OpClose:
			// stack: func, uv0, uv1, ... uvn-1  (ups pushed after func)
			n := int(vm.readU16(fr))
			ups := make([]runtime.Value, n)
			for i := n - 1; i >= 0; i-- {
				ups[i] = runtime.DeepCopy(vm.pop())
			}
			f := vm.pop()
			if f.Kind != runtime.KindFunc {
				return runtime.Null(), vm.errf("CLOSE expects function")
			}
			proto := f.Obj.(*runtime.FuncObj)
			clone := *proto
			clone.Upvalues = ups
			vm.push(runtime.Func(&clone))
		case compile.OpIterNew:
			x := vm.pop()
			it, err := newIter(x)
			if err != nil {
				return runtime.Null(), vm.wrapErr(err)
			}
			vm.push(runtime.MakeIter(it))
		case compile.OpIterNext:
			raw := vm.pop()
			it, err := runtime.AsIter(raw)
			if err != nil {
				return runtime.Null(), vm.wrapErr(err)
			}
			v, ok := it.Next()
			if ok {
				vm.push(v)
				vm.push(runtime.Bool(true))
			} else {
				vm.push(runtime.Bool(false))
			}
		default:
			return runtime.Null(), vm.errf("unknown opcode %v at ip %d", op, fr.ip-1)
		}
		if vm.fatal != nil {
			return runtime.Null(), vm.wrapErr(vm.fatal)
		}
	}
	if vm.fatal != nil {
		return runtime.Null(), vm.wrapErr(vm.fatal)
	}
	return runtime.Unit(), nil
}

// runDefers executes deferred calls LIFO (Go-like). Clears the list first so nested
// returns from a defer body do not re-run the same entries.
//
// User functions run in a nested VM so they complete fully without interfering with
// the exiting frame's OpReturn / OpTryQ path (unlike OpCall, which only installs a frame).
func (vm *VM) runDefers(fr *frame) error {
	ds := fr.defers
	fr.defers = nil
	for i := len(ds) - 1; i >= 0; i-- {
		if err := vm.invokeDeferred(ds[i]); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) invokeDeferred(d deferredCall) error {
	switch d.callee.Kind {
	case runtime.KindBuiltin:
		b := d.callee.Obj.(*runtime.BuiltinObj)
		_, err := b.Fn(d.args)
		if err != nil {
			return vm.wrapErr(err)
		}
		return nil
	case runtime.KindFunc:
		fn := d.callee.Obj.(*runtime.FuncObj)
		env := vm.Env
		if fn.Env != nil {
			env = fn.Env
		}
		// Nested VM: full run to completion, shared Env (stdout/globals).
		_, err := New(env).RunFunc(fn, d.args)
		if err != nil {
			return vm.wrapErr(err)
		}
		return nil
	default:
		return vm.errf("defer: cannot call %s", d.callee.KindName())
	}
}

// FrameLoc is one stack frame for traces.
type FrameLoc struct {
	File string
	Func string
	Line int
}

// RuntimeError carries a message and stack frames (deepest first).
type RuntimeError struct {
	Msg   string
	Stack []FrameLoc
}

func (e *RuntimeError) Error() string {
	if e == nil {
		return ""
	}
	if len(e.Stack) == 0 {
		return e.Msg
	}
	// Primary location
	top := e.Stack[0]
	head := e.Msg
	if top.File != "" && top.Line > 0 {
		head = fmt.Sprintf("%s:%d: %s", top.File, top.Line, e.Msg)
	} else if top.Line > 0 {
		head = fmt.Sprintf("line %d: %s", top.Line, e.Msg)
	}
	if len(e.Stack) == 1 {
		return head
	}
	var b strings.Builder
	b.WriteString(head)
	b.WriteString("\nstack:")
	for _, f := range e.Stack {
		b.WriteString("\n  ")
		if f.File != "" {
			b.WriteString(f.File)
			b.WriteByte(':')
		}
		if f.Line > 0 {
			fmt.Fprintf(&b, "%d ", f.Line)
		}
		if f.Func != "" {
			b.WriteString("in ")
			b.WriteString(f.Func)
		}
	}
	return b.String()
}

func frameLoc(fr *frame) FrameLoc {
	loc := FrameLoc{Func: "?"}
	if fr == nil || fr.chunk == nil {
		return loc
	}
	loc.File = fr.chunk.File
	if fr.fn != nil && fr.fn.Name != "" {
		loc.Func = fr.fn.Name
	} else if fr.chunk.Name != "" {
		loc.Func = fr.chunk.Name
	}
	// ip already advanced past opcode; Lines is parallel to Code bytes
	ip := fr.ip - 1
	if ip >= 0 && ip < len(fr.chunk.Lines) {
		loc.Line = fr.chunk.Lines[ip]
	}
	return loc
}

func (vm *VM) stackTrace() []FrameLoc {
	out := make([]FrameLoc, 0, len(vm.frames))
	// deepest (current) first
	for i := len(vm.frames) - 1; i >= 0; i-- {
		out = append(out, frameLoc(&vm.frames[i]))
	}
	return out
}

func (vm *VM) errf(format string, args ...any) error {
	return &RuntimeError{
		Msg:   fmt.Sprintf(format, args...),
		Stack: vm.stackTrace(),
	}
}

func (vm *VM) wrapErr(err error) error {
	if err == nil {
		return nil
	}
	// don't wrap process exits
	if _, isExit := runtime.IsExit(err); isExit {
		return err
	}
	if re, ok := err.(*RuntimeError); ok {
		if len(re.Stack) == 0 {
			re.Stack = vm.stackTrace()
		}
		return re
	}
	return &RuntimeError{
		Msg:   err.Error(),
		Stack: vm.stackTrace(),
	}
}

// maxFrames caps call depth to catch infinite recursion before Go stack overflow.
const maxFrames = 10_000

// arityErr reports under-arity for user functions. Extra args are still ignored
// (matches historical behavior); missing args used to silently become null and
// produce misleading errors like "numeric op on int and null".
func arityErr(fn *runtime.FuncObj, args []runtime.Value) error {
	if fn == nil || fn.Arity < 0 {
		return nil
	}
	if len(args) >= fn.Arity {
		return nil
	}
	name := fn.Name
	if name == "" {
		name = "<fn>"
	}
	return fmt.Errorf("wrong number of arguments to %s: have %d, want %d", name, len(args), fn.Arity)
}

func (vm *VM) call(callee runtime.Value, args []runtime.Value) (runtime.Value, error) {
	if len(vm.frames) >= maxFrames {
		return runtime.Null(), vm.errf("stack overflow: call depth exceeds %d frames", maxFrames)
	}
	switch callee.Kind {
	case runtime.KindBuiltin:
		b := callee.Obj.(*runtime.BuiltinObj)
		// Do not gate on b.Arity here: many builtins treat trailing args as
		// optional (e.g. map/range workers). Each builtin validates itself.
		v, err := b.Fn(args)
		if err != nil {
			return runtime.Null(), vm.wrapErr(err)
		}
		return v, nil
	case runtime.KindFunc:
		fn := callee.Obj.(*runtime.FuncObj)
		if err := arityErr(fn, args); err != nil {
			return runtime.Null(), vm.wrapErr(err)
		}
		// Coverage: record function hit
		if vm.Env.Coverage != nil && fn.Name != "" {
			key := fn.Name
			if ch, ok := fn.Chunk.(*compile.Chunk); ok && ch.File != "" {
				key = ch.File + ":" + fn.Name
			}
			vm.Env.Coverage[key] = true
		}
		// Package/module functions carry a home Env for sibling globals.
		if fn.Env != nil && fn.Env != vm.Env {
			sub := New(fn.Env)
			v, err := sub.RunFunc(fn, args)
			if err != nil {
				// merge stacks: callee frames then caller
				if re, ok := err.(*RuntimeError); ok {
					re.Stack = append(re.Stack, vm.stackTrace()...)
					return runtime.Null(), re
				}
				return runtime.Null(), vm.wrapErr(err)
			}
			return v, nil
		}
		chunk := fn.Chunk.(*compile.Chunk)
		slots := make([]runtime.Value, chunk.NumLocs)
		for i := 0; i < fn.Arity && i < len(args); i++ {
			slots[i] = args[i]
		}
		// Captured locals sit after params (by-value snapshot from OpClose).
		for i, uv := range fn.Upvalues {
			slot := fn.Arity + i
			if slot < len(slots) {
				slots[slot] = uv
			}
		}
		vm.frames = append(vm.frames, frame{
			fn: fn, chunk: chunk, ip: 0, slots: slots, bp: len(vm.stack),
		})
		vm.calledFrame = true
		return runtime.Null(), nil
	default:
		return runtime.Null(), vm.errf("cannot call %s", callee.KindName())
	}
}

// debugCheck pauses execution if a breakpoint matches or step mode is on.
func (vm *VM) debugCheck(fr *frame) {
	if vm.Debug == nil || vm.Debug.OnPause == nil {
		return
	}
	loc := frameLoc(fr)
	shouldPause := vm.Debug.StepMode
	if loc.File != "" && loc.Line > 0 {
		key := fmt.Sprintf("%s:%d", loc.File, loc.Line)
		bkey := fmt.Sprintf("%s:%d", filepath.Base(loc.File), loc.Line)
		// After continue, ignore breakpoints until the line changes (multi-op lines).
		if vm.Debug.SkipBP != "" {
			if key == vm.Debug.SkipBP || bkey == vm.Debug.SkipBP {
				if !vm.Debug.StepMode {
					shouldPause = false
				}
			} else {
				vm.Debug.SkipBP = ""
			}
		}
		if vm.Debug.SkipBP == "" && !shouldPause {
			if vm.Debug.Breakpoints[key] || vm.Debug.Breakpoints[bkey] {
				shouldPause = true
			}
		}
	}
	if shouldPause {
		if loc.File != "" && loc.Line > 0 {
			// continue will skip this line until IP moves to another line
			vm.Debug.SkipBP = fmt.Sprintf("%s:%d", loc.File, loc.Line)
		}
		// Collect locals for inspection
		locals := map[string]runtime.Value{}
		for i, slot := range fr.slots {
			name := fmt.Sprintf("local_%d", i)
			if fr.fn != nil && fr.fn.Chunk != nil {
				if ch, ok := fr.fn.Chunk.(*compile.Chunk); ok && i < len(ch.LocalNames) {
					name = ch.LocalNames[i]
				}
			}
			locals[name] = slot
		}
		vm.Debug.LastStack = vm.stackTrace()
		vm.Debug.Paused = true
		vm.Debug.OnPause(loc, locals)
		vm.Debug.Paused = false
	}
}

// NewDebugState creates a debug state with no breakpoints.
func NewDebugState() *DebugState {
	return &DebugState{
		Breakpoints: map[string]bool{},
	}
}
