package stdlib

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/loreste/weft/internal/runtime"
)

// packageSignal — cooperative shutdown flags (Python signal lite).
// Does not install custom handlers that call Weft code (unsafe under VM);
// exposes notified flags for runbooks/workers to poll.
func packageSignal(env *runtime.Env) runtime.Value {
	p := pkg()

	var mu sync.Mutex
	got := map[string]bool{}
	var once sync.Once

	ensure := func() {
		once.Do(func() {
			ch := make(chan os.Signal, 4)
			signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
			go func() {
				for sig := range ch {
					mu.Lock()
					switch sig {
					case os.Interrupt:
						got["SIGINT"] = true
						got["INT"] = true
					case syscall.SIGTERM:
						got["SIGTERM"] = true
						got["TERM"] = true
					}
					got["any"] = true
					mu.Unlock()
				}
			}()
		})
	}

	// signal.listen() — start watching SIGINT/SIGTERM (idempotent)
	set(p, "listen", func(args []runtime.Value) (runtime.Value, error) {
		ensure()
		return runtime.Unit(), nil
	}, 0)

	// signal.received(name?) -> bool  name: SIGINT|SIGTERM|INT|TERM|any
	set(p, "received", func(args []runtime.Value) (runtime.Value, error) {
		ensure()
		name := "any"
		if len(args) >= 1 {
			name = args[0].String()
		}
		mu.Lock()
		defer mu.Unlock()
		return runtime.Bool(got[name]), nil
	}, 1)

	// signal.reset() — clear flags
	set(p, "reset", func(args []runtime.Value) (runtime.Value, error) {
		mu.Lock()
		got = map[string]bool{}
		mu.Unlock()
		return runtime.Unit(), nil
	}, 0)

	// signal.pid() -> int
	set(p, "pid", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Int(int64(os.Getpid())), nil
	}, 0)

	return p
}
