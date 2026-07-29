//go:build slim && !js

package stdlib

import (
	"fmt"

	"github.com/loreste/weft/internal/runtime"
)

// Slim build: omit SQL/broker drivers from the binary. Packages remain
// importable but methods return a clear error (see docs/STABILITY.md).

func slimStub(name string) runtime.Value {
	m := runtime.NewMap()
	for _, method := range []string{
		"open", "connect", "query", "exec", "close", "ping",
		"get", "set", "publish", "subscribe", "consume",
		"migrate", "up", "down",
	} {
		n := name + "." + method
		set(m, method, func(args []runtime.Value) (runtime.Value, error) {
			return runtime.Null(), fmt.Errorf("%s is not available in the slim build (rebuild without -tags slim)", n)
		}, -1)
	}
	return m
}

func packageDB(env *runtime.Env) runtime.Value      { return slimStub("db") }
func packageMigrate(env *runtime.Env) runtime.Value { return slimStub("migrate") }
func packageMongo(env *runtime.Env) runtime.Value   { return slimStub("mongo") }
func packageRedis(env *runtime.Env) runtime.Value   { return slimStub("redis") }
func packageNATS(env *runtime.Env) runtime.Value    { return slimStub("nats") }
func packageAMQP(env *runtime.Env) runtime.Value    { return slimStub("amqp") }
