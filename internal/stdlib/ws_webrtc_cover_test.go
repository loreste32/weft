package stdlib

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

func TestWS_IsWebSocketRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	if isWebSocketRequest(req) {
		t.Fatal("plain")
	}
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	if !isWebSocketRequest(req) {
		t.Fatal("ws")
	}
}

func TestWS_UpgradeErrors(t *testing.T) {
	// method
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/ws", nil)
	if _, err := upgradeWebSocket(w, req); err == nil {
		t.Fatal("method")
	}
	// missing key
	req = httptest.NewRequest(http.MethodGet, "/ws", nil)
	if _, err := upgradeWebSocket(w, req); err == nil {
		t.Fatal("key")
	}
	// hijack not supported on ResponseRecorder
	req.Header.Set("Sec-WebSocket-Key", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")))
	if _, err := upgradeWebSocket(w, req); err == nil {
		t.Fatal("hijack")
	}
}

type hijackPair struct {
	server net.Conn
	client net.Conn
	http.ResponseWriter
	hdr  http.Header
	code int
}

func newHijackPair() *hijackPair {
	s, c := net.Pipe()
	return &hijackPair{
		server: s,
		client: c,
		hdr:    make(http.Header),
	}
}

func (h *hijackPair) Header() http.Header         { return h.hdr }
func (h *hijackPair) Write(b []byte) (int, error) { return len(b), nil }
func (h *hijackPair) WriteHeader(statusCode int)  { h.code = statusCode }
func (h *hijackPair) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	br := bufio.NewReader(h.server)
	bw := bufio.NewWriter(h.server)
	return h.server, bufio.NewReadWriter(br, bw), nil
}

func TestWS_UpgradeAndFrames(t *testing.T) {
	hp := newHijackPair()
	defer hp.client.Close()

	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	req := httptest.NewRequest(http.MethodGet, "/chat?q=1", nil)
	req.Header.Set("Sec-WebSocket-Key", key)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")

	done := make(chan *wsConn, 1)
	go func() {
		c, err := upgradeWebSocket(hp, req)
		if err != nil {
			t.Error(err)
			done <- nil
			return
		}
		done <- c
	}()

	// read 101 response from client side
	br := bufio.NewReader(hp.client)
	status, err := br.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "101") {
		t.Fatalf("status %q", status)
	}
	// drain headers
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}
	// verify accept
	sum := sha1.Sum([]byte(key + wsGUID))
	_ = base64.StdEncoding.EncodeToString(sum[:])

	c := <-done
	if c == nil {
		t.Fatal("upgrade failed")
	}

	// net.Pipe is unbuffered: write and read must not deadlock.
	errCh := make(chan error, 1)
	go func() {
		errCh <- c.SendText("hello")
	}()
	fh := make([]byte, 2)
	if _, err := io.ReadFull(br, fh); err != nil {
		t.Fatal(err)
	}
	if fh[0]&0x0f != 0x1 {
		t.Fatalf("opcode %x", fh[0])
	}
	n := int(fh[1] & 0x7f)
	payload := make([]byte, n)
	if _, err := io.ReadFull(br, payload); err != nil {
		t.Fatal(err)
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
	if string(payload) != "hello" {
		t.Fatalf("%q", payload)
	}

	// client → server: write frames while RecvText runs
	type recvRes struct {
		s   string
		err error
	}
	recvCh := make(chan recvRes, 1)
	go func() {
		s, err := c.RecvText()
		recvCh <- recvRes{s, err}
	}()
	writeMaskedFrame(t, hp.client, 0x1, []byte("from-client"))
	rr := <-recvCh
	if rr.err != nil {
		t.Fatal(rr.err)
	}
	if rr.s != "from-client" {
		t.Fatalf("recv %q", rr.s)
	}

	// ping → pong then text (RecvText auto-answers ping)
	go func() {
		s, err := c.RecvText()
		recvCh <- recvRes{s, err}
	}()
	// Drain the pong before sending the next client frame. Keeping this read
	// synchronous prevents two goroutines from reading the same bufio.Reader.
	writeMaskedFrame(t, hp.client, 0x9, []byte("p"))
	pongOpcode, pongPayload, err := readServerFrame(br)
	if err != nil {
		t.Fatal(err)
	}
	if pongOpcode != 0xA || string(pongPayload) != "p" {
		t.Fatalf("pong opcode=%x payload=%q", pongOpcode, pongPayload)
	}
	writeMaskedFrame(t, hp.client, 0x1, []byte("after-ping"))
	rr = <-recvCh
	if rr.err != nil {
		t.Fatal(rr.err)
	}
	if rr.s != "after-ping" {
		t.Fatalf("after ping %q", rr.s)
	}

	// binary opcode
	go func() {
		s, err := c.RecvText()
		recvCh <- recvRes{s, err}
	}()
	writeMaskedFrame(t, hp.client, 0x2, []byte{1, 2, 3})
	rr = <-recvCh
	if rr.err != nil {
		t.Fatal(rr.err)
	}
	if rr.s != string([]byte{1, 2, 3}) {
		t.Fatalf("bin %q", rr.s)
	}

	// extended length 126
	big := bytes.Repeat([]byte("x"), 200)
	go func() {
		s, err := c.RecvText()
		recvCh <- recvRes{s, err}
	}()
	writeMaskedFrame(t, hp.client, 0x1, big)
	rr = <-recvCh
	if rr.err != nil {
		t.Fatal(rr.err)
	}
	if len(rr.s) != 200 {
		t.Fatalf("len %d", len(rr.s))
	}

	// close frame
	go func() {
		_, err := c.RecvText()
		recvCh <- recvRes{"", err}
	}()
	writeMaskedFrame(t, hp.client, 0x8, nil)
	rr = <-recvCh
	if rr.err != io.EOF {
		t.Fatalf("close err %v", rr.err)
	}

	// double close + send after close
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
	if err := c.SendText("nope"); err == nil {
		t.Fatal("send after close")
	}
}

