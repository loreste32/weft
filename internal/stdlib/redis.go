//go:build !js && !slim

package stdlib

import (
	"context"
	"strings"
	"time"

	"github.com/loreste/weft/internal/runtime"
	"github.com/redis/go-redis/v9"
)

// packageRedis — Redis / Valkey / compatible NoSQL KV + pubsub.
//
//	r := redis.connect("redis://localhost:6379/0")?
//	r.set("k", "v")?
//	r.get("k")?
func packageRedis(env *runtime.Env) runtime.Value {
	p := pkg()

	set(p, "connect", func(args []runtime.Value) (runtime.Value, error) {
		addr := "redis://127.0.0.1:6379/0"
		if len(args) >= 1 && args[0].String() != "" {
			addr = args[0].String()
		}
		opt, err := redis.ParseURL(addr)
		if err != nil {
			// bare host:port
			if !strings.Contains(addr, "://") {
				opt = &redis.Options{Addr: addr}
			} else {
				return errRes(err.Error(), "redis"), nil
			}
		}
		if len(args) >= 2 {
			if n := mapGetInt(args[1], "db", -1); n >= 0 {
				if v, e := safeInt(n); e == nil {
					opt.DB = v
				}
			}
			if s := mapGetStr(args[1], "password", ""); s != "" {
				opt.Password = s
			}
		}
		rdb := redis.NewClient(opt)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := rdb.Ping(ctx).Err(); err != nil {
			_ = rdb.Close()
			return errRes(err.Error(), "redis"), nil
		}
		return runtime.Ok(wrapRedis(env, rdb)), nil
	}, 2)

	return p
}

