package weft_test

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Adversarial ESL black-box tests: real Weft CLI processes against Go mock
// TCP servers that fragment frames, coalesce frame boundaries, kill the
// connection mid-operation, and flood the dispatcher. All servers run with
// deadlines and all clients with a context timeout so a regression fails
// instead of hanging CI.

const adversarialClientTimeout = 20 * time.Second

func weftCLI(t *testing.T) string {
	t.Helper()
	cli := os.Getenv("WEFT_CLI")
	if cli == "" {
		t.Skip("WEFT_CLI is required for the real Weft ESL process test")
	}
	return cli
}

func eslTestListener(t *testing.T) (net.Listener, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split listener address: %v", err)
	}
	if _, err := strconv.Atoi(port); err != nil {
		t.Fatalf("invalid listener port %q: %v", port, err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return listener, port
}

// runESLClient runs a black-box client script against port and returns its
// combined output. A client that hangs is killed after the context timeout.
func runESLClient(t *testing.T, cli, script, port string, extraEnv ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), adversarialClientTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, cli, "run", script)
	cmd.Dir = repositoryRoot()
	cmd.Env = append(os.Environ(), "ESL_TEST_PORT="+port)
	cmd.Env = append(cmd.Env, extraEnv...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("Weft ESL client %s hung (killed after %s)\n%s", script, adversarialClientTimeout, output)
	}
	if err != nil {
		t.Fatalf("Weft ESL client %s failed: %v\n%s", script, err, output)
	}
	return string(output)
}

func serveOnce(t *testing.T, listener net.Listener, serve func(conn net.Conn) error) <-chan error {
	t.Helper()
	serverErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- fmt.Errorf("accept: %w", err)
			return
		}
		defer func() { _ = conn.Close() }()
		if err := conn.SetDeadline(time.Now().Add(processFixtureTimeout)); err != nil {
			serverErr <- fmt.Errorf("set deadline: %w", err)
			return
		}
		serverErr <- serve(conn)
	}()
	return serverErr
}

func waitServer(t *testing.T, serverErr <-chan error) {
	t.Helper()
	select {
	case err := <-serverErr:
		if err != nil {
			t.Fatalf("ESL mock server failed: %v", err)
		}
	case <-time.After(processFixtureTimeout):
		t.Fatal("timed out waiting for ESL mock server")
	}
}

func eslAuthExchange(conn net.Conn, reader *bufio.Reader) error {
	if err := writeProcessFixture(conn, "Content-Type: auth/request\n\n"); err != nil {
		return err
	}
	auth, err := readUntilProcessFixture(reader, "auth pw")
	if err != nil {
		return fmt.Errorf("read auth: %w", err)
	}
	if !strings.Contains(auth, "auth pw") {
		return fmt.Errorf("unexpected auth command %q", auth)
	}
	return writeProcessFixture(conn, "Content-Type: command/reply\nReply-Text: +OK accepted\n\n")
}

// TestESLServerCloseDuringCommand: the server closes the connection while a
// command reply is still pending. The pending command, every later command,
// and event waiters must all fail — never hang.
func TestESLServerCloseDuringCommand(t *testing.T) {
	cli := weftCLI(t)
	listener, port := eslTestListener(t)

	serverErr := serveOnce(t, listener, func(conn net.Conn) error {
		reader := bufio.NewReader(conn)
		if err := eslAuthExchange(conn, reader); err != nil {
			return err
		}
		if _, err := readUntilProcessFixture(reader, "api freeze"); err != nil {
			return fmt.Errorf("read pending command: %w", err)
		}
		return nil // deferred close kills the connection mid-command
	})

	output := runESLClient(t, cli, "packages/telecom/esl_blackbox_server_close.weft", port)
	for _, marker := range []string{"PENDING-FAILED:", "AFTER-FAILED:", "EVENT-FAILED:"} {
		if !strings.Contains(output, marker) {
			t.Fatalf("missing %q in client output:\n%s", marker, output)
		}
	}
	waitServer(t, serverErr)
}

