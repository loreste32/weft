package stdlib

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// packageCluster — distributed state, locking, and node coordination for multi-instance deployments.
func packageCluster(env *runtime.Env) runtime.Value {
	p := pkg()

	// ─── distributed key-value store (backed by Redis) ────────────

	// cluster.store(redis_url) -> Result[map]
	// Connect to a shared state store (Redis-backed).
	set(p, "store", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("cluster.store(redis_url)", "cluster"), nil
		}
		url := args[0].String()
		return runtime.Ok(wrapClusterStore(url, env)), nil
	}, 1)

	// ─── distributed locks ────────────────────────────────────────

	// cluster.lock(store, key, opts?) -> Result[map]
	// Acquire a distributed lock. opts: {ttl_sec, wait_sec, owner}
	set(p, "lock", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("cluster.lock(store, key, opts?)", "cluster"), nil
		}
		store := args[0]
		key := args[1].String()
		ttl := 30
		wait := 10
		owner := fmt.Sprintf("weft-%d", time.Now().UnixNano())

		if len(args) >= 3 && args[2].Kind == runtime.KindMap {
			mo := args[2].Obj.(*runtime.MapObj)
			if v, ok := mo.Vals["ttl_sec"]; ok {
				if n, e := runtime.AsInt(v); e == nil {
					ttl = int(n)
				}
			}
			if v, ok := mo.Vals["wait_sec"]; ok {
				if n, e := runtime.AsInt(v); e == nil {
					wait = int(n)
				}
			}
			if v, ok := mo.Vals["owner"]; ok && v.Kind != runtime.KindNull {
				owner = v.String()
			}
		}

		lockKey := "weft:lock:" + key
		deadline := time.Now().Add(time.Duration(wait) * time.Second)

		// try to acquire with spin
		for time.Now().Before(deadline) {
			ok := storeSetNX(store, lockKey, owner, ttl)
			if ok {
				return runtime.Ok(wrapDistLock(store, lockKey, owner, ttl)), nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		return errRes(fmt.Sprintf("lock %q not acquired within %ds", key, wait), "cluster"), nil
	}, 3)

	// ─── node registry ────────────────────────────────────────────

	// cluster.register(store, node_id, opts?) -> Result[map]
	// Register this node in the cluster. opts: {ttl_sec, metadata}
	set(p, "register", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("cluster.register(store, node_id, opts?)", "cluster"), nil
		}
		store := args[0]
		nodeID := args[1].String()
		ttl := 30
		metadata := map[string]string{}

		if len(args) >= 3 && args[2].Kind == runtime.KindMap {
			mo := args[2].Obj.(*runtime.MapObj)
			if v, ok := mo.Vals["ttl_sec"]; ok {
				if n, e := runtime.AsInt(v); e == nil {
					ttl = int(n)
				}
			}
			if v, ok := mo.Vals["metadata"]; ok && v.Kind == runtime.KindMap {
				mmo := v.Obj.(*runtime.MapObj)
				for _, k := range mmo.Keys {
					metadata[k] = mmo.Vals[k].String()
				}
			}
		}

		// store node registration
		nodeKey := "weft:node:" + nodeID
		nodeData, _ := json.Marshal(map[string]any{
			"id":         nodeID,
			"registered": time.Now().UTC().Format(time.RFC3339),
			"metadata":   metadata,
		})
		storeSet(store, nodeKey, string(nodeData), ttl)

		// add to node set
		storeSetAdd(store, "weft:nodes", nodeID)

		// start heartbeat
		node := &clusterNode{
			store:  store,
			id:     nodeID,
			key:    nodeKey,
			ttl:    ttl,
			data:   string(nodeData),
			ctx:    env.Context(),
			cancel: func() {},
		}
		ctx, cancel := context.WithCancel(env.Context())
		node.ctx = ctx
		node.cancel = cancel
		go node.heartbeat()

		return runtime.Ok(wrapClusterNode(node)), nil
	}, 3)

	// cluster.nodes(store) -> Result[[map]]
	// List all registered nodes.
	set(p, "nodes", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("cluster.nodes(store)", "cluster"), nil
		}
		store := args[0]
		members := storeSetMembers(store, "weft:nodes")
		items := make([]runtime.Value, 0)
		for _, nodeID := range members {
			data := storeGet(store, "weft:node:"+nodeID)
			if data == "" {
				continue // expired
			}
			var info map[string]any
			json.Unmarshal([]byte(data), &info)
			items = append(items, goToValue(info))
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)

	// ─── distributed rate limiter ─────────────────────────────────

	// cluster.rate_limit(store, key, max_requests, window_sec) -> Result[bool]
	// Returns true if request is allowed, false if rate limited.
	set(p, "rate_limit", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 4 {
			return errRes("cluster.rate_limit(store, key, max, window_sec)", "cluster"), nil
		}
		store := args[0]
		key := "weft:ratelimit:" + args[1].String()
		maxReqs, _ := runtime.AsInt(args[2])
		windowSec, _ := runtime.AsInt(args[3])

		count := storeIncr(store, key)
		if count == 1 {
			storeExpire(store, key, int(windowSec))
		}
		return runtime.Ok(runtime.Bool(count <= maxReqs)), nil
	}, 4)

	// ─── distributed counter ──────────────────────────────────────

	// cluster.counter(store, key) -> Result[map]
	// Get a distributed atomic counter.
	set(p, "counter", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("cluster.counter(store, key)", "cluster"), nil
		}
		store := args[0]
		key := "weft:counter:" + args[1].String()
		return runtime.Ok(wrapDistCounter(store, key)), nil
	}, 2)

	// ─── pub/sub ──────────────────────────────────────────────────

	// cluster.publish(store, channel, message) -> Result
	set(p, "publish", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 {
			return errRes("cluster.publish(store, channel, message)", "cluster"), nil
		}
		store := args[0]
		channel := args[1].String()
		msg := args[2].String()
		storePublish(store, channel, msg)
		return runtime.Ok(runtime.Unit()), nil
	}, 3)

	return p
}

