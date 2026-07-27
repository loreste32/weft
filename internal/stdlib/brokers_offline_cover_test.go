package stdlib

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/loreste/weft/internal/runtime"
	"github.com/nats-io/nats.go"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func callPkg(t *testing.T, p runtime.Value, name string, args ...runtime.Value) runtime.Value {
	t.Helper()
	fn, ok := p.Obj.(*runtime.MapObj).Vals[name]
	if !ok {
		t.Fatalf("missing %s", name)
	}
	r, err := fn.Obj.(*runtime.BuiltinObj).Fn(args)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return r
}

func callMap(t *testing.T, m runtime.Value, name string, args ...runtime.Value) runtime.Value {
	t.Helper()
	fn, ok := m.Obj.(*runtime.MapObj).Vals[name]
	if !ok {
		t.Fatalf("missing method %s", name)
	}
	r, err := fn.Obj.(*runtime.BuiltinObj).Fn(args)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return r
}

func mustOk(t *testing.T, r runtime.Value) runtime.Value {
	t.Helper()
	ro, ok := r.Obj.(*runtime.ResultObj)
	if !ok || !ro.Ok {
		t.Fatalf("expected Ok, got %v", r)
	}
	return ro.Val
}

func mustErr(t *testing.T, r runtime.Value) {
	t.Helper()
	ro, ok := r.Obj.(*runtime.ResultObj)
	if !ok || ro.Ok {
		t.Fatalf("expected Err, got %v", r)
	}
}

func TestRedis_ConnectFailAndParse(t *testing.T) {
	env := runtime.NewEnv()
	p := packageRedis(env)

	// invalid URL with scheme → parse error path (no dial)
	mustErr(t, callPkg(t, p, "connect", runtime.Str("http://not-redis")))

	// options map + refused port (single attempt path via Ping timeout)
	opts := runtime.NewMap()
	omo := opts.Obj.(*runtime.MapObj)
	omo.Keys = []string{"db", "password"}
	omo.Vals["db"] = runtime.Int(2)
	omo.Vals["password"] = runtime.Str("secret")
	mustErr(t, callPkg(t, p, "connect", runtime.Str("127.0.0.1:1"), opts))
}

func TestRedis_MiniredisFull(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	env := runtime.NewEnv()
	env.Call = func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		if fn.Kind == runtime.KindBuiltin {
			return fn.Obj.(*runtime.BuiltinObj).Fn(args)
		}
		return runtime.Unit(), nil
	}
	p := packageRedis(env)
	addr := "redis://" + mr.Addr() + "/0"
	client := mustOk(t, callPkg(t, p, "connect", runtime.Str(addr)))

	// set / get / exists / incr / del
	mustOk(t, callMap(t, client, "set", runtime.Str("k"), runtime.Str("v")))
	mustOk(t, callMap(t, client, "set", runtime.Str("ttl"), runtime.Str("x"), runtime.Int(60)))
	g := mustOk(t, callMap(t, client, "get", runtime.Str("k")))
	if g.S != "v" {
		t.Fatalf("get %v", g)
	}
	miss := mustOk(t, callMap(t, client, "get", runtime.Str("missing")))
	if miss.Kind != runtime.KindNull {
		t.Fatalf("miss %v", miss)
	}
	ex := mustOk(t, callMap(t, client, "exists", runtime.Str("k")))
	if !ex.B {
		t.Fatal("exists")
	}
	// exists arity miss
	ex0 := mustOk(t, callMap(t, client, "exists"))
	if ex0.B {
		t.Fatal("exists empty")
	}
	n := mustOk(t, callMap(t, client, "incr", runtime.Str("n")))
	if n.I != 1 {
		t.Fatalf("incr %v", n)
	}

	// hash
	mustOk(t, callMap(t, client, "hset", runtime.Str("h"), runtime.Str("f"), runtime.Str("1")))
	hg := mustOk(t, callMap(t, client, "hget", runtime.Str("h"), runtime.Str("f")))
	if hg.S != "1" {
		t.Fatalf("hget %v", hg)
	}
	mustOk(t, callMap(t, client, "hget", runtime.Str("h"), runtime.Str("no")))
	ha := mustOk(t, callMap(t, client, "hgetall", runtime.Str("h")))
	if ha.Kind != runtime.KindMap {
		t.Fatalf("hgetall %v", ha)
	}

	// list
	ln := mustOk(t, callMap(t, client, "lpush", runtime.Str("q"), runtime.Str("a")))
	if ln.I < 1 {
		t.Fatal(ln)
	}
	rv := mustOk(t, callMap(t, client, "rpop", runtime.Str("q")))
	if rv.S != "a" {
		t.Fatalf("rpop %v", rv)
	}
	mustOk(t, callMap(t, client, "rpop", runtime.Str("q"))) // nil

	// publish
	pub := mustOk(t, callMap(t, client, "publish", runtime.Str("ch"), runtime.Str("m")))
	_ = pub

	// brpop/blpop short timeout empty
	mustOk(t, callMap(t, client, "brpop", runtime.Int(1), runtime.Str("emptyq")))
	mustOk(t, callMap(t, client, "blpop", runtime.Int(1), runtime.Str("emptyq")))

	// brpop with data
	mr.Lpush("wq", "job1")
	br := mustOk(t, callMap(t, client, "brpop", runtime.Int(2), runtime.Str("wq")))
	if br.Kind != runtime.KindList {
		t.Fatalf("brpop data %v", br)
	}
	mr.Lpush("wq2", "job2")
	bl := mustOk(t, callMap(t, client, "blpop", runtime.Int(2), runtime.Str("wq2")))
	if bl.Kind != runtime.KindList {
		t.Fatalf("blpop data %v", bl)
	}

	// arity errors
	mustErr(t, callMap(t, client, "get"))
	mustErr(t, callMap(t, client, "set", runtime.Str("only")))
	mustErr(t, callMap(t, client, "del"))
	mustErr(t, callMap(t, client, "hset", runtime.Str("h")))
	mustErr(t, callMap(t, client, "hget", runtime.Str("h")))
	mustErr(t, callMap(t, client, "hgetall"))
	mustErr(t, callMap(t, client, "lpush", runtime.Str("q")))
	mustErr(t, callMap(t, client, "rpop"))
	mustErr(t, callMap(t, client, "publish", runtime.Str("c")))
	mustErr(t, callMap(t, client, "brpop", runtime.Int(1)))
	mustErr(t, callMap(t, client, "blpop", runtime.Int(1)))
	mustErr(t, callMap(t, client, "incr"))
	mustErr(t, callMap(t, client, "brpop", runtime.Str("bad"), runtime.Str("k")))
	mustErr(t, callMap(t, client, "blpop", runtime.Str("bad"), runtime.Str("k")))
	mustErr(t, callMap(t, client, "subscribe", runtime.Str("ch")))
	mustErr(t, callMap(t, client, "subscribe", runtime.Str("ch"), runtime.Str("notfn")))

	// del multi
	mustOk(t, callMap(t, client, "set", runtime.Str("d1"), runtime.Str("1")))
	dn := mustOk(t, callMap(t, client, "del", runtime.Str("d1"), runtime.Str("d2")))
	if dn.I < 1 {
		t.Fatal(dn)
	}

	// subscribe with handler (background)
	handler := runtime.MakeBuiltin("h", 1, func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Unit(), nil
	})
	sub := mustOk(t, callMap(t, client, "subscribe", runtime.Str("news"), handler))
	_ = callMap(t, sub, "close")
	// publish while subscribed path already covered by miniredis

	// ping / close
	mustOk(t, callMap(t, client, "ping"))
	mustOk(t, callMap(t, client, "close"))
}

