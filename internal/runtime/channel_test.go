package runtime_test

import (
	"testing"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

func TestChannelSendRecv(t *testing.T) {
	ch := runtime.MakeChannel(1)
	if err := runtime.ChannelSend(ch, runtime.Int(42)); err != nil {
		t.Fatal(err)
	}
	v, err := runtime.ChannelRecv(ch)
	if err != nil {
		t.Fatal(err)
	}
	if v.I != 42 {
		t.Fatalf("got %d, want 42", v.I)
	}
}

func TestChannelDeepCopiesOnSend(t *testing.T) {
	ch := runtime.MakeChannel(1)
	list := runtime.List(runtime.Int(1))
	if err := runtime.ChannelSend(ch, list); err != nil {
		t.Fatal(err)
	}
	// mutate original
	list.Obj.(*runtime.ListObj).Items[0] = runtime.Int(99)
	v, _ := runtime.ChannelRecv(ch)
	if v.Obj.(*runtime.ListObj).Items[0].I != 1 {
		t.Fatal("send should deep-copy")
	}
}

func TestChannelClose(t *testing.T) {
	ch := runtime.MakeChannel(0)
	if err := runtime.ChannelClose(ch); err != nil {
		t.Fatal(err)
	}
	// send on closed
	if err := runtime.ChannelSend(ch, runtime.Int(1)); err == nil {
		t.Fatal("send on closed should error")
	}
	// recv on closed
	_, err := runtime.ChannelRecv(ch)
	if err == nil {
		t.Fatal("recv on closed should error")
	}
	// double close is safe
	if err := runtime.ChannelClose(ch); err != nil {
		t.Fatal("double close should be safe")
	}
}

func TestChannelTryRecv(t *testing.T) {
	ch := runtime.MakeChannel(1)
	// empty
	_, ok, err := runtime.ChannelTryRecv(ch)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("should be empty")
	}
	// with value
	runtime.ChannelSend(ch, runtime.Int(7))
	v, ok, err := runtime.ChannelTryRecv(ch)
	if err != nil || !ok || v.I != 7 {
		t.Fatal("try_recv with value")
	}
	// closed + empty
	runtime.ChannelClose(ch)
	_, ok, err = runtime.ChannelTryRecv(ch)
	if err != nil || ok {
		t.Fatal("try_recv on closed empty should return ok=false, no error")
	}
}

func TestChannelNotAChannel(t *testing.T) {
	v := runtime.Int(5)
	if err := runtime.ChannelSend(v, runtime.Int(1)); err == nil {
		t.Fatal("send to non-channel should error")
	}
	if _, err := runtime.ChannelRecv(v); err == nil {
		t.Fatal("recv from non-channel should error")
	}
	if err := runtime.ChannelClose(v); err == nil {
		t.Fatal("close non-channel should error")
	}
	if _, _, err := runtime.ChannelTryRecv(v); err == nil {
		t.Fatal("try_recv non-channel should error")
	}
}

func TestSelectRecv(t *testing.T) {
	ch1 := runtime.MakeChannel(1)
	ch2 := runtime.MakeChannel(1)
	runtime.ChannelSend(ch2, runtime.Str("hello"))

	i, v, err := runtime.SelectRecv([]runtime.Value{ch1, ch2}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if i != 1 || v.S != "hello" {
		t.Fatalf("select_recv: i=%d, v=%v", i, v)
	}
}

func TestSelectRecvTimeout(t *testing.T) {
	ch := runtime.MakeChannel(0)
	_, _, err := runtime.SelectRecv([]runtime.Value{ch}, 10) // 10ms
	if err == nil {
		t.Fatal("should timeout")
	}
}

func TestSelectRecvEmpty(t *testing.T) {
	_, _, err := runtime.SelectRecv(nil, 0)
	if err == nil {
		t.Fatal("empty should error")
	}
}

func TestSelectRecvNotChannel(t *testing.T) {
	_, _, err := runtime.SelectRecv([]runtime.Value{runtime.Int(1)}, 0)
	if err == nil {
		t.Fatal("non-channel should error")
	}
}

func TestMakeChannelNegativeBuffer(t *testing.T) {
	ch := runtime.MakeChannel(-1)
	// should not panic; buffer clamped to 0
	go func() {
		runtime.ChannelSend(ch, runtime.Int(1))
	}()
	time.Sleep(10 * time.Millisecond)
	v, err := runtime.ChannelRecv(ch)
	if err != nil || v.I != 1 {
		t.Fatal("negative buffer channel")
	}
}
