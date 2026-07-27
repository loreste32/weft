package stdlib

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// Minimal RFC6455 server WebSocket (text + close). Pure Go, no deps.

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

type wsConn struct {
	mu     sync.Mutex
	rwc    net.Conn
	bufr   *bufio.Reader
	closed bool
}

func isWebSocketRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (*wsConn, error) {
	if r.Method != http.MethodGet {
		return nil, fmt.Errorf("websocket: GET required")
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, fmt.Errorf("websocket: missing Sec-WebSocket-Key")
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("websocket: hijack not supported")
	}
	sum := sha1.Sum([]byte(key + wsGUID))
	accept := base64.StdEncoding.EncodeToString(sum[:])

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := bufrw.WriteString(resp); err != nil {
		conn.Close()
		return nil, err
	}
	if err := bufrw.Flush(); err != nil {
		conn.Close()
		return nil, err
	}
	// use remaining buffered + conn
	return &wsConn{rwc: conn, bufr: bufrw.Reader}, nil
}

func (c *wsConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	// best-effort close frame
	_ = c.writeFrameLocked(0x8, nil)
	return c.rwc.Close()
}

func (c *wsConn) SendText(s string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("websocket closed")
	}
	return c.writeFrameLocked(0x1, []byte(s))
}

func (c *wsConn) writeFrameLocked(opcode byte, payload []byte) error {
	// server→client: unmasked
	hdr := []byte{0x80 | opcode} // FIN + opcode
	n := len(payload)
	switch {
	case n < 126:
		hdr = append(hdr, byte(n))
	case n < 65536:
		hdr = append(hdr, 126, byte(n>>8), byte(n))
	default:
		hdr = append(hdr, 127)
		var lenb [8]byte
		binary.BigEndian.PutUint64(lenb[:], uint64(n))
		hdr = append(hdr, lenb[:]...)
	}
	if err := c.rwc.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return err
	}
	if _, err := c.rwc.Write(hdr); err != nil {
		return err
	}
	if n > 0 {
		_, err := c.rwc.Write(payload)
		return err
	}
	return nil
}

func (c *wsConn) RecvText() (string, error) {
	for {
		opcode, payload, err := c.readFrame()
		if err != nil {
			return "", err
		}
		switch opcode {
		case 0x1: // text
			return string(payload), nil
		case 0x2: // binary — return as string of bytes
			return string(payload), nil
		case 0x8: // close
			c.mu.Lock()
			c.closed = true
			c.mu.Unlock()
			return "", io.EOF
		case 0x9: // ping → pong
			c.mu.Lock()
			_ = c.writeFrameLocked(0xA, payload)
			c.mu.Unlock()
		case 0xA: // pong
			continue
		default:
			continue
		}
	}
}

func (c *wsConn) readFrame() (opcode byte, payload []byte, err error) {
	_ = c.rwc.SetReadDeadline(time.Now().Add(24 * time.Hour))
	h := make([]byte, 2)
	if _, err = io.ReadFull(c.bufr, h); err != nil {
		return 0, nil, err
	}
	opcode = h[0] & 0x0f
	masked := h[1]&0x80 != 0
	n := int(h[1] & 0x7f)
	switch n {
	case 126:
		var ext [2]byte
		if _, err = io.ReadFull(c.bufr, ext[:]); err != nil {
			return 0, nil, err
		}
		n = int(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err = io.ReadFull(c.bufr, ext[:]); err != nil {
			return 0, nil, err
		}
		n64 := binary.BigEndian.Uint64(ext[:])
		if n64 > maxBodyBytes {
			return 0, nil, fmt.Errorf("websocket frame too large")
		}
		n = int(n64)
	}
	var mask [4]byte
	if masked {
		if _, err = io.ReadFull(c.bufr, mask[:]); err != nil {
			return 0, nil, err
		}
	}
	payload = make([]byte, n)
	if n > 0 {
		if _, err = io.ReadFull(c.bufr, payload); err != nil {
			return 0, nil, err
		}
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return opcode, payload, nil
}

func newWSConnValue(env *runtime.Env, c *wsConn, params map[string]string, r *http.Request) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("ws."+name, arity, fn)
	}
	// fields
	mo.Keys = append(mo.Keys, "path", "params", "query")
	mo.Vals["path"] = runtime.Str(r.URL.Path)
	mo.Vals["params"] = stringMapValue(params)
	mo.Vals["query"] = runtime.Str(r.URL.RawQuery)

	put("send", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("conn.send(msg)", "ws"), nil
		}
		if err := c.SendText(args[0].String()); err != nil {
			return errRes(err.Error(), "ws"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})
	put("recv", 0, func(args []runtime.Value) (runtime.Value, error) {
		s, err := c.RecvText()
		if err != nil {
			if err == io.EOF {
				return errRes("closed", "ws"), nil
			}
			return errRes(err.Error(), "ws"), nil
		}
		return runtime.Ok(runtime.Str(s)), nil
	})
	put("close", 0, func(args []runtime.Value) (runtime.Value, error) {
		_ = c.Close()
		return runtime.Ok(runtime.Unit()), nil
	})
	return m
}

// packageWS exposes standalone helpers if needed.
func packageWS(env *runtime.Env) runtime.Value {
	p := pkg()
	// ws is primarily used via app.ws; keep package for discoverability
	set(p, "ok", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Str("use app.ws(path, handler) — conn.send / conn.recv / conn.close"), nil
	}, 0)
	return p
}