// TestESLServerCloseDuringEventWait: the server closes the connection while
// the client is blocked in recv_event. The waiter must fail, a later
// recv_event_timeout must fail fast (not run out its clock), and a later
// command must report the dead connection.
func TestESLServerCloseDuringEventWait(t *testing.T) {
	cli := weftCLI(t)
	listener, port := eslTestListener(t)

	serverErr := serveOnce(t, listener, func(conn net.Conn) error {
		reader := bufio.NewReader(conn)
		if err := eslAuthExchange(conn, reader); err != nil {
			return err
		}
		// Give the client time to block in recv_event, then kill the conn.
		time.Sleep(300 * time.Millisecond)
		return nil
	})

	output := runESLClient(t, cli, "packages/telecom/esl_blackbox_server_close.weft", port, "ESL_TEST_MODE=event")
	for _, marker := range []string{"WAIT-FAILED:", "TIMEOUT-FAILED:", "COMMAND-FAILED:"} {
		if !strings.Contains(output, marker) {
			t.Fatalf("missing %q in client output:\n%s", marker, output)
		}
	}
	waitServer(t, serverErr)
}

// TestESLFragmentedFrames: every frame arrives in adversarial chunks —
// byte-by-byte, split mid-header, split mid Content-Length digits, 1-byte
// body dribbles — and frame boundaries are coalesced into shared writes.
func TestESLFragmentedFrames(t *testing.T) {
	cli := weftCLI(t)
	listener, port := eslTestListener(t)

	serverErr := serveOnce(t, listener, func(conn net.Conn) error {
		reader := bufio.NewReader(conn)
		// Dribble the auth request byte by byte.
		for _, b := range []byte("Content-Type: auth/request\n\n") {
			if err := writeProcessFixture(conn, string(b)); err != nil {
				return err
			}
			time.Sleep(2 * time.Millisecond)
		}
		if _, err := readUntilProcessFixture(reader, "auth pw"); err != nil {
			return fmt.Errorf("read auth: %w", err)
		}
		// Auth reply split mid-header.
		for _, chunk := range []string{
			"Content-Type: command/re",
			"ply\nReply-Text: +OK acc",
			"epted\n",
			"\n",
		} {
			if err := writeProcessFixture(conn, chunk); err != nil {
				return err
			}
			time.Sleep(5 * time.Millisecond)
		}
		if _, err := readUntilProcessFixture(reader, "api fragmented"); err != nil {
			return fmt.Errorf("read command: %w", err)
		}

		replyBody := "fragmented-body-ok"
		reply := fmt.Sprintf("Content-Type: command/reply\nReply-Text: +OK frag\nContent-Length: %d\n\n%s", len(replyBody), replyBody)
		event1Body := `{"Event":"FRAGMENTED","seq":1}`
		event1 := fmt.Sprintf("Content-Type: text/event-json\nContent-Length: %d\n\n%s", len(event1Body), event1Body)
		event2Body := `{"Event":"COALESCED"}`
		event2 := fmt.Sprintf("Content-Type: text/event-json\nContent-Length: %d\n\n%s", len(event2Body), event2Body)

		// Split the reply between the two Content-Length digits, dribble the
		// body one byte at a time, and coalesce the reply tail with the head
		// of the first event in a single write.
		digitSplit := strings.Index(reply, "Content-Length: ") + len("Content-Length: ") + 1
		bodyStart := strings.Index(reply, "\n\n") + 2
		body := reply[bodyStart:]
		chunks := []string{reply[:digitSplit], reply[digitSplit:bodyStart]}
		for i := 0; i < 3; i++ {
			chunks = append(chunks, body[i:i+1])
		}
		firstHalf := len(event1) / 2
		chunks = append(chunks,
			body[3:]+event1[:firstHalf], // reply tail + event1 head, one write
			event1[firstHalf:]+event2,   // event1 tail + whole event2, one write
		)
		for _, chunk := range chunks {
			if err := writeProcessFixture(conn, chunk); err != nil {
				return err
			}
			time.Sleep(5 * time.Millisecond)
		}
		// Keep the connection open until the client closes it.
		_, _ = reader.Read(make([]byte, 1))
		return nil
	})

	output := runESLClient(t, cli, "packages/telecom/esl_blackbox_fragmented.weft", port)
	for _, marker := range []string{"REPLY-OK: fragmented-body-ok", "EVENT-OK: FRAGMENTED", "EVENT2-OK: COALESCED"} {
		if !strings.Contains(output, marker) {
			t.Fatalf("missing %q in client output:\n%s", marker, output)
		}
	}
	waitServer(t, serverErr)
}

