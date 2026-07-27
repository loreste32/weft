package runtime

import (
	"fmt"
	"time"
)

// ChanObj is a typed Weft channel (carries Values; send deep-copies).
type ChanObj struct {
	ch     chan Value
	closed bool
}

// MakeChannel creates a buffered channel (cap 0 = unbuffered).
func MakeChannel(buffer int) Value {
	if buffer < 0 {
		buffer = 0
	}
	return Value{Kind: KindStruct, Obj: &StructObj{
		TypeName: "channel",
		Fields: map[string]Value{
			"__ch": {Kind: KindIter, Obj: &ChanObj{ch: make(chan Value, buffer)}},
		},
		Order: []string{"__ch"},
	}}
}

func asChan(v Value) (*ChanObj, error) {
	if v.Kind == KindStruct {
		so := v.Obj.(*StructObj)
		if so.TypeName == "channel" {
			if raw, ok := so.Fields["__ch"]; ok {
				if c, ok := raw.Obj.(*ChanObj); ok {
					return c, nil
				}
			}
		}
	}
	// also allow direct ChanObj stored somehow
	if c, ok := v.Obj.(*ChanObj); ok {
		return c, nil
	}
	return nil, fmt.Errorf("not a channel")
}

// ChannelSend sends a deep-copied value.
func ChannelSend(ch Value, v Value) error {
	c, err := asChan(ch)
	if err != nil {
		return err
	}
	if c.closed {
		return fmt.Errorf("send on closed channel")
	}
	c.ch <- DeepCopy(v)
	return nil
}

// ChannelRecv receives a value (blocks).
func ChannelRecv(ch Value) (Value, error) {
	c, err := asChan(ch)
	if err != nil {
		return Null(), err
	}
	v, ok := <-c.ch
	if !ok {
		return Null(), fmt.Errorf("recv on closed channel")
	}
	return v, nil
}

// ChannelClose closes the channel.
func ChannelClose(ch Value) error {
	c, err := asChan(ch)
	if err != nil {
		return err
	}
	if !c.closed {
		c.closed = true
		close(c.ch)
	}
	return nil
}

// ChannelTryRecv non-blocking receive. ok=false if empty.
func ChannelTryRecv(ch Value) (Value, bool, error) {
	c, err := asChan(ch)
	if err != nil {
		return Null(), false, err
	}
	select {
	case v, ok := <-c.ch:
		if !ok {
			return Null(), false, nil
		}
		return v, true, nil
	default:
		return Null(), false, nil
	}
}

// SelectRecv waits on the first of several channels; returns (index, value).
func SelectRecv(chs []Value, timeoutMs int64) (int, Value, error) {
	if len(chs) == 0 {
		return -1, Null(), fmt.Errorf("select_recv: empty")
	}
	type result struct {
		i int
		v Value
	}
	cases := make([]<-chan Value, len(chs))
	for i, ch := range chs {
		c, err := asChan(ch)
		if err != nil {
			return -1, Null(), err
		}
		cases[i] = c.ch
	}
	// dynamic select via goroutines + done
	out := make(chan result, 1)
	done := make(chan struct{})
	for i, ch := range cases {
		go func(i int, ch <-chan Value) {
			select {
			case v, ok := <-ch:
				if !ok {
					return
				}
				select {
				case out <- result{i: i, v: v}:
				case <-done:
				}
			case <-done:
			}
		}(i, ch)
	}
	if timeoutMs > 0 {
		select {
		case r := <-out:
			close(done)
			return r.i, r.v, nil
		case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
			close(done)
			return -1, Null(), fmt.Errorf("select_recv: timeout")
		}
	}
	r := <-out
	close(done)
	return r.i, r.v, nil
}