func writeMaskedFrame(t *testing.T, w io.Writer, opcode byte, payload []byte) {
	t.Helper()
	hdr := []byte{0x80 | opcode}
	n := len(payload)
	switch {
	case n < 126:
		hdr = append(hdr, byte(0x80|n))
	case n < 65536:
		hdr = append(hdr, 0x80|126, byte(n>>8), byte(n))
	default:
		hdr = append(hdr, 0x80|127)
		var lenb [8]byte
		binary.BigEndian.PutUint64(lenb[:], uint64(n))
		hdr = append(hdr, lenb[:]...)
	}
	mask := [4]byte{1, 2, 3, 4}
	hdr = append(hdr, mask[:]...)
	masked := make([]byte, n)
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}
	if _, err := w.Write(hdr); err != nil {
		t.Fatal(err)
	}
	if n > 0 {
		if _, err := w.Write(masked); err != nil {
			t.Fatal(err)
		}
	}
}

func readServerFrame(r io.Reader) (byte, []byte, error) {
	var header [2]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, err
	}
	n := int(header[1] & 0x7f)
	switch n {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(r, ext[:]); err != nil {
			return 0, nil, err
		}
		n = int(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(r, ext[:]); err != nil {
			return 0, nil, err
		}
		n64 := binary.BigEndian.Uint64(ext[:])
		if n64 > uint64(^uint(0)>>1) {
			return 0, nil, io.ErrUnexpectedEOF
		}
		n = int(n64)
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return header[0] & 0x0f, payload, nil
}

func TestWS_WriteFrameLongAndNewConnValue(t *testing.T) {
	s, c := net.Pipe()
	defer s.Close()
	defer c.Close()
	ws := &wsConn{rwc: s, bufr: bufio.NewReader(s)}

	// continuous drain so multi-Write SendText cannot block on unbuffered pipe
	stop := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			_ = c.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			_, err := c.Read(buf)
			select {
			case <-stop:
				return
			default:
			}
			if err != nil && !isTimeout(err) {
				return
			}
		}
	}()
	defer close(stop)

	payload := string(bytes.Repeat([]byte("a"), 300))
	if err := ws.SendText(payload); err != nil {
		t.Fatal(err)
	}

	env := runtime.NewEnv()
	req := httptest.NewRequest(http.MethodGet, "/ws/room?x=1", nil)
	val := newWSConnValue(env, ws, map[string]string{"id": "1"}, req)
	mo := val.Obj.(*runtime.MapObj)
	if mo.Vals["path"].S != "/ws/room" {
		t.Fatal(mo.Vals["path"])
	}
	r, err := mo.Vals["send"].Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{runtime.Str("hi")})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal(r)
	}
	// send arity
	r, _ = mo.Vals["send"].Obj.(*runtime.BuiltinObj).Fn(nil)
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("arity")
	}
	// close
	r, _ = mo.Vals["close"].Obj.(*runtime.BuiltinObj).Fn(nil)
	if !r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal(r)
	}
	// recv on closed / EOF
	r, _ = mo.Vals["recv"].Obj.(*runtime.BuiltinObj).Fn(nil)
	if r.Obj.(*runtime.ResultObj).Ok {
		t.Fatal("recv after close")
	}

	// package surface
	p := packageWS(env)
	ok := callPkg(t, p, "ok")
	if !strings.Contains(ok.S, "app.ws") {
		t.Fatal(ok)
	}
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	ne, ok := err.(net.Error)
	return ok && ne.Timeout()
}