// ─── store operations (delegate to redis stdlib) ──────────────────

func getStoreURL(store runtime.Value) string {
	if store.Kind == runtime.KindMap {
		mo := store.Obj.(*runtime.MapObj)
		if v, ok := mo.Vals["_url"]; ok {
			return v.String()
		}
	}
	return store.String()
}

func storeCall(store runtime.Value, method string, args map[string]string) string {
	_ = getStoreURL(store)
	// Use HTTP interface if store is an HTTP URL, otherwise use redis commands
	// For simplicity, delegate to the redis stdlib via stored function handles
	if store.Kind == runtime.KindMap {
		mo := store.Obj.(*runtime.MapObj)
		if fn, ok := mo.Vals[method]; ok && fn.Kind == runtime.KindBuiltin {
			bo := fn.Obj.(*runtime.BuiltinObj)
			var fnArgs []runtime.Value
			for k, v := range args {
				_ = k
				fnArgs = append(fnArgs, runtime.Str(v))
			}
			result, _ := bo.Fn(fnArgs)
			if result.Kind == runtime.KindStruct {
				// Result type — unwrap
				ro := result.Obj.(*runtime.ResultObj)
				if ro.Ok {
					return ro.Val.String()
				}
			}
			return result.String()
		}
	}
	return ""
}

func storeSet(store runtime.Value, key, value string, ttlSec int) {
	if store.Kind != runtime.KindMap {
		return
	}
	mo := store.Obj.(*runtime.MapObj)
	if fn, ok := mo.Vals["set"]; ok && fn.Kind == runtime.KindBuiltin {
		bo := fn.Obj.(*runtime.BuiltinObj)
		bo.Fn([]runtime.Value{runtime.Str(key), runtime.Str(value)})
	}
	if ttlSec > 0 {
		storeExpire(store, key, ttlSec)
	}
}

func storeGet(store runtime.Value, key string) string {
	if store.Kind != runtime.KindMap {
		return ""
	}
	mo := store.Obj.(*runtime.MapObj)
	if fn, ok := mo.Vals["get"]; ok && fn.Kind == runtime.KindBuiltin {
		bo := fn.Obj.(*runtime.BuiltinObj)
		result, _ := bo.Fn([]runtime.Value{runtime.Str(key)})
		if result.Kind == runtime.KindStruct {
			ro := result.Obj.(*runtime.ResultObj)
			if ro.Ok {
				return ro.Val.String()
			}
		}
		return result.String()
	}
	return ""
}