func TestRedis_SubscribeNoCall(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	env := runtime.NewEnv() // Call nil
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	w := wrapRedis(env, rdb)
	handler := runtime.MakeBuiltin("h", 1, func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Unit(), nil
	})
	mustErr(t, callMap(t, w, "subscribe", runtime.Str("c"), handler))
}

func TestNATS_ConnectFail(t *testing.T) {
	env := runtime.NewEnv()
	p := packageNATS(env)

	// refuse quickly
	mustErr(t, callPkg(t, p, "connect", runtime.Str("nats://127.0.0.1:1")))

	// options paths (token / user) still fail dial
	opts := runtime.NewMap()
	omo := opts.Obj.(*runtime.MapObj)
	omo.Keys = []string{"token", "user", "password"}
	omo.Vals["token"] = runtime.Str("tok")
	omo.Vals["user"] = runtime.Str("u")
	omo.Vals["password"] = runtime.Str("p")
	mustErr(t, callPkg(t, p, "connect", runtime.Str("nats://127.0.0.1:1"), opts))

	// empty host falls back to DefaultURL — may succeed if a local broker exists
	r := callPkg(t, p, "connect", runtime.Str(""))
	if ro, ok := r.Obj.(*runtime.ResultObj); ok && ro.Ok {
		_ = callMap(t, ro.Val, "close")
	}
}

func TestNATS_MsgValue(t *testing.T) {
	msg := &nats.Msg{Subject: "jobs", Data: []byte("payload"), Reply: ""}
	v := natsMsgValue(msg)
	mo := v.Obj.(*runtime.MapObj)
	if mo.Vals["subject"].S != "jobs" || mo.Vals["data"].S != "payload" {
		t.Fatal(v)
	}
	// no reply subject
	mustErr(t, callMap(t, v, "respond", runtime.Str("x")))
	mustErr(t, callMap(t, v, "respond")) // arity

	// with reply — Respond fails offline but covers call
	msg2 := &nats.Msg{Subject: "j", Data: []byte("d"), Reply: "inbox.1"}
	v2 := natsMsgValue(msg2)
	// Respond without connection returns error
	r := callMap(t, v2, "respond", runtime.Str("ok"))
	if ro, ok := r.Obj.(*runtime.ResultObj); ok && ro.Ok {
		t.Log("respond ok unexpectedly")
	}
}

