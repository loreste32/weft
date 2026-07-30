package runtime

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

// ChanObj is a typed Weft channel (carries Values; send deep-copies).
type ChanObj struct {
	ch     chan Value
	mu     sync.RWMutex
	closed atomic.Bool
	done   chan struct{}
}

// MakeChannel creates a buffered channel (cap 0 = unbuffered).
func MakeChannel(buffer int) Value {
	if buffer < 0 {
		buffer = 0
	}
	return Value{Kind: KindStruct, Obj: &StructObj{
		TypeName: "channel",
		Fields: map[string]Value{
			"__ch": {Kind: KindIter, Obj: &ChanObj{
				ch:   make(chan Value, buffer),
				done: make(chan struct{}),
			}},
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
	copyValue := DeepCopy(v)
	if c.closed.Load() {
		return fmt.Errorf("send on closed channel")
	}
	c.mu.RLock()
	if c.closed.Load() {
		c.mu.RUnlock()
		return fmt.Errorf("send on closed channel")
	}
	defer c.mu.RUnlock()
	select {
	case c.ch <- copyValue:
		return nil
	case <-c.done:
		return fmt.Errorf("send on closed channel")
	}
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
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(c.done)

	// Wait for any in-flight senders (which observe done and release their
	// read lock), then close the data channel without racing a send.
	c.mu.Lock()
	defer c.mu.Unlock()
	close(c.ch)
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
	type activeChannel struct {
		index int
		ch    <-chan Value
	}
	active := make([]activeChannel, 0, len(chs))
	for i, ch := range chs {
		c, err := asChan(ch)
		if err != nil {
			return -1, Null(), err
		}
		active = append(active, activeChannel{index: i, ch: c.ch})
	}

	var timer *time.Timer
	if timeoutMs > 0 {
		timer = time.NewTimer(time.Duration(timeoutMs) * time.Millisecond)
		defer timer.Stop()
	}

	// reflect.Select is the safe dynamic equivalent of a Go select over a
	// runtime-sized channel list. Closed channels are removed and, when all
	// channels are closed, the operation returns instead of waiting forever.
	for len(active) > 0 {
		cases := make([]reflect.SelectCase, 0, len(active)+1)
		for _, candidate := range active {
			cases = append(cases, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(candidate.ch),
			})
		}
		if timer != nil {
			cases = append(cases, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(timer.C),
			})
		}

		chosen, value, ok := reflect.Select(cases)
		if timer != nil && chosen == len(cases)-1 {
			return -1, Null(), fmt.Errorf("select_recv: timeout")
		}
		if !ok {
			active = append(active[:chosen], active[chosen+1:]...)
			continue
		}
		return active[chosen].index, value.Interface().(Value), nil
	}
	return -1, Null(), fmt.Errorf("select_recv: all channels closed")
}