// TestESLConcurrentCommandStress: 10 concurrent commands each receive their
// own reply, then a 260-command flood against a silent server exercises the
// 256-entry pending-request cap (4 explicit rejections) and the dead-path
// cleanup of the surviving 256 when the server closes.
func TestESLConcurrentCommandStress(t *testing.T) {
	cli := weftCLI(t)
	listener, port := eslTestListener(t)

	serverErr := serveOnce(t, listener, func(conn net.Conn) error {
		reader := bufio.NewReader(conn)
		if err := eslAuthExchange(conn, reader); err != nil {
			return err
		}

		// Collect all 10 stress commands in arrival order.
		var received strings.Builder
		var order []string
		for len(order) < 10 {
			chunk := make([]byte, 4096)
			n, err := reader.Read(chunk)
			if err != nil {
				return fmt.Errorf("read stress commands: %w", err)
			}
			received.Write(chunk[:n])
			order = stressOrder(received.String())
		}
		// Reply in FIFO arrival order, split across two coalesced writes.
		var replies strings.Builder
		for _, name := range order {
			body := name + "-ok"
			fmt.Fprintf(&replies, "Content-Type: command/reply\nReply-Text: +OK %s\nContent-Length: %d\n\n%s", name, len(body), body)
		}
		all := replies.String()
		if err := writeProcessFixture(conn, all[:len(all)/2]); err != nil {
			return err
		}
		time.Sleep(20 * time.Millisecond)
		if err := writeProcessFixture(conn, all[len(all)/2:]); err != nil {
			return err
		}

		// Flood phase: the 4 over-cap commands are rejected client-side and
		// never reach the wire, so exactly 256 floods arrive. Swallow them,
		// give the coordinator a beat to process the rejections, then kill
		// the connection without a single reply.
		seen := 0
		buf := make([]byte, 4096)
		for seen < 256 {
			n, err := reader.Read(buf)
			if err != nil {
				return fmt.Errorf("read flood commands (seen %d): %w", seen, err)
			}
			seen += strings.Count(string(buf[:n]), "flood-")
		}
		time.Sleep(300 * time.Millisecond)
		return nil // deferred close fails the 256 pending commands
	})

	output := runESLClient(t, cli, "packages/telecom/esl_blackbox_stress.weft", port)
	for _, marker := range []string{"STRESS-OK: 10 concurrent commands answered", "FLOOD-OK: full=4 closed=256"} {
		if !strings.Contains(output, marker) {
			t.Fatalf("missing %q in client output:\n%s", marker, output)
		}
	}
	waitServer(t, serverErr)
}

// stressOrder returns the stress command names in their order of first
// appearance in the received byte stream.
func stressOrder(text string) []string {
	type hit struct {
		name string
		idx  int
	}
	hits := make([]hit, 0, 10)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("stress-0%d", i)
		if idx := strings.Index(text, "api "+name); idx >= 0 {
			hits = append(hits, hit{name, idx})
		}
	}
	for i := 0; i < len(hits); i++ {
		for j := i + 1; j < len(hits); j++ {
			if hits[j].idx < hits[i].idx {
				hits[i], hits[j] = hits[j], hits[i]
			}
		}
	}
	order := make([]string, 0, len(hits))
	for _, h := range hits {
		order = append(order, h.name)
	}
	return order
}
