package vm_test

import (
	"strings"
	"testing"

	"github.com/loreste/weft/internal/compile"
	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/internal/vm"
)

func runChunk(t *testing.T, ch *compile.Chunk) (err error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("VM must not panic: %v", r)
		}
	}()
	fo := &runtime.FuncObj{Name: ch.Name, Arity: 0, Chunk: ch}
	_, err = vm.New(runtime.NewEnv()).RunFunc(fo, nil)
	return err
}

func TestVM_NoPanicOnStackUnderflow(t *testing.T) {
	ch := &compile.Chunk{
		Name:    "under",
		Code:    []byte{byte(compile.OpPop), byte(compile.OpReturn)},
		NumLocs: 0,
		Lines:   []int{1, 1},
	}
	err := runChunk(t, ch)
	if err == nil || !strings.Contains(err.Error(), "underflow") {
		t.Fatalf("want stack underflow error, got %v", err)
	}
}

func TestVM_NoPanicOnConstOOB(t *testing.T) {
	ch := &compile.Chunk{
		Name:    "const",
		Code:    []byte{byte(compile.OpLoadConst), 0, 9, byte(compile.OpReturn)},
		Consts:  []any{runtime.Int(1)},
		NumLocs: 0,
		Lines:   []int{1, 1, 1, 1},
	}
	err := runChunk(t, ch)
	if err == nil {
		t.Fatal("want const OOB error")
	}
}

func TestVM_NoPanicOnLocalOOB(t *testing.T) {
	ch := &compile.Chunk{
		Name:    "loc",
		Code:    []byte{byte(compile.OpLoadLocal), 0, 5, byte(compile.OpReturn)},
		NumLocs: 1,
		Lines:   []int{1, 1, 1, 1},
	}
	// slots length = NumLocs = 1, index 5 OOB
	err := runChunk(t, ch)
	if err == nil {
		t.Fatal("want local OOB error")
	}
}

func TestVM_NoPanicOnTruncatedOperand(t *testing.T) {
	ch := &compile.Chunk{
		Name:    "trunc",
		Code:    []byte{byte(compile.OpLoadConst), 0}, // missing low byte
		Consts:  []any{runtime.Int(1)},
		NumLocs: 0,
		Lines:   []int{1, 1},
	}
	err := runChunk(t, ch)
	if err == nil {
		t.Fatal("want truncated operand error")
	}
}

func TestVM_NoPanicOnMisalignedJump(t *testing.T) {
	// Even if validation is skipped, runtime must not panic.
	ch := &compile.Chunk{
		Name: "misalign",
		Code: []byte{
			byte(compile.OpJump), 0, 1,
			byte(compile.OpLoadConst), 0, 0,
			byte(compile.OpReturn),
		},
		Consts:  []any{runtime.Int(1)},
		NumLocs: 0,
		Lines:   []int{1, 1, 1, 1, 1, 1, 1},
	}
	err := runChunk(t, ch)
	// may error (underflow/bad op) but must not panic
	_ = err
}
