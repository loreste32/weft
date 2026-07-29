package stdlib

import (
	"fmt"
	"sync"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// packageSupervisor — Erlang-style process supervision for telecom and long-lived services.
func packageSupervisor(env *runtime.Env) runtime.Value {
	p := pkg()

	// supervisor.new(opts) -> map
	// Create a supervisor with a restart strategy.
	// opts: {strategy: "one_for_one"|"one_for_all"|"rest_for_one", max_restarts, within_seconds}
	set(p, "new", func(args []runtime.Value) (runtime.Value, error) {
		s := &supervisor{
			strategy:      "one_for_one",
			maxRestarts:   3,
			withinSeconds: 10,
			children:      make(map[string]*childSpec),
			env:           env,
		}
		if len(args) >= 1 && args[0].Kind == runtime.KindMap {
			mo := args[0].Obj.(*runtime.MapObj)
			if v, ok := mo.Vals["strategy"]; ok && v.Kind != runtime.KindNull {
				s.strategy = v.String()
			}
			if v, ok := mo.Vals["max_restarts"]; ok {
				if n, e := runtime.AsInt(v); e == nil {
					s.maxRestarts = int(n)
				}
			}
			if v, ok := mo.Vals["within_seconds"]; ok {
				if n, e := runtime.AsInt(v); e == nil {
					s.withinSeconds = int(n)
				}
			}
		}
		return wrapSupervisor(s), nil
	}, 1)

	// supervisor.process(fn, opts?) -> map
	// Create an isolated process (actor) with a mailbox.
	set(p, "process", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("supervisor.process(fn, opts?)", "supervisor"), nil
		}
		proc := &weftProcess{
			mailbox: make(chan runtime.Value, 100),
			done:    make(chan struct{}),
			env:     env,
		}
		return wrapProcess(proc, args[0]), nil
	}, 2)

	return p
}

// ─── supervisor ───────────────────────────────────────────────────

type supervisor struct {
	mu            sync.Mutex
	strategy      string
	maxRestarts   int
	withinSeconds int
	children      map[string]*childSpec
	childOrder    []string
	restartLog    []time.Time
	env           *runtime.Env
}

type childSpec struct {
	name    string
	fn      runtime.Value
	args    []runtime.Value
	running bool
	pid     int
	done    chan struct{}
	cancel  func()
	err     error
}

func (s *supervisor) checkRestartBudget() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-time.Duration(s.withinSeconds) * time.Second)
	var recent []time.Time
	for _, t := range s.restartLog {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	s.restartLog = recent
	return len(recent) < s.maxRestarts
}

func (s *supervisor) recordRestart() {
	s.mu.Lock()
	s.restartLog = append(s.restartLog, time.Now())
	s.mu.Unlock()
}

func wrapSupervisor(s *supervisor) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	putFn := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("supervisor."+name, arity, fn)
	}

	// sup.start_child(name, fn, args?) -> Result
	putFn("start_child", 3, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("sup.start_child(name, fn, args?)", "supervisor"), nil
		}
		name := args[0].String()
		fn := args[1]
		var fnArgs []runtime.Value
		if len(args) >= 3 && args[2].Kind == runtime.KindList {
			lo := args[2].Obj.(*runtime.ListObj)
			fnArgs = lo.Items
		}

		s.mu.Lock()
		if _, exists := s.children[name]; exists {
			s.mu.Unlock()
			return errRes("child already exists: "+name, "supervisor"), nil
		}

		child := &childSpec{
			name: name,
			fn:   fn,
			args: fnArgs,
			done: make(chan struct{}),
		}
		s.children[name] = child
		s.childOrder = append(s.childOrder, name)
		s.mu.Unlock()

		go s.runChild(child)
		return runtime.Ok(runtime.Str(name)), nil
	})

	// sup.stop_child(name) -> Result
	putFn("stop_child", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("sup.stop_child(name)", "supervisor"), nil
		}
		name := args[0].String()
		s.mu.Lock()
		child, ok := s.children[name]
		s.mu.Unlock()
		if !ok {
			return errRes("child not found: "+name, "supervisor"), nil
		}
		if child.cancel != nil {
			child.cancel()
		}
		child.running = false
		return runtime.Ok(runtime.Unit()), nil
	})

	// sup.children() -> [map]
	putFn("children", 0, func(args []runtime.Value) (runtime.Value, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		items := make([]runtime.Value, 0, len(s.children))
		for _, name := range s.childOrder {
			child := s.children[name]
			cm := runtime.NewMap()
			cmo := cm.Obj.(*runtime.MapObj)
			cmo.Keys = append(cmo.Keys, "name", "running")
			cmo.Vals["name"] = runtime.Str(child.name)
			cmo.Vals["running"] = runtime.Bool(child.running)
			if child.err != nil {
				cmo.Keys = append(cmo.Keys, "error")
				cmo.Vals["error"] = runtime.Str(child.err.Error())
			}
			items = append(items, cm)
		}
		return runtime.List(items...), nil
	})

	// sup.stats() -> map
	putFn("stats", 0, func(args []runtime.Value) (runtime.Value, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		running := 0
		for _, child := range s.children {
			if child.running {
				running++
			}
		}
		result := runtime.NewMap()
		rmo := result.Obj.(*runtime.MapObj)
		rmo.Keys = append(rmo.Keys, "total", "running", "restarts", "strategy")
		rmo.Vals["total"] = runtime.Int(int64(len(s.children)))
		rmo.Vals["running"] = runtime.Int(int64(running))
		rmo.Vals["restarts"] = runtime.Int(int64(len(s.restartLog)))
		rmo.Vals["strategy"] = runtime.Str(s.strategy)
		return result, nil
	})

	// sup.shutdown() -> unit
	putFn("shutdown", 0, func(args []runtime.Value) (runtime.Value, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, child := range s.children {
			if child.cancel != nil {
				child.cancel()
			}
			child.running = false
		}
		return runtime.Unit(), nil
	})

	return m
}

