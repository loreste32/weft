package stdlib

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"sync"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// Script PRNG (not for secrets — use crypto.random_hex for tokens).
var (
	rngMu sync.Mutex
	rng   = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// packageRandom — Python random lite (deterministic seeding for tests/scripts).
func packageRandom() runtime.Value {
	p := pkg()

	set(p, "seed", func(args []runtime.Value) (runtime.Value, error) {
		var seed int64
		if len(args) >= 1 {
			if n, err := runtime.AsInt(args[0]); err == nil {
				seed = n
			}
		} else {
			var b [8]byte
			if _, err := crand.Read(b[:]); err == nil {
				seed = int64(binary.LittleEndian.Uint64(b[:]))
			} else {
				seed = time.Now().UnixNano()
			}
		}
		rngMu.Lock()
		rng = rand.New(rand.NewSource(seed))
		rngMu.Unlock()
		return runtime.Int(seed), nil
	}, 1)

	// random.float() -> [0,1)
	set(p, "float", func(args []runtime.Value) (runtime.Value, error) {
		rngMu.Lock()
		v := rng.Float64()
		rngMu.Unlock()
		return runtime.Float(v), nil
	}, 0)

	// random.int(n) -> [0,n) or random.int(lo, hi) inclusive
	set(p, "int", func(args []runtime.Value) (runtime.Value, error) {
		rngMu.Lock()
		defer rngMu.Unlock()
		if len(args) >= 2 {
			lo, err1 := runtime.AsInt(args[0])
			hi, err2 := runtime.AsInt(args[1])
			if err1 != nil || err2 != nil || hi < lo {
				return runtime.Int(0), nil
			}
			span := hi - lo + 1
			if span <= 0 {
				return runtime.Int(lo), nil
			}
			return runtime.Int(lo + int64(rng.Int63n(span))), nil
		}
		if len(args) == 1 {
			n, err := runtime.AsInt(args[0])
			if err != nil || n <= 0 {
				return runtime.Int(0), nil
			}
			return runtime.Int(rng.Int63n(n)), nil
		}
		return runtime.Int(rng.Int63()), nil
	}, 2)

	// random.choice(list) -> element or null
	set(p, "choice", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return runtime.Null(), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		if len(items) == 0 {
			return runtime.Null(), nil
		}
		rngMu.Lock()
		i := rng.Intn(len(items))
		rngMu.Unlock()
		return items[i], nil
	}, 1)

	// random.shuffle(list) -> new shuffled list
	set(p, "shuffle", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return runtime.List(), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		out := make([]runtime.Value, len(items))
		copy(out, items)
		rngMu.Lock()
		rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
		rngMu.Unlock()
		return runtime.List(out...), nil
	}, 1)

	// random.sample(list, k) -> k unique elements (or fewer)
	set(p, "sample", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindList {
			return runtime.List(), nil
		}
		items := args[0].Obj.(*runtime.ListObj).Items
		k := 1
		if len(args) >= 2 {
			if n, err := runtime.AsInt(args[1]); err == nil && n >= 0 {
				k = int(n)
			}
		}
		if k > len(items) {
			k = len(items)
		}
		out := make([]runtime.Value, len(items))
		copy(out, items)
		rngMu.Lock()
		rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
		rngMu.Unlock()
		return runtime.List(out[:k]...), nil
	}, 2)

	// random.bool() -> bool
	set(p, "bool", func(args []runtime.Value) (runtime.Value, error) {
		rngMu.Lock()
		b := rng.Intn(2) == 1
		rngMu.Unlock()
		return runtime.Bool(b), nil
	}, 0)

	// random.bytes(n) -> str of n random bytes (latin-1-ish); prefer crypto for secrets
	set(p, "bytes", func(args []runtime.Value) (runtime.Value, error) {
		n := 16
		if len(args) >= 1 {
			if x, err := runtime.AsInt(args[0]); err == nil && x > 0 {
				n = int(x)
			}
		}
		if n > 1<<20 {
			return errRes("random.bytes: n too large", "random"), nil
		}
		b := make([]byte, n)
		rngMu.Lock()
		for i := range b {
			b[i] = byte(rng.Intn(256))
		}
		rngMu.Unlock()
		return runtime.Str(string(b)), nil
	}, 1)

	return p
}
