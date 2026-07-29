package compile

import (
	"testing"

	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/internal/stdlib"
)

func compileSrc(t *testing.T, src string) *Program {
	t.Helper()
	f, errs := parse.ParseFile("t.weft", src)
	if errs.HasErrors() {
		t.Fatal(errs)
	}
	env := runtime.NewEnv()
	stdlib.Register(env, stdlib.Options{})
	prog, cerrs := CompileFile(f, env)
	if cerrs.HasErrors() {
		t.Fatal(cerrs)
	}
	return prog
}

func TestValidateChunkOK(t *testing.T) {
	prog := compileSrc(t, `
fn add(a, b) { a + b }
fn main {
    say(add(1, 2))
    xs := [1, 2, 3]
    for x in xs { say(x) }
}
`)
	if err := ValidateProgram(prog); err != nil {
		t.Fatal(err)
	}
}

func TestValidateChunkBadConst(t *testing.T) {
	ch := &Chunk{
		Name:    "bad",
		Code:    []byte{byte(OpLoadConst), 0, 5, byte(OpReturn)}, // const index 5 missing
		Consts:  []any{runtime.Int(1)},
		NumLocs: 0,
		Lines:   []int{1, 1, 1, 1},
	}
	if err := ValidateChunk(ch); err == nil {
		t.Fatal("want const OOB error")
	}
}

func TestValidateChunkBadJump(t *testing.T) {
	// JUMP with huge positive offset
	ch := &Chunk{
		Name:    "jmp",
		Code:    []byte{byte(OpJump), 0x7f, 0xff, byte(OpReturn)},
		NumLocs: 0,
		Lines:   []int{1, 1, 1, 1},
	}
	if err := ValidateChunk(ch); err == nil {
		t.Fatal("want jump OOB error")
	}
}

func TestValidateChunkUnknownOp(t *testing.T) {
	ch := &Chunk{
		Name:    "u",
		Code:    []byte{0xff},
		NumLocs: 0,
		Lines:   []int{1},
	}
	if err := ValidateChunk(ch); err == nil {
		t.Fatal("want unknown opcode")
	}
}

func TestValidateChunkTruncated(t *testing.T) {
	ch := &Chunk{
		Name:    "t",
		Code:    []byte{byte(OpLoadConst), 0}, // missing low byte
		NumLocs: 0,
		Lines:   []int{1, 1},
	}
	if err := ValidateChunk(ch); err == nil {
		t.Fatal("want truncated operand")
	}
}

func TestValidateChunkMisalignedJump(t *testing.T) {
	// JUMP +1 lands mid LOAD_CONST operand
	ch := &Chunk{
		Name: "misalign",
		Code: []byte{
			byte(OpJump), 0, 1,
			byte(OpLoadConst), 0, 0,
			byte(OpReturn),
		},
		Consts:  []any{},
		NumLocs: 0,
		Lines:   []int{1, 1, 1, 1, 1, 1, 1},
	}
	if err := ValidateChunk(ch); err == nil {
		t.Fatal("want misaligned jump rejected")
	}
}

func TestValidateChunkLocalWithZeroLocs(t *testing.T) {
	ch := &Chunk{
		Name:    "loc",
		Code:    []byte{byte(OpLoadLocal), 0, 0, byte(OpReturn)},
		NumLocs: 0,
		Lines:   []int{1, 1, 1, 1},
	}
	if err := ValidateChunk(ch); err == nil {
		t.Fatal("want local OOB with NumLocs=0")
	}
}
