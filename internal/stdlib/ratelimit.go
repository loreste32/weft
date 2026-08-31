package stdlib

import (
	"sync"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// packageRatelimit: token bucket rate limiter for API calls.
//
//	rl := ratelimit.new(10, "second")   // 10 req/sec
//	rl := ratelimit.new(100, "minute")  // 100 req/min
//	ratelimit.wait(rl)                  // blocks until token available
//	ratelimit.acquire(rl) -> bool           // non-blocking
func packageRatelimit() runtime.Value {
	p := pkg()

	set(p, "new", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("ratelimit.new(rate, unit)", "ratelimit"), nil
		}
		rate, err := runtime.AsInt(args[0])
		if err != nil || rate <= 0 {
			return errRes("rate must be positive int", "ratelimit"), nil
		}
		unit := args[1].String()
		var interval time.Duration
		switch unit {
		case "second", "sec", "s":
			interval = time.Second
		case "minute", "min", "m":
			interval = time.Minute
		case "hour", "hr", "h":
			interval = time.Hour
		default:
			return errRes("unit must be second/minute/hour", "ratelimit"), nil
		}

		safeRate, err := safeInt(rate)
		if err != nil {
			return errRes("rate too large", "ratelimit"), nil
		}
		bucket := &tokenBucket{
			rate:     safeRate,
			interval: interval,
			tokens:   safeRate,
			last:     time.Now(),
		}

		handle := runtime.NewMap()
		mo := handle.Obj.(*runtime.MapObj)
		mo.Keys = []string{"_rl", "wait", "acquire", "rate", "unit"}
		mo.Vals["_rl"] = runtime.Str("ratelimit")
		mo.Vals["rate"] = runtime.Int(rate)
		mo.Vals["unit"] = runtime.Str(unit)

		mo.Vals["wait"] = runtime.MakeBuiltin("ratelimit.wait", 0, func(_ []runtime.Value) (runtime.Value, error) {
			bucket.wait()
			return runtime.Unit(), nil
		})
		mo.Vals["acquire"] = runtime.MakeBuiltin("ratelimit.acquire", 0, func(_ []runtime.Value) (runtime.Value, error) {
			return runtime.Bool(bucket.tryAcquire()), nil
		})
		return handle, nil
	}, 2)

	// Convenience: ratelimit.wait(rl) and ratelimit.acquire(rl)
	set(p, "wait", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("ratelimit.wait(rl)", "ratelimit"), nil
		}
		if args[0].Kind == runtime.KindMap {
			if fn, ok := args[0].Obj.(*runtime.MapObj).Vals["wait"]; ok {
				return fn.Obj.(*runtime.BuiltinObj).Fn(nil)
			}
		}
		return runtime.Unit(), nil
	}, 1)

	set(p, "acquire", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Bool(false), nil
		}
		if args[0].Kind == runtime.KindMap {
			if fn, ok := args[0].Obj.(*runtime.MapObj).Vals["acquire"]; ok {
				return fn.Obj.(*runtime.BuiltinObj).Fn(nil)
			}
		}
		return runtime.Bool(false), nil
	}, 1)

	return p
}

type tokenBucket struct {
	mu       sync.Mutex
	rate     int
	interval time.Duration
	tokens   int
	last     time.Time
}

func (b *tokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(b.last)
	add := int(elapsed * time.Duration(b.rate) / b.interval)
	if add > 0 {
		b.tokens += add
		if b.tokens > b.rate {
			b.tokens = b.rate
		}
		b.last = now
	}
}

func (b *tokenBucket) tryAcquire() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refill()
	if b.tokens > 0 {
		b.tokens--
		return true
	}
	return false
}

func (b *tokenBucket) wait() {
	for {
		if b.tryAcquire() {
			return
		}
		time.Sleep(b.interval / time.Duration(b.rate+1))
	}
}