func storeSetNX(store runtime.Value, key, value string, ttlSec int) bool {
	if store.Kind != runtime.KindMap {
		return false
	}
	mo := store.Obj.(*runtime.MapObj)
	if fn, ok := mo.Vals["setnx"]; ok && fn.Kind == runtime.KindBuiltin {
		bo := fn.Obj.(*runtime.BuiltinObj)
		result, _ := bo.Fn([]runtime.Value{runtime.Str(key), runtime.Str(value)})
		if result.B {
			if ttlSec > 0 {
				storeExpire(store, key, ttlSec)
			}
			return true
		}
	}
	return false
}

func storeIncr(store runtime.Value, key string) int64 {
	if store.Kind != runtime.KindMap {
		return 0
	}
	mo := store.Obj.(*runtime.MapObj)
	if fn, ok := mo.Vals["incr"]; ok && fn.Kind == runtime.KindBuiltin {
		bo := fn.Obj.(*runtime.BuiltinObj)
		result, _ := bo.Fn([]runtime.Value{runtime.Str(key)})
		if n, e := runtime.AsInt(result); e == nil {
			return n
		}
	}
	return 0
}

func storeExpire(store runtime.Value, key string, sec int) {
	if store.Kind != runtime.KindMap {
		return
	}
	mo := store.Obj.(*runtime.MapObj)
	if fn, ok := mo.Vals["expire"]; ok && fn.Kind == runtime.KindBuiltin {
		bo := fn.Obj.(*runtime.BuiltinObj)
		bo.Fn([]runtime.Value{runtime.Str(key), runtime.Int(int64(sec))})
	}
}

func storeDel(store runtime.Value, key string) {
	if store.Kind != runtime.KindMap {
		return
	}
	mo := store.Obj.(*runtime.MapObj)
	if fn, ok := mo.Vals["del"]; ok && fn.Kind == runtime.KindBuiltin {
		bo := fn.Obj.(*runtime.BuiltinObj)
		bo.Fn([]runtime.Value{runtime.Str(key)})
	}
}

func storeSetAdd(store runtime.Value, key, member string) {
	if store.Kind != runtime.KindMap {
		return
	}
	mo := store.Obj.(*runtime.MapObj)
	if fn, ok := mo.Vals["sadd"]; ok && fn.Kind == runtime.KindBuiltin {
		bo := fn.Obj.(*runtime.BuiltinObj)
		bo.Fn([]runtime.Value{runtime.Str(key), runtime.Str(member)})
	}
}

func storeSetMembers(store runtime.Value, key string) []string {
	if store.Kind != runtime.KindMap {
		return nil
	}
	mo := store.Obj.(*runtime.MapObj)
	if fn, ok := mo.Vals["smembers"]; ok && fn.Kind == runtime.KindBuiltin {
		bo := fn.Obj.(*runtime.BuiltinObj)
		result, _ := bo.Fn([]runtime.Value{runtime.Str(key)})
		if result.Kind == runtime.KindList {
			lo := result.Obj.(*runtime.ListObj)
			var out []string
			for _, v := range lo.Items {
				out = append(out, v.String())
			}
			return out
		}
	}
	return nil
}

func storePublish(store runtime.Value, channel, msg string) {
	if store.Kind != runtime.KindMap {
		return
	}
	mo := store.Obj.(*runtime.MapObj)
	if fn, ok := mo.Vals["publish"]; ok && fn.Kind == runtime.KindBuiltin {
		bo := fn.Obj.(*runtime.BuiltinObj)
		bo.Fn([]runtime.Value{runtime.Str(channel), runtime.Str(msg)})
	}
}

// ─── store wrapper ────────────────────────────────────────────────

