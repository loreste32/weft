package stdlib

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/loreste/weft/internal/netsafe"
	"github.com/loreste/weft/internal/runtime"
)

// wsConnect is added to the ws package for client-side WebSocket connections.
//
//	conn := ws.connect("ws://localhost:8080/ws")?
//	conn.send("hello")?
//	msg := conn.recv()?
//	conn.close()
func wsConnectBuiltin(env *runtime.Env) runtime.Value {
	return runtime.MakeBuiltin("ws.connect", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("ws.connect(url)", "ws"), nil
		}
		rawURL := args[0].String()
		conn, err := wsClientDial(rawURL)
		if err != nil {
			return errRes(err.Error(), "ws"), nil
		}
		return runtime.Ok(wrapWSClientConn(conn)), nil
	})
}

type wsClientConn struct {
	mu   sync.Mutex
	conn net.Conn
	br   *bufio.Reader
}

func wsClientDial(rawURL string) (*wsClientConn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		if u.Scheme == "wss" {
			host += ":443"
		} else {
			host += ":80"
		}
	}
	// SSRF check: block private/metadata IPs
	if err := netsafe.CheckHost(u.Hostname()); err != nil {
		return nil, fmt.Errorf("ws.connect rejected: %w", err)
	}
	conn, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		return nil, err
	}
	// Generate WebSocket key
	keyBytes := make([]byte, 16)
	rand.Read(keyBytes)
	key := base64.StdEncoding.EncodeToString(keyBytes)

	path := u.Path
	if path == "" {
		path = "/"
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}

	// Send upgrade request
	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n",
		path, u.Host, key)
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, err
	}

	// Read response
	br := bufio.NewReader(conn)
	statusLine, err := br.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, err
	}
	if !strings.Contains(statusLine, "101") {
		conn.Close()
		return nil, fmt.Errorf("websocket upgrade failed: %s", strings.TrimSpace(statusLine))
	}
	// Read headers until blank line
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, err
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	return &wsClientConn{conn: conn, br: br}, nil
}

func wrapWSClientConn(wsc *wsClientConn) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("ws."+name, arity, fn)
	}

	put("send", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("conn.send(msg)", "ws"), nil
		}
		msg := []byte(args[0].String())
		if err := wsClientWrite(wsc, msg); err != nil {
			return errRes(err.Error(), "ws"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})

	put("recv", 0, func(args []runtime.Value) (runtime.Value, error) {
		msg, err := wsClientRead(wsc)
		if err != nil {
			return errRes(err.Error(), "ws"), nil
		}
		return runtime.Ok(runtime.Str(string(msg))), nil
	})

	put("close", 0, func(args []runtime.Value) (runtime.Value, error) {
		wsc.mu.Lock()
		defer wsc.mu.Unlock()
		wsc.conn.Close()
		return runtime.Ok(runtime.Unit()), nil
	})

	return m
}

func wsClientWrite(wsc *wsClientConn, payload []byte) error {
	wsc.mu.Lock()
	defer wsc.mu.Unlock()

	// Client frames must be masked (RFC 6455)
	maskKey := make([]byte, 4)
	rand.Read(maskKey)

	n := len(payload)
	var frame []byte
	frame = append(frame, 0x81) // FIN + text opcode
	if n < 126 {
		frame = append(frame, byte(n)|0x80) // masked
	} else if n < 65536 {
		frame = append(frame, 126|0x80)
		frame = append(frame, byte(n>>8), byte(n))
	} else {
		frame = append(frame, 127|0x80)
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(n))
		frame = append(frame, b...)
	}
	frame = append(frame, maskKey...)
	masked := make([]byte, n)
	for i, b := range payload {
		masked[i] = b ^ maskKey[i%4]
	}
	frame = append(frame, masked...)
	_, err := wsc.conn.Write(frame)
	return err
}

func wsClientRead(wsc *wsClientConn) ([]byte, error) {
	// Read frame header
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(wsc.br, hdr); err != nil {
		return nil, err
	}
	opcode := hdr[0] & 0x0F
	if opcode == 0x08 {
		return nil, fmt.Errorf("connection closed")
	}
	masked := (hdr[1] & 0x80) != 0
	length := int64(hdr[1] & 0x7F)
	if length == 126 {
		ext := make([]byte, 2)
		if _, err := io.ReadFull(wsc.br, ext); err != nil {
			return nil, err
		}
		length = int64(binary.BigEndian.Uint16(ext))
	} else if length == 127 {
		ext := make([]byte, 8)
		if _, err := io.ReadFull(wsc.br, ext); err != nil {
			return nil, err
		}
		length = int64(binary.BigEndian.Uint64(ext))
	}
	var maskKey []byte
	if masked {
		maskKey = make([]byte, 4)
		if _, err := io.ReadFull(wsc.br, maskKey); err != nil {
			return nil, err
		}
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(wsc.br, payload); err != nil {
		return nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}
	return payload, nil
}

// wsAcceptKey computes the Sec-WebSocket-Accept header value.
func wsAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