func (s *supervisor) runChild(child *childSpec) {
	child.running = true
	defer func() {
		child.running = false
		close(child.done)
	}()

	// run the function
	if child.fn.Kind == runtime.KindBuiltin {
		bo := child.fn.Obj.(*runtime.BuiltinObj)
		_, err := bo.Fn(child.args)
		if err != nil {
			child.err = err
			s.handleChildFailure(child)
		}
	}
}

func (s *supervisor) handleChildFailure(failed *childSpec) {
	if !s.checkRestartBudget() {
		fmt.Printf("supervisor: max restarts exceeded, not restarting %s\n", failed.name)
		return
	}
	s.recordRestart()

	switch s.strategy {
	case "one_for_one":
		// restart only the failed child
		failed.done = make(chan struct{})
		go s.runChild(failed)

	case "one_for_all":
		// restart all children
		s.mu.Lock()
		for _, name := range s.childOrder {
			child := s.children[name]
			if child.cancel != nil {
				child.cancel()
			}
			child.running = false
		}
		for _, name := range s.childOrder {
			child := s.children[name]
			child.done = make(chan struct{})
			go s.runChild(child)
		}
		s.mu.Unlock()

	case "rest_for_one":
		// restart the failed child and all children started after it
		s.mu.Lock()
		found := false
		for _, name := range s.childOrder {
			if name == failed.name {
				found = true
			}
			if found {
				child := s.children[name]
				if child.cancel != nil {
					child.cancel()
				}
				child.running = false
				child.done = make(chan struct{})
				go s.runChild(child)
			}
		}
		s.mu.Unlock()
	}
}

// ─── isolated process (actor) ─────────────────────────────────────

type weftProcess struct {
	mu      sync.Mutex
	mailbox chan runtime.Value
	done    chan struct{}
	env     *runtime.Env
	running bool
	err     error
}

func wrapProcess(proc *weftProcess, fn runtime.Value) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	putFn := func(name string, arity int, f runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("process."+name, arity, f)
	}

	// proc.start() — start the process
	putFn("start", 0, func(args []runtime.Value) (runtime.Value, error) {
		proc.running = true
		go func() {
			defer func() {
				proc.running = false
				close(proc.done)
			}()
			if fn.Kind == runtime.KindBuiltin {
				bo := fn.Obj.(*runtime.BuiltinObj)
				_, err := bo.Fn(nil)
				if err != nil {
					proc.err = err
				}
			}
		}()
		return runtime.Ok(runtime.Unit()), nil
	})

	// proc.send(msg) — send a message to the process mailbox
	putFn("send", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("proc.send(msg)", "supervisor"), nil
		}
		select {
		case proc.mailbox <- args[0]:
			return runtime.Ok(runtime.Unit()), nil
		default:
			return errRes("mailbox full", "supervisor"), nil
		}
	})

	// proc.recv(timeout_sec?) — receive a message from the mailbox
	putFn("recv", 1, func(args []runtime.Value) (runtime.Value, error) {
		timeout := 30 * time.Second
		if len(args) >= 1 {
			if n, e := runtime.AsInt(args[0]); e == nil && n > 0 {
				timeout = time.Duration(n) * time.Second
			}
		}
		select {
		case msg := <-proc.mailbox:
			return runtime.Ok(msg), nil
		case <-time.After(timeout):
			return errRes("recv timeout", "supervisor"), nil
		case <-proc.done:
			return errRes("process exited", "supervisor"), nil
		}
	})

	// proc.alive() -> bool
	putFn("alive", 0, func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Bool(proc.running), nil
	})

	// proc.stop()
	putFn("stop", 0, func(args []runtime.Value) (runtime.Value, error) {
		proc.running = false
		return runtime.Unit(), nil
	})

	return m
}