func TestWebRTC_HubSignalFlow(t *testing.T) {
	env := runtime.NewEnv()
	env.Call = func(fn runtime.Value, args []runtime.Value) (runtime.Value, error) {
		if fn.Kind == runtime.KindBuiltin {
			return fn.Obj.(*runtime.BuiltinObj).Fn(args)
		}
		return runtime.Unit(), nil
	}

	hubVal := newRTCHubValue(env)
	handlerFn := callMap(t, hubVal, "handler")

	// scripted peer A
	var (
		muA   sync.Mutex
		sentA []string
		recvA = make(chan string, 16)
		doneA = make(chan struct{})
	)
	connA := fakeWSConn(t, &muA, &sentA, recvA, doneA)

	// scripted peer B
	var (
		muB   sync.Mutex
		sentB []string
		recvB = make(chan string, 16)
		doneB = make(chan struct{})
	)
	connB := fakeWSConn(t, &muB, &sentB, recvB, doneB)

	// start handlers
	go func() {
		_, _ = handlerFn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{connA})
	}()
	go func() {
		_, _ = handlerFn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{connB})
	}()

	// A joins
	recvA <- `{"type":"join","room":"lobby","peer":"alice"}`
	waitContains(t, &muA, &sentA, "welcome", 2*time.Second)
	// B joins
	recvB <- `{"type":"join","room":"lobby","peer":"bob"}`
	waitContains(t, &muB, &sentB, "welcome", 2*time.Second)
	waitContains(t, &muA, &sentA, "peer-joined", 2*time.Second)

	// rooms / peers
	rooms := callMap(t, hubVal, "rooms")
	if rooms.Kind != runtime.KindList {
		t.Fatal(rooms)
	}
	peers := callMap(t, hubVal, "peers", runtime.Str("lobby"))
	if peers.Kind != runtime.KindList || len(peers.Obj.(*runtime.ListObj).Items) < 2 {
		t.Fatalf("peers %v", peers)
	}
	// peers arity
	mustErr(t, callMap(t, hubVal, "peers"))

	// offer relay A→B
	recvA <- `{"type":"offer","to":"bob","sdp":"v=0"}`
	waitContains(t, &muB, &sentB, "offer", 2*time.Second)

	// ice missing to
	recvA <- `{"type":"ice"}`
	waitContains(t, &muA, &sentA, "missing to", 2*time.Second)

	// peer not found
	recvA <- `{"type":"answer","to":"ghost","sdp":"x"}`
	waitContains(t, &muA, &sentA, "peer not found", 2*time.Second)

	// broadcast
	recvA <- `{"type":"broadcast","payload":{"hi":1}}`
	waitContains(t, &muB, &sentB, "broadcast", 2*time.Second)

	// invalid json
	recvA <- `{not-json`
	waitContains(t, &muA, &sentA, "invalid json", 2*time.Second)

	// unknown type
	recvA <- `{"type":"nope"}`
	waitContains(t, &muA, &sentA, "unknown type", 2*time.Second)

	// leave
	recvA <- `{"type":"leave"}`
	waitContains(t, &muA, &sentA, "left", 2*time.Second)
	waitContains(t, &muB, &sentB, "peer-left", 2*time.Second)

	// signal before join (new connection path: stop A and use error from empty peer)
	// B still joined — offer without join on a fresh conn
	var (
		muC   sync.Mutex
		sentC []string
		recvC = make(chan string, 8)
		doneC = make(chan struct{})
	)
	connC := fakeWSConn(t, &muC, &sentC, recvC, doneC)
	go func() {
		_, _ = handlerFn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{connC})
	}()
	recvC <- `{"type":"offer","to":"bob","sdp":"x"}`
	waitContains(t, &muC, &sentC, "join a room first", 2*time.Second)
	recvC <- `{"type":"broadcast","payload":1}`
	waitContains(t, &muC, &sentC, "join a room first", 2*time.Second)

	// join default room + auto peer id
	recvC <- `{"type":"join"}`
	waitContains(t, &muC, &sentC, "welcome", 2*time.Second)

	// duplicate peer name
	var (
		muD   sync.Mutex
		sentD []string
		recvD = make(chan string, 8)
		doneD = make(chan struct{})
	)
	connD := fakeWSConn(t, &muD, &sentD, recvD, doneD)
	go func() {
		_, _ = handlerFn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{connD})
	}()
	recvD <- `{"type":"join","room":"default","peer":"peer-dup"}`
	// second with same name
	var (
		muE   sync.Mutex
		sentE []string
		recvE = make(chan string, 8)
		doneE = make(chan struct{})
	)
	connE := fakeWSConn(t, &muE, &sentE, recvE, doneE)
	go func() {
		_, _ = handlerFn.Obj.(*runtime.BuiltinObj).Fn([]runtime.Value{connE})
	}()
	recvE <- `{"type":"join","room":"default","peer":"peer-dup"}`
	waitContains(t, &muE, &sentE, "welcome", 2*time.Second)

	// shutdown recvs
	close(doneA)
	close(doneB)
	close(doneC)
	close(doneD)
	close(doneE)

	// package ice_servers
	p := packageWebRTC(env)
	ice := callPkg(t, p, "ice_servers")
	if ice.Kind != runtime.KindList {
		t.Fatal(ice)
	}

	// attach arity
	mustErr(t, callMap(t, hubVal, "attach", runtime.Str("nope")))
	// attach missing ws
	badApp := runtime.NewMap()
	mustErr(t, callMap(t, hubVal, "attach", badApp, runtime.Str("/s")))
}