func wrapClusterStore(redisURL string, env *runtime.Env) runtime.Value {
	// Connect to Redis using the redis stdlib
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = append(mo.Keys, "_url")
	mo.Vals["_url"] = runtime.Str(redisURL)

	// get the redis package and call connect
	redisVal, ok := env.Globals["redis"]
	if !ok {
		return m
	}
	if redisVal.Kind != runtime.KindMap {
		return m
	}
	rmo := redisVal.Obj.(*runtime.MapObj)
	connectFn, ok := rmo.Vals["connect"]
	if !ok || connectFn.Kind != runtime.KindBuiltin {
		return m
	}
	bo := connectFn.Obj.(*runtime.BuiltinObj)
	result, err := bo.Fn([]runtime.Value{runtime.Str(redisURL)})
	if err != nil {
		return m
	}
	// unwrap Result
	if result.Kind == runtime.KindStruct {
		ro := result.Obj.(*runtime.ResultObj)
		if ro.Ok && ro.Val.Kind == runtime.KindMap {
			// copy redis client methods to our store
			clientMo := ro.Val.Obj.(*runtime.MapObj)
			for _, k := range clientMo.Keys {
				mo.Keys = append(mo.Keys, k)
				mo.Vals[k] = clientMo.Vals[k]
			}
		}
	}
	return m
}

// ─── distributed lock ─────────────────────────────────────────────

func wrapDistLock(store runtime.Value, key, owner string, ttl int) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	putFn := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("lock."+name, arity, fn)
	}

	mo.Keys = append(mo.Keys, "key", "owner")
	mo.Vals["key"] = runtime.Str(strings.TrimPrefix(key, "weft:lock:"))
	mo.Vals["owner"] = runtime.Str(owner)

	// lock.release() -> Result
	putFn("release", 0, func(args []runtime.Value) (runtime.Value, error) {
		// only release if we still own it
		current := storeGet(store, key)
		if current == owner {
			storeDel(store, key)
			return runtime.Ok(runtime.Bool(true)), nil
		}
		return runtime.Ok(runtime.Bool(false)), nil
	})

	// lock.extend(seconds) -> Result
	putFn("extend", 1, func(args []runtime.Value) (runtime.Value, error) {
		sec := ttl
		if len(args) >= 1 {
			if n, e := runtime.AsInt(args[0]); e == nil {
				sec = int(n)
			}
		}
		current := storeGet(store, key)
		if current == owner {
			storeExpire(store, key, sec)
			return runtime.Ok(runtime.Bool(true)), nil
		}
		return runtime.Ok(runtime.Bool(false)), nil
	})

	return m
}

// ─── node heartbeat ───────────────────────────────────────────────

type clusterNode struct {
	store  runtime.Value
	id     string
	key    string
	ttl    int
	data   string
	ctx    context.Context
	cancel context.CancelFunc
}

func (n *clusterNode) heartbeat() {
	ticker := time.NewTicker(time.Duration(n.ttl/3) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			storeSet(n.store, n.key, n.data, n.ttl)
		}
	}
}

func wrapClusterNode(n *clusterNode) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	putFn := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("node."+name, arity, fn)
	}

	mo.Keys = append(mo.Keys, "id")
	mo.Vals["id"] = runtime.Str(n.id)

	// node.deregister()
	putFn("deregister", 0, func(args []runtime.Value) (runtime.Value, error) {
		n.cancel()
		storeDel(n.store, n.key)
		return runtime.Ok(runtime.Unit()), nil
	})

	return m
}

// ─── distributed counter ──────────────────────────────────────────

func wrapDistCounter(store runtime.Value, key string) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	putFn := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("counter."+name, arity, fn)
	}

	putFn("incr", 0, func(args []runtime.Value) (runtime.Value, error) {
		v := storeIncr(store, key)
		return runtime.Int(v), nil
	})

	putFn("get", 0, func(args []runtime.Value) (runtime.Value, error) {
		v := storeGet(store, key)
		if v == "" {
			return runtime.Int(0), nil
		}
		n := int64(0)
		fmt.Sscanf(v, "%d", &n)
		return runtime.Int(n), nil
	})

	putFn("reset", 0, func(args []runtime.Value) (runtime.Value, error) {
		storeDel(store, key)
		return runtime.Ok(runtime.Unit()), nil
	})

	return m
}

// suppress unused import warnings
var (
	_ = http.StatusOK
	_ = io.EOF
	_ = strings.Contains
	_ = sync.Mutex{}
)
