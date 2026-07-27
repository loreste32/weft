package runtime_test

import (
	"fmt"
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func TestExitSignalError(t *testing.T) {
	e := &runtime.ExitSignal{Code: 1, Message: "bye"}
	if e.Error() != "bye" {
		t.Fatalf("got %q", e.Error())
	}
	e2 := &runtime.ExitSignal{Code: 42}
	if e2.Error() != "exit 42" {
		t.Fatalf("got %q", e2.Error())
	}
}

func TestIsExit(t *testing.T) {
	code, ok := runtime.IsExit(&runtime.ExitSignal{Code: 3})
	if !ok || code != 3 {
		t.Fatal("should detect exit")
	}
	_, ok = runtime.IsExit(fmt.Errorf("not exit"))
	if ok {
		t.Fatal("regular error is not exit")
	}
	_, ok = runtime.IsExit(nil)
	if ok {
		t.Fatal("nil is not exit")
	}
}
