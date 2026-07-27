package stdlib

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageTest — Weft-side assertions for `weft test`.
// Failures return Go errors so the VM stops the test function immediately.
func packageTest() runtime.Value {
	p := pkg()

	// test.eq(got, want) / test.equal
	eq := func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Null(), fmt.Errorf("test.eq(got, want)")
		}
		if !runtime.Equal(args[0], args[1]) {
			return runtime.Null(), fmt.Errorf("test.eq: got %s, want %s", args[0].String(), args[1].String())
		}
		return runtime.Unit(), nil
	}
	set(p, "eq", eq, 2)
	set(p, "equal", eq, 2)

	// test.ne(a, b)
	set(p, "ne", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Null(), fmt.Errorf("test.ne(a, b)")
		}
		if runtime.Equal(args[0], args[1]) {
			return runtime.Null(), fmt.Errorf("test.ne: both equal %s", args[0].String())
		}
		return runtime.Unit(), nil
	}, 2)

	// test.is_true(v) — "true" is a keyword, cannot be a field name
	isTrue := func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || !args[0].IsTruthy() {
			msg := "test.is_true: value is falsey"
			if len(args) >= 1 {
				msg = fmt.Sprintf("test.is_true: got %s", args[0].String())
			}
			return runtime.Null(), fmt.Errorf("%s", msg)
		}
		return runtime.Unit(), nil
	}
	set(p, "is_true", isTrue, 1)
	set(p, "ok_bool", isTrue, 1) // alias

	// test.is_false(v)
	set(p, "is_false", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Null(), fmt.Errorf("test.is_false(v)")
		}
		if args[0].IsTruthy() {
			return runtime.Null(), fmt.Errorf("test.is_false: got %s", args[0].String())
		}
		return runtime.Unit(), nil
	}, 1)

	// test.ok(result) — Result must be Ok (unwraps not required)
	set(p, "ok", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Null(), fmt.Errorf("test.ok(result)")
		}
		v := args[0]
		if v.Kind != runtime.KindResult {
			return runtime.Null(), fmt.Errorf("test.ok: not a Result (got %s)", v.KindName())
		}
		ro := v.Obj.(*runtime.ResultObj)
		if !ro.Ok {
			return runtime.Null(), fmt.Errorf("test.ok: Err(%s)", ro.Err.String())
		}
		return runtime.Unit(), nil
	}, 1)

	// test.err(result) — Result must be Err
	set(p, "err", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Null(), fmt.Errorf("test.err(result)")
		}
		v := args[0]
		if v.Kind != runtime.KindResult {
			return runtime.Null(), fmt.Errorf("test.err: not a Result")
		}
		ro := v.Obj.(*runtime.ResultObj)
		if ro.Ok {
			return runtime.Null(), fmt.Errorf("test.err: got Ok(%s)", ro.Val.String())
		}
		return runtime.Unit(), nil
	}, 1)

	// test.contains(haystack, needle)
	set(p, "contains", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Null(), fmt.Errorf("test.contains(haystack, needle)")
		}
		hay, needle := args[0], args[1]
		switch hay.Kind {
		case runtime.KindStr:
			if !strings.Contains(hay.S, needle.String()) {
				return runtime.Null(), fmt.Errorf("test.contains: %q not in %q", needle.String(), hay.S)
			}
		case runtime.KindList:
			found := false
			for _, it := range hay.Obj.(*runtime.ListObj).Items {
				if runtime.Equal(it, needle) {
					found = true
					break
				}
			}
			if !found {
				return runtime.Null(), fmt.Errorf("test.contains: %s not in list", needle.String())
			}
		default:
			if !strings.Contains(hay.String(), needle.String()) {
				return runtime.Null(), fmt.Errorf("test.contains: %q not in %q", needle.String(), hay.String())
			}
		}
		return runtime.Unit(), nil
	}, 2)

	// test.approx(a, b, eps?) — float close enough
	set(p, "approx", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Null(), fmt.Errorf("test.approx(a, b, eps?)")
		}
		a, okA := asFloat64(args[0])
		b, okB := asFloat64(args[1])
		if !okA || !okB {
			return runtime.Null(), fmt.Errorf("test.approx: need numbers")
		}
		eps := 1e-9
		if len(args) >= 3 {
			if e, ok := asFloat64(args[2]); ok && e > 0 {
				eps = e
			}
		}
		if math.Abs(a-b) > eps {
			return runtime.Null(), fmt.Errorf("test.approx: |%g - %g| = %g > %g", a, b, math.Abs(a-b), eps)
		}
		return runtime.Unit(), nil
	}, 3)

	// test.fail(msg?)
	set(p, "fail", func(args []runtime.Value) (runtime.Value, error) {
		msg := "test.fail"
		if len(args) >= 1 && args[0].String() != "" {
			msg = "test.fail: " + args[0].String()
		}
		return runtime.Null(), fmt.Errorf("%s", msg)
	}, 1)

	// test.assert(cond, msg?) — cond must be truthy
	set(p, "assert", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || !args[0].IsTruthy() {
			msg := "test.assert failed"
			if len(args) >= 2 && args[1].String() != "" {
				msg = "test.assert: " + args[1].String()
			}
			return runtime.Null(), fmt.Errorf("%s", msg)
		}
		return runtime.Unit(), nil
	}, 2)

	// test.skip(msg?) — runner marks as skipped
	set(p, "skip", func(args []runtime.Value) (runtime.Value, error) {
		msg := "skipped"
		if len(args) >= 1 && args[0].String() != "" {
			msg = args[0].String()
		}
		return runtime.Null(), &TestSkip{Msg: msg}
	}, 1)

	// test.is_null(v) — "null" is a keyword
	set(p, "is_null", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindNull {
			got := "missing"
			if len(args) >= 1 {
				got = args[0].String()
			}
			return runtime.Null(), fmt.Errorf("test.is_null: got %s", got)
		}
		return runtime.Unit(), nil
	}, 1)

	return p
}

// TestSkipPrefix marks skip errors after VM wrapErr (which stringifies causes).
const TestSkipPrefix = "weft:test:skip:"

// TestSkip is returned by test.skip — runner treats it as SKIP not FAIL.
type TestSkip struct {
	Msg string
}

func (e *TestSkip) Error() string {
	msg := "skipped"
	if e != nil && e.Msg != "" {
		msg = e.Msg
	}
	return TestSkipPrefix + msg
}

// IsTestSkip reports whether err (possibly VM-wrapped) is test.skip.
func IsTestSkip(err error) (msg string, ok bool) {
	if err == nil {
		return "", false
	}
	var skip *TestSkip
	if errors.As(err, &skip) {
		return skip.Msg, true
	}
	s := err.Error()
	if i := strings.Index(s, TestSkipPrefix); i >= 0 {
		return s[i+len(TestSkipPrefix):], true
	}
	return "", false
}
