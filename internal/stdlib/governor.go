package stdlib

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// packageGovernor — execution governors and token budgeting for LLM calls and long-lived scripts.
func packageGovernor(env *runtime.Env) runtime.Value {
	p := pkg()

	// governor.new(opts) -> map
	// Create an execution governor with token/time/cost budgets.
	// opts: {max_tokens, max_duration_sec, max_cost_usd, max_steps, on_exceeded}
	set(p, "new", func(args []runtime.Value) (runtime.Value, error) {
		g := &governor{
			maxTokens:    0,
			maxDuration:  0,
			maxCostMicro: 0,
			maxSteps:     0,
			onExceeded:   "raise",
		}
		if len(args) >= 1 && args[0].Kind == runtime.KindMap {
			mo := args[0].Obj.(*runtime.MapObj)
			if v, ok := mo.Vals["max_tokens"]; ok {
				if n, e := runtime.AsInt(v); e == nil {
					g.maxTokens = n
				}
			}
			if v, ok := mo.Vals["max_duration_sec"]; ok {
				if n, e := runtime.AsInt(v); e == nil {
					g.maxDuration = time.Duration(n) * time.Second
				}
			}
			if v, ok := mo.Vals["max_cost_usd"]; ok {
				if f, ok := asFloat64(v); ok {
					g.maxCostMicro = int64(f * 1_000_000)
				}
			}
			if v, ok := mo.Vals["max_steps"]; ok {
				if n, e := runtime.AsInt(v); e == nil {
					g.maxSteps = n
				}
			}
			if v, ok := mo.Vals["on_exceeded"]; ok && v.Kind != runtime.KindNull {
				g.onExceeded = v.String()
			}
		}
		if g.maxDuration > 0 {
			g.ctx, g.cancel = context.WithTimeout(env.Context(), g.maxDuration)
		} else {
			g.ctx, g.cancel = context.WithCancel(env.Context())
		}
		return wrapGovernor(g), nil
	}, 1)

	return p
}

type governor struct {
	mu           sync.Mutex
	maxTokens    int64
	maxDuration  time.Duration
	maxCostMicro int64
	maxSteps     int64
	onExceeded   string // "raise", "truncate", "hangup"

	usedTokens   atomic.Int64
	promptTokens atomic.Int64
	compTokens   atomic.Int64
	costMicro    atomic.Int64
	steps        atomic.Int64
	startTime    time.Time
	exceeded     atomic.Bool

	ctx    context.Context
	cancel context.CancelFunc
}

func (g *governor) track(prompt, completion int64, costUSD float64) error {
	g.promptTokens.Add(prompt)
	g.compTokens.Add(completion)
	total := prompt + completion
	g.usedTokens.Add(total)
	g.costMicro.Add(int64(costUSD * 1_000_000))
	g.steps.Add(1)

	if g.maxTokens > 0 && g.usedTokens.Load() > g.maxTokens {
		g.exceeded.Store(true)
		return fmt.Errorf("token budget exceeded: %d/%d", g.usedTokens.Load(), g.maxTokens)
	}
	if g.maxCostMicro > 0 && g.costMicro.Load() > g.maxCostMicro {
		g.exceeded.Store(true)
		return fmt.Errorf("cost budget exceeded: $%.4f/$%.4f", float64(g.costMicro.Load())/1e6, float64(g.maxCostMicro)/1e6)
	}
	if g.maxSteps > 0 && g.steps.Load() > g.maxSteps {
		g.exceeded.Store(true)
		return fmt.Errorf("step limit exceeded: %d/%d", g.steps.Load(), g.maxSteps)
	}
	if g.ctx.Err() != nil {
		g.exceeded.Store(true)
		return fmt.Errorf("execution timeout")
	}
	return nil
}

func wrapGovernor(g *governor) runtime.Value {
	g.startTime = time.Now()
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	putFn := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("governor."+name, arity, fn)
	}

	// gov.track(prompt_tokens, completion_tokens, cost_usd?) -> Result
	putFn("track", 3, func(args []runtime.Value) (runtime.Value, error) {
		prompt := int64(0)
		comp := int64(0)
		cost := 0.0
		if len(args) >= 1 {
			if n, e := runtime.AsInt(args[0]); e == nil {
				prompt = n
			}
		}
		if len(args) >= 2 {
			if n, e := runtime.AsInt(args[1]); e == nil {
				comp = n
			}
		}
		if len(args) >= 3 {
			if f, ok := asFloat64(args[2]); ok {
				cost = f
			}
		}
		if err := g.track(prompt, comp, cost); err != nil {
			if g.onExceeded == "raise" {
				return errRes(err.Error(), "governor"), nil
			}
			// truncate/hangup — return exceeded status but don't error
			result := runtime.NewMap()
			rmo := result.Obj.(*runtime.MapObj)
			rmo.Keys = append(rmo.Keys, "exceeded", "reason")
			rmo.Vals["exceeded"] = runtime.Bool(true)
			rmo.Vals["reason"] = runtime.Str(err.Error())
			return runtime.Ok(result), nil
		}
		return runtime.Ok(runtime.Bool(false)), nil
	})

	// gov.check() -> Result[bool]  true if still within budget
	putFn("check", 0, func(args []runtime.Value) (runtime.Value, error) {
		if g.exceeded.Load() {
			return runtime.Ok(runtime.Bool(false)), nil
		}
		if g.ctx.Err() != nil {
			g.exceeded.Store(true)
			return runtime.Ok(runtime.Bool(false)), nil
		}
		return runtime.Ok(runtime.Bool(true)), nil
	})

	// gov.stats() -> map
	putFn("stats", 0, func(args []runtime.Value) (runtime.Value, error) {
		result := runtime.NewMap()
		rmo := result.Obj.(*runtime.MapObj)
		put := func(k string, v runtime.Value) {
			rmo.Keys = append(rmo.Keys, k)
			rmo.Vals[k] = v
		}
		put("total_tokens", runtime.Int(g.usedTokens.Load()))
		put("prompt_tokens", runtime.Int(g.promptTokens.Load()))
		put("completion_tokens", runtime.Int(g.compTokens.Load()))
		put("cost_usd", runtime.Float(float64(g.costMicro.Load())/1e6))
		put("steps", runtime.Int(g.steps.Load()))
		put("elapsed_sec", runtime.Float(time.Since(g.startTime).Seconds()))
		put("exceeded", runtime.Bool(g.exceeded.Load()))
		if g.maxTokens > 0 {
			put("remaining_tokens", runtime.Int(g.maxTokens-g.usedTokens.Load()))
		}
		return result, nil
	})

	// gov.cancel() — cancel the execution context
	putFn("cancel", 0, func(args []runtime.Value) (runtime.Value, error) {
		g.cancel()
		g.exceeded.Store(true)
		return runtime.Unit(), nil
	})

	// gov.remaining_tokens() -> int
	putFn("remaining_tokens", 0, func(args []runtime.Value) (runtime.Value, error) {
		if g.maxTokens <= 0 {
			return runtime.Int(-1), nil
		}
		return runtime.Int(g.maxTokens - g.usedTokens.Load()), nil
	})

	// gov.elapsed() -> float (seconds)
	putFn("elapsed", 0, func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Float(time.Since(g.startTime).Seconds()), nil
	})

	return m
}
