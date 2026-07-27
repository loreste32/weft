package stdlib

import (
	"encoding/binary"
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func TestBinstructPackUnpackRoundTrip(t *testing.T) {
	b, err := bsPack(">I", []runtime.Value{runtime.Int(0x01020304)})
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 4 {
		t.Fatalf("len %d", len(b))
	}
	if binary.BigEndian.Uint32(b) != 0x01020304 {
		t.Fatalf("%x", b)
	}
	vals, err := bsUnpack(">I", b)
	if err != nil {
		t.Fatal(err)
	}
	if vals[0].I != 0x01020304 {
		t.Fatalf("%v", vals[0])
	}
	n, err := bsSize(">IH4s")
	if err != nil || n != 10 {
		t.Fatalf("size %d %v", n, err)
	}
	// little endian
	b2, err := bsPack("<H", []runtime.Value{runtime.Int(0x3412)})
	if err != nil {
		t.Fatal(err)
	}
	v2, err := bsUnpack("<H", b2)
	if err != nil || v2[0].I != 0x3412 {
		t.Fatalf("%v %v", v2, err)
	}
}

func TestBinstructBadLayout(t *testing.T) {
	if _, err := bsSize(">Z"); err == nil {
		t.Fatal("expected error for bad code")
	}
	if _, err := bsSize("3"); err == nil {
		t.Fatal("expected trailing count error")
	}
	if _, err := bsPack(">I", nil); err == nil {
		t.Fatal("expected not enough values")
	}
	if _, err := bsUnpack(">I", []byte{1}); err == nil {
		t.Fatal("expected short buffer")
	}
}
