package weft_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const processFixtureTimeout = 10 * time.Second

func TestESLBlackBoxProcess(t *testing.T) {
	cli := os.Getenv("WEFT_CLI")
	if cli == "" {
		t.Skip("WEFT_CLI is required for the real Weft ESL process test")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := listener.Close(); closeErr != nil {
			t.Errorf("close listener: %v", closeErr)
		}
	}()

	address := listener.Addr().String()
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatalf("split listener address %q: %v", address, err)
	}
	if _, err := strconv.Atoi(port); err != nil {
		t.Fatalf("invalid listener port %q: %v", port, err)
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- serveESLBlackBox(listener)
	}()

	root := repositoryRoot()
	cmd := exec.Command(cli, "run", "packages/telecom/esl_blackbox_client.weft")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "ESL_TEST_PORT="+port)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Weft ESL client failed: %v\n%s", err, output)
	}

	select {
	case err := <-serverErr:
		if err != nil {
			t.Fatalf("ESL mock server failed: %v", err)
		}
	case <-time.After(processFixtureTimeout):
		t.Fatal("timed out waiting for ESL mock server")
	}
}

func repositoryRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func serveESLBlackBox(listener net.Listener) (err error) {
	conn, err := listener.Accept()
	if err != nil {
		return fmt.Errorf("accept: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close connection: %w", closeErr)
		}
	}()

	if err := conn.SetDeadline(time.Now().Add(processFixtureTimeout)); err != nil {
		return fmt.Errorf("set initial deadline: %w", err)
	}
	reader := bufio.NewReader(conn)
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

	if err := writeProcessFixture(conn, "Content-Type: command/reply\nReply-Text: +OK accepted\n\n"+
		"Content-Type: text/event-json\nContent-Length: 16\n\n{\"Event\":\"AUTH\"}"); err != nil {
		return err
	}
	commands, err := readUntilProcessFixture(reader, "api one", "api two")
	if err != nil {
		return fmt.Errorf("read concurrent commands: %w", err)
	}
	if !strings.Contains(commands, "api one") || !strings.Contains(commands, "api two") {
		return fmt.Errorf("missing concurrent commands in %q", commands)
	}

	if err := conn.SetDeadline(time.Now().Add(processFixtureTimeout)); err != nil {
		return fmt.Errorf("reset reply deadline: %w", err)
	}
	if err := writeProcessFixture(conn,
		"Content-Type: text/event-json\nContent-Length: 17\n\n{\"Event\":\"FIRST\"}"+
			"Content-Type: command/reply\nReply-Text: +OK one\nContent-Length: 6\n\none-ok"+
			"Content-Type: command/reply\nReply-Text: +OK two\nContent-Length: 6\n\ntwo-ok"); err != nil {
		return err
	}

	marker, err := readUntilProcessFixture(reader, "api timeout-marker")
	if err != nil {
		return fmt.Errorf("read timeout marker: %w", err)
	}
	if !strings.Contains(marker, "api timeout-marker") {
		return fmt.Errorf("missing timeout marker in %q", marker)
	}

	if err := writeProcessFixture(conn,
		"Content-Type: text/event-json\nContent-Length: 18\n\n{\"Event\":\"SECOND\"}"+
			"Content-Type: command/reply\nReply-Text: +OK marker\nContent-Length: 9\n\nmarker-ok"); err != nil {
		return err
	}
	return nil
}

func writeProcessFixture(conn net.Conn, data string) error {
	n, err := conn.Write([]byte(data))
	if err != nil {
		return fmt.Errorf("write %d bytes: %w", len(data), err)
	}
	if n != len(data) {
		return fmt.Errorf("write %d bytes: %w", len(data), io.ErrShortWrite)
	}
	return nil
}

func readUntilProcessFixture(reader *bufio.Reader, needles ...string) (string, error) {
	var received bytes.Buffer
	chunk := make([]byte, 4096)
	for {
		n, err := reader.Read(chunk)
		if n > 0 {
			received.Write(chunk[:n])
			text := received.String()
			allFound := true
			for _, needle := range needles {
				if !strings.Contains(text, needle) {
					allFound = false
					break
				}
			}
			if allFound {
				return text, nil
			}
		}
		if err != nil {
			return received.String(), err
		}
	}
}
