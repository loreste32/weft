//go:build !js && !slim

package stdlib

import (
	"time"

	"github.com/nats-io/nats.go"

	"github.com/loreste/weft/internal/runtime"
)

// packageNATS — NATS messaging.
//
//	nc := nats.connect("nats://127.0.0.1:4222")?
//	nc.publish("jobs", payload)?
//	nc.subscribe("jobs", fn(msg) { say(msg.data) })?
func packageNATS(env *runtime.Env) runtime.Value {
	p := pkg()

	set(p, "connect", func(args []runtime.Value) (runtime.Value, error) {
		url := nats.DefaultURL
		if len(args) >= 1 && args[0].String() != "" {
			url = args[0].String()
		}
		opts := []nats.Option{
			nats.Name("weft"),
			nats.Timeout(5 * time.Second),
		}
		if len(args) >= 2 {
			if s := mapGetStr(args[1], "token", ""); s != "" {
				opts = append(opts, nats.Token(s))
			}
			if s := mapGetStr(args[1], "user", ""); s != "" {
				opts = append(opts, nats.UserInfo(s, mapGetStr(args[1], "password", "")))
			}
		}
		nc, err := nats.Connect(url, opts...)
		if err != nil {
			return errRes(err.Error(), "nats"), nil
		}
		return runtime.Ok(wrapNATS(env, nc)), nil
	}, 2)

	return p
}

func wrapNATS(env *runtime.Env, nc *nats.Conn) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("nats."+name, arity, fn)
	}

	put("publish", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("nc.publish(subject, data)", "nats"), nil
		}
		if err := nc.Publish(args[0].String(), []byte(args[1].String())); err != nil {
			return errRes(err.Error(), "nats"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})

	// nc.request(subject, data, timeout_sec?) -> Result[str]
	put("request", 3, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("nc.request(subject, data, timeout_sec?)", "nats"), nil
		}
		to := 2 * time.Second
		if len(args) >= 3 {
			if n, err := runtime.AsInt(args[2]); err == nil && n > 0 {
				to = time.Duration(n) * time.Second
			}
		}
		msg, err := nc.Request(args[0].String(), []byte(args[1].String()), to)
		if err != nil {
			return errRes(err.Error(), "nats"), nil
		}
		return runtime.Ok(runtime.Str(string(msg.Data))), nil
	})

	// nc.subscribe(subject, handler) -> Result[sub]
	// handler(msg) where msg = {subject, data, reply}
	put("subscribe", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("nc.subscribe(subject, handler)", "nats"), nil
		}
		subject := args[0].String()
		handler := args[1]
		if handler.Kind != runtime.KindFunc && handler.Kind != runtime.KindBuiltin {
			return errRes("handler must be a function", "nats"), nil
		}
		if env.Call == nil {
			return errRes("runtime Call not configured", "nats"), nil
		}
		sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
			mv := natsMsgValue(msg)
			_, _ = env.Call(handler, []runtime.Value{mv})
		})
		if err != nil {
			return errRes(err.Error(), "nats"), nil
		}
		return runtime.Ok(wrapNATSSub(sub)), nil
	})

	// nc.queue_subscribe(subject, queue, handler)
	put("queue_subscribe", 3, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 {
			return errRes("nc.queue_subscribe(subject, queue, handler)", "nats"), nil
		}
		handler := args[2]
		if env.Call == nil {
			return errRes("runtime Call not configured", "nats"), nil
		}
		sub, err := nc.QueueSubscribe(args[0].String(), args[1].String(), func(msg *nats.Msg) {
			_, _ = env.Call(handler, []runtime.Value{natsMsgValue(msg)})
		})
		if err != nil {
			return errRes(err.Error(), "nats"), nil
		}
		return runtime.Ok(wrapNATSSub(sub)), nil
	})

	put("flush", 1, func(args []runtime.Value) (runtime.Value, error) {
		to := 2 * time.Second
		if len(args) >= 1 {
			if n, err := runtime.AsInt(args[0]); err == nil {
				to = time.Duration(n) * time.Second
			}
		}
		if err := nc.FlushTimeout(to); err != nil {
			return errRes(err.Error(), "nats"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})

	put("close", 0, func(args []runtime.Value) (runtime.Value, error) {
		nc.Close()
		return runtime.Ok(runtime.Unit()), nil
	})

	put("connected", 0, func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Bool(nc.IsConnected()), nil
	})

	return m
}

func natsMsgValue(msg *nats.Msg) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"subject", "data", "reply"}
	mo.Vals["subject"] = runtime.Str(msg.Subject)
	mo.Vals["data"] = runtime.Str(string(msg.Data))
	mo.Vals["reply"] = runtime.Str(msg.Reply)
	// reply helper
	mo.Keys = append(mo.Keys, "respond")
	mo.Vals["respond"] = runtime.MakeBuiltin("msg.respond", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("msg.respond(data)", "nats"), nil
		}
		if msg.Reply == "" {
			return errRes("no reply subject", "nats"), nil
		}
		if err := msg.Respond([]byte(args[0].String())); err != nil {
			return errRes(err.Error(), "nats"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})
	return m
}

func wrapNATSSub(sub *nats.Subscription) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"unsubscribe", "drain", "subject"}
	mo.Vals["subject"] = runtime.Str(sub.Subject)
	mo.Vals["unsubscribe"] = runtime.MakeBuiltin("sub.unsubscribe", 0, func(args []runtime.Value) (runtime.Value, error) {
		if err := sub.Unsubscribe(); err != nil {
			return errRes(err.Error(), "nats"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})
	mo.Vals["drain"] = runtime.MakeBuiltin("sub.drain", 0, func(args []runtime.Value) (runtime.Value, error) {
		if err := sub.Drain(); err != nil {
			return errRes(err.Error(), "nats"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})
	return m
}