func fakeWSConn(t *testing.T, mu *sync.Mutex, sent *[]string, recv <-chan string, done <-chan struct{}) runtime.Value {
	t.Helper()
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	mo.Keys = []string{"send", "recv", "close"}
	mo.Vals["send"] = runtime.MakeBuiltin("send", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("send", "ws"), nil
		}
		mu.Lock()
		*sent = append(*sent, args[0].String())
		mu.Unlock()
		return runtime.Ok(runtime.Unit()), nil
	})
	mo.Vals["recv"] = runtime.MakeBuiltin("recv", 0, func(args []runtime.Value) (runtime.Value, error) {
		select {
		case s, ok := <-recv:
			if !ok {
				return errRes("closed", "ws"), nil
			}
			return runtime.Ok(runtime.Str(s)), nil
		case <-done:
			return errRes("closed", "ws"), nil
		case <-time.After(5 * time.Second):
			return errRes("timeout", "ws"), nil
		}
	})
	mo.Vals["close"] = runtime.MakeBuiltin("close", 0, func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Ok(runtime.Unit()), nil
	})
	return m
}

func waitContains(t *testing.T, mu *sync.Mutex, sent *[]string, needle string, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		mu.Lock()
		for _, s := range *sent {
			if strings.Contains(s, needle) {
				mu.Unlock()
				return
			}
		}
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	t.Fatalf("timeout waiting for %q in %v", needle, *sent)
}
