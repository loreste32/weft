package runtime

import (
	"fmt"
	"sync"
	"time"
)

type taskResult struct {
	val Value
	err error
}

// spawnTask runs fn in a goroutine. Returns a handle map with:
//
//	.join()  — raw value (or Err Result on failure); blocking
//	.await() — always Result (Ok/Err) for use with `?`
//
// No shared mutable heap: args are deep-copied (design: concurrent by default).
func spawnTask(env *Env, fn Value, args []Value) (Value, error) {
	if fn.Kind != KindFunc && fn.Kind != KindBuiltin {
		return Null(), fmt.Errorf("spawn: expected function")
	}
	if env.Call == nil {
		return Null(), fmt.Errorf("spawn: Call not configured")
	}
	// Deep-copy args so tasks do not share mutable collections
	cp := make([]Value, len(args))
	for i, a := range args {
		cp[i] = DeepCopy(a)
	}
	ch := make(chan taskResult, 1)
	go func() {
		v, err := env.Call(fn, cp)
		ch <- taskResult{val: v, err: err}
	}()
	// Buffer once so .join and .await can both be called safely (same result).
	var once sync.Once
	var cached taskResult
	get := func() taskResult {
		once.Do(func() { cached = <-ch })
		return cached
	}
	h := NewMap()
	mo := h.Obj.(*MapObj)
	mo.Keys = []string{"join", "await"}
	mo.Vals["join"] = MakeBuiltin("task.join", 0, func(_ []Value) (Value, error) {
		r := get()
		if r.err != nil {
			return Err(NewError(r.err.Error(), "task")), nil
		}
		return r.val, nil
	})
	// await always returns Result for concurrent default: `spawn(fn).await()?`
	mo.Vals["await"] = MakeBuiltin("task.await", 0, func(_ []Value) (Value, error) {
		r := get()
		if r.err != nil {
			return Err(NewError(r.err.Error(), "task")), nil
		}
		if r.val.Kind == KindResult {
			return r.val, nil
		}
		return Ok(r.val), nil
	})
	return h, nil
}

func newTaskGroup(env *Env) Value {
	type state struct {
		mu  sync.Mutex
		fns []Value
		env *Env
	}
	st := &state{env: env}
	g := NewMap()
	mo := g.Obj.(*MapObj)
	mo.Keys = []string{"go", "wait", "spawn"}
	add := func(fn Value) {
		st.mu.Lock()
		st.fns = append(st.fns, fn)
		st.mu.Unlock()
	}
	mo.Vals["go"] = MakeBuiltin("group.go", 1, func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Null(), fmt.Errorf("group.go(fn)")
		}
		add(args[0])
		return Unit(), nil
	})
	mo.Vals["spawn"] = mo.Vals["go"] // alias
	mo.Vals["wait"] = MakeBuiltin("group.wait", 0, func(_ []Value) (Value, error) {
		st.mu.Lock()
		fns := append([]Value{}, st.fns...)
		st.fns = nil
		st.mu.Unlock()
		return parallelTasks(st.env, fns)
	})
	return g
}

func parallelTasks(env *Env, fns []Value) (Value, error) {
	if env.Call == nil {
		return Err(NewError("Call not configured", "task")), nil
	}
	type item struct {
		i   int
		val Value
		err error
	}
	out := make([]Value, len(fns))
	var wg sync.WaitGroup
	ch := make(chan item, len(fns))
	for i, fn := range fns {
		if fn.Kind != KindFunc && fn.Kind != KindBuiltin {
			return Err(NewError(fmt.Sprintf("parallel[%d]: not a function", i), "task")), nil
		}
		wg.Add(1)
		go func(i int, fn Value) {
			defer wg.Done()
			v, err := env.Call(fn, nil)
			ch <- item{i: i, val: v, err: err}
		}(i, fn)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	for it := range ch {
		if it.err != nil {
			return Err(NewError(it.err.Error(), "task")), nil
		}
		out[it.i] = it.val
	}
	return Ok(List(out...)), nil
}

// raceTasks runs all fns concurrently; first completed value wins (others abandoned).
// Concurrent-by-default alternative to asyncio.wait(FIRST_COMPLETED).
func raceTasks(env *Env, fns []Value) (Value, error) {
	if env.Call == nil {
		return Err(NewError("Call not configured", "task")), nil
	}
	if len(fns) == 0 {
		return Err(NewError("race([]) needs at least one fn", "task")), nil
	}
	type item struct {
		val Value
		err error
	}
	ch := make(chan item, len(fns))
	for i, fn := range fns {
		if fn.Kind != KindFunc && fn.Kind != KindBuiltin {
			return Err(NewError(fmt.Sprintf("race[%d]: not a function", i), "task")), nil
		}
		go func(fn Value) {
			v, err := env.Call(fn, nil)
			ch <- item{val: v, err: err}
		}(fn)
	}
	it := <-ch
	if it.err != nil {
		return Err(NewError(it.err.Error(), "task")), nil
	}
	if it.val.Kind == KindResult {
		return it.val, nil
	}
	return Ok(it.val), nil
}

// timeoutTask runs fn concurrently; if not done within d, returns Err kind "timeout".
func timeoutTask(env *Env, d time.Duration, fn Value) (Value, error) {
	if env.Call == nil {
		return Err(NewError("Call not configured", "task")), nil
	}
	if fn.Kind != KindFunc && fn.Kind != KindBuiltin {
		return Err(NewError("timeout: expected function", "task")), nil
	}
	if d <= 0 {
		return Err(NewError("timeout: duration must be positive", "task")), nil
	}
	ch := make(chan taskResult, 1)
	go func() {
		v, err := env.Call(fn, nil)
		ch <- taskResult{val: v, err: err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			return Err(NewError(r.err.Error(), "task")), nil
		}
		if r.val.Kind == KindResult {
			return r.val, nil
		}
		return Ok(r.val), nil
	case <-time.After(d):
		return Err(NewError("timeout", "timeout")), nil
	}
}