func TestNATS_LiveOptional(t *testing.T) {
	env := runtime.NewEnv()
	env.Call = func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		if fn.Kind == runtime.KindBuiltin {
			return fn.Obj.(*runtime.BuiltinObj).Fn(args)
		}
		return runtime.Unit(), nil
	}
	p := packageNATS(env)
	r := callPkg(t, p, "connect", runtime.Str("nats://127.0.0.1:4222"))
	ro, ok := r.Obj.(*runtime.ResultObj)
	if !ok || !ro.Ok {
		t.Skip("no local NATS")
	}
	nc := ro.Val
	mustOk(t, callMap(t, nc, "publish", runtime.Str("weft.test"), runtime.Str("hi")))
	mustErr(t, callMap(t, nc, "publish", runtime.Str("only")))
	// request will timeout
	_ = callMap(t, nc, "request", runtime.Str("weft.none"), runtime.Str("q"), runtime.Int(1))
	mustErr(t, callMap(t, nc, "request", runtime.Str("only")))
	handler := runtime.MakeBuiltin("h", 1, func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Unit(), nil
	})
	sub := mustOk(t, callMap(t, nc, "subscribe", runtime.Str("weft.sub"), handler))
	mustOk(t, callMap(t, sub, "unsubscribe"))
	sub2 := mustOk(t, callMap(t, nc, "queue_subscribe", runtime.Str("weft.q"), runtime.Str("workers"), handler))
	_ = callMap(t, sub2, "drain")
	mustErr(t, callMap(t, nc, "subscribe", runtime.Str("x")))
	mustErr(t, callMap(t, nc, "subscribe", runtime.Str("x"), runtime.Str("notfn")))
	mustErr(t, callMap(t, nc, "queue_subscribe", runtime.Str("a"), runtime.Str("b")))
	_ = callMap(t, nc, "connected")
	mustOk(t, callMap(t, nc, "flush", runtime.Int(1)))
	mustOk(t, callMap(t, nc, "close"))
}

func TestAMQP_ConnectFail(t *testing.T) {
	env := runtime.NewEnv()
	p := packageAMQP(env)
	mustErr(t, callPkg(t, p, "connect", runtime.Str("amqp://guest:guest@127.0.0.1:1/")))
	mustErr(t, callPkg(t, p, "connect", runtime.Str("")))
	mustErr(t, callPkg(t, p, "connect", runtime.Str("://bad")))
}

func TestAMQP_MsgValue(t *testing.T) {
	d := amqp.Delivery{
		Body:        []byte("hello"),
		Exchange:    "ex",
		RoutingKey:  "rk",
		ContentType: "text/plain",
	}
	// autoAck true → ack is no-op success
	v := amqpMsgValue(&d, true)
	mo := v.Obj.(*runtime.MapObj)
	if mo.Vals["data"].S != "hello" {
		t.Fatal(v)
	}
	mustOk(t, callMap(t, v, "ack"))
	// nack without channel errors
	v2 := amqpMsgValue(&d, false)
	_ = callMap(t, v2, "ack")  // may err
	_ = callMap(t, v2, "nack") // may err
	_ = callMap(t, v2, "nack", runtime.Bool(false))
}

func TestMongo_ConnectFail(t *testing.T) {
	env := runtime.NewEnv()
	p := packageMongo(env)
	// invalid URI fails before dial
	mustErr(t, callPkg(t, p, "connect", runtime.Str("mongodb://")))
	// refused ping
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = ctx
	mustErr(t, callPkg(t, p, "connect", runtime.Str("mongodb://127.0.0.1:1")))
}

func TestMongo_BSONHelpers(t *testing.T) {
	// valueToBSON / valueToBSONMap
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"a", "n", "b"}
	mo.Vals["a"] = runtime.Str("x")
	mo.Vals["n"] = runtime.Int(3)
	mo.Vals["b"] = runtime.Bool(true)
	doc := valueToBSON(m)
	if doc == nil {
		t.Fatal("nil doc")
	}
	bm := valueToBSONMap(m)
	if bm["a"] != "x" {
		t.Fatalf("%v", bm)
	}
	// non-map → empty
	empty := valueToBSONMap(runtime.Str("nope"))
	if len(empty) != 0 {
		t.Fatal(empty)
	}

	// hasMongoOp
	if hasMongoOp(bson.M{"name": "a"}) {
		t.Fatal("plain")
	}
	if !hasMongoOp(bson.M{"$set": bson.M{"x": 1}}) {
		t.Fatal("op")
	}

	// bsonSanitize + bsonToValue
	raw := bson.M{
		"s": "hi",
		"nested": bson.M{
			"k": 1,
		},
		"arr": bson.A{"a", 2, bson.M{"z": true}},
		"d": bson.D{
			{Key: "p", Value: "q"},
		},
	}
	v := bsonToValue(raw)
	if v.Kind != runtime.KindMap {
		t.Fatalf("%v", v)
	}

	// stringifyID
	if stringifyID(nil) != "" {
		t.Fatal()
	}
	oid := primitive.NewObjectID()
	if stringifyID(oid) != oid.Hex() {
		t.Fatalf("%s vs %s", stringifyID(oid), oid.Hex())
	}
	if !strings.Contains(stringifyID(42), "42") {
		t.Fatal(stringifyID(42))
	}
}