func wrapRedis(env *runtime.Env, rdb *redis.Client) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("redis."+name, arity, fn)
	}
	ctx := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), 10*time.Second)
	}

	put("get", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("r.get(key)", "redis"), nil
		}
		c, cancel := ctx()
		defer cancel()
		s, err := rdb.Get(c, args[0].String()).Result()
		if err == redis.Nil {
			return runtime.Ok(runtime.Null()), nil
		}
		if err != nil {
			return errRes(err.Error(), "redis"), nil
		}
		return runtime.Ok(runtime.Str(s)), nil
	})
	put("set", 3, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("r.set(key, value, ttl_sec?)", "redis"), nil
		}
		var exp time.Duration
		if len(args) >= 3 {
			if n, err := runtime.AsInt(args[2]); err == nil && n > 0 {
				exp = time.Duration(n) * time.Second
			}
		}
		c, cancel := ctx()
		defer cancel()
		if err := rdb.Set(c, args[0].String(), args[1].String(), exp).Err(); err != nil {
			return errRes(err.Error(), "redis"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})
	put("del", -1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("r.del(key...)", "redis"), nil
		}
		keys := make([]string, len(args))
		for i, a := range args {
			keys[i] = a.String()
		}
		c, cancel := ctx()
		defer cancel()
		n, err := rdb.Del(c, keys...).Result()
		if err != nil {
			return errRes(err.Error(), "redis"), nil
		}
		return runtime.Ok(runtime.Int(n)), nil
	})
	put("exists", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Ok(runtime.Bool(false)), nil
		}
		c, cancel := ctx()
		defer cancel()
		n, err := rdb.Exists(c, args[0].String()).Result()
		if err != nil {
			return errRes(err.Error(), "redis"), nil
		}
		return runtime.Ok(runtime.Bool(n > 0)), nil
	})
	put("hset", 3, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 {
			return errRes("r.hset(key, field, value)", "redis"), nil
		}
		c, cancel := ctx()
		defer cancel()
		if err := rdb.HSet(c, args[0].String(), args[1].String(), args[2].String()).Err(); err != nil {
			return errRes(err.Error(), "redis"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})
	put("hget", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("r.hget(key, field)", "redis"), nil
		}
		c, cancel := ctx()
		defer cancel()
		s, err := rdb.HGet(c, args[0].String(), args[1].String()).Result()
		if err == redis.Nil {
			return runtime.Ok(runtime.Null()), nil
		}
		if err != nil {
			return errRes(err.Error(), "redis"), nil
		}
		return runtime.Ok(runtime.Str(s)), nil
	})
	put("hgetall", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("r.hgetall(key)", "redis"), nil
		}
		c, cancel := ctx()
		defer cancel()
		m, err := rdb.HGetAll(c, args[0].String()).Result()
		if err != nil {
			return errRes(err.Error(), "redis"), nil
		}
		return runtime.Ok(stringMapValue(m)), nil
	})
	put("lpush", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("r.lpush(key, value)", "redis"), nil
		}
		c, cancel := ctx()
		defer cancel()
		n, err := rdb.LPush(c, args[0].String(), args[1].String()).Result()
		if err != nil {
			return errRes(err.Error(), "redis"), nil
		}
		return runtime.Ok(runtime.Int(n)), nil
	})
	put("rpop", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("r.rpop(key)", "redis"), nil
		}
		c, cancel := ctx()
		defer cancel()
		s, err := rdb.RPop(c, args[0].String()).Result()
		if err == redis.Nil {
			return runtime.Ok(runtime.Null()), nil
		}
		if err != nil {
			return errRes(err.Error(), "redis"), nil
		}
		return runtime.Ok(runtime.Str(s)), nil
	})
	put("publish", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("r.publish(channel, message)", "redis"), nil
		}
		c, cancel := ctx()
		defer cancel()
		n, err := rdb.Publish(c, args[0].String(), args[1].String()).Result()
		if err != nil {
			return errRes(err.Error(), "redis"), nil
		}
		return runtime.Ok(runtime.Int(n)), nil
	})
	// r.brpop(timeout_sec, key...) -> Result[[key, value]|null]  worker queue
	put("brpop", -1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("r.brpop(timeout_sec, key...)", "redis"), nil
		}
		to, err := runtime.AsInt(args[0])
		if err != nil {
			return errRes("brpop timeout seconds", "redis"), nil
		}
		keys := make([]string, 0, len(args)-1)
		for _, a := range args[1:] {
			keys = append(keys, a.String())
		}
		c, cancel := context.WithTimeout(context.Background(), time.Duration(to+5)*time.Second)
		defer cancel()
		res, err := rdb.BRPop(c, time.Duration(to)*time.Second, keys...).Result()
		if err == redis.Nil {
			return runtime.Ok(runtime.Null()), nil
		}
		if err != nil {
			return errRes(err.Error(), "redis"), nil
		}
		items := make([]runtime.Value, len(res))
		for i, s := range res {
			items[i] = runtime.Str(s)
		}
		return runtime.Ok(runtime.List(items...)), nil
	})
	put("blpop", -1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("r.blpop(timeout_sec, key...)", "redis"), nil
		}
		to, err := runtime.AsInt(args[0])
		if err != nil {
			return errRes("blpop timeout seconds", "redis"), nil
		}
		keys := make([]string, 0, len(args)-1)
		for _, a := range args[1:] {
			keys = append(keys, a.String())
		}
		c, cancel := context.WithTimeout(context.Background(), time.Duration(to+5)*time.Second)
		defer cancel()
		res, err := rdb.BLPop(c, time.Duration(to)*time.Second, keys...).Result()
		if err == redis.Nil {
			return runtime.Ok(runtime.Null()), nil
		}
		if err != nil {
			return errRes(err.Error(), "redis"), nil
		}
		items := make([]runtime.Value, len(res))
		for i, s := range res {
			items[i] = runtime.Str(s)
		}
		return runtime.Ok(runtime.List(items...)), nil
	})
	// r.subscribe(channel, handler) — background pub/sub worker
	put("subscribe", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("r.subscribe(channel, handler)", "redis"), nil
		}
		handler := args[1]
		if handler.Kind != runtime.KindFunc && handler.Kind != runtime.KindBuiltin {
			return errRes("handler must be a function", "redis"), nil
		}
		if env.Call == nil {
			return errRes("runtime Call not configured", "redis"), nil
		}
		ch := args[0].String()
		pubsub := rdb.Subscribe(context.Background(), ch)
		go func() {
			defer pubsub.Close()
			c := pubsub.Channel()
			for msg := range c {
				m := runtime.NewMap()
				mo := m.Obj.(*runtime.MapObj)
				mo.Keys = []string{"channel", "data"}
				mo.Vals["channel"] = runtime.Str(msg.Channel)
				mo.Vals["data"] = runtime.Str(msg.Payload)
				_, _ = env.Call(handler, []runtime.Value{m})
			}
		}()
		// return handle with close
		h := runtime.NewMap()
		ho := h.Obj.(*runtime.MapObj)
		ho.Keys = []string{"close", "channel"}
		ho.Vals["channel"] = runtime.Str(ch)
		ho.Vals["close"] = runtime.MakeBuiltin("sub.close", 0, func(args []runtime.Value) (runtime.Value, error) {
			_ = pubsub.Close()
			return runtime.Ok(runtime.Unit()), nil
		})
		return runtime.Ok(h), nil
	})
	put("incr", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("r.incr(key)", "redis"), nil
		}
		c, cancel := ctx()
		defer cancel()
		n, err := rdb.Incr(c, args[0].String()).Result()
		if err != nil {
			return errRes(err.Error(), "redis"), nil
		}
		return runtime.Ok(runtime.Int(n)), nil
	})
	put("close", 0, func(args []runtime.Value) (runtime.Value, error) {
		if err := rdb.Close(); err != nil {
			return errRes(err.Error(), "redis"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})
	put("ping", 0, func(args []runtime.Value) (runtime.Value, error) {
		c, cancel := ctx()
		defer cancel()
		if err := rdb.Ping(c).Err(); err != nil {
			return errRes(err.Error(), "redis"), nil
		}
		return runtime.Ok(runtime.Bool(true)), nil
	})
	return m
}
