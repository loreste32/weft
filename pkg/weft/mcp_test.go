package weft_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestMCPServeStdioCallsWeftFunction(t *testing.T) {
	root := t.TempDir()
	program := filepath.Join(root, "server.weft")
	source := `
use mcp

fn echo(args) -> Result {
    Ok({"echo": args.value})
}

fn main {
    mcp.serve_stdio([
        mcp.tool("echo", "Echo a value", echo),
    ])
}
`
	if err := os.WriteFile(program, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	inputRead, inputWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outputRead, outputWrite, err := os.Pipe()
	if err != nil {
		_ = inputRead.Close()
		_ = inputWrite.Close()
		t.Fatal(err)
	}
	oldStdin, oldStdout := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = inputRead, outputWrite
	t.Cleanup(func() {
		os.Stdin, os.Stdout = oldStdin, oldStdout
		_ = inputRead.Close()
		_ = inputWrite.Close()
		_ = outputRead.Close()
		_ = outputWrite.Close()
	})

	ctx := weft.New(weft.Options{})
	done := make(chan error, 1)
	go func() {
		done <- ctx.RunFile(context.Background(), program)
	}()
	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"value":"ok"}}}`,
	}
	for _, request := range requests {
		if _, err := io.WriteString(inputWrite, request+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	if err := inputWrite.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal("MCP server: ", err)
	}
	if err := outputWrite.Close(); err != nil {
		t.Fatal(err)
	}

	var callResponse map[string]any
	scanner := bufio.NewScanner(outputRead)
	for scanner.Scan() {
		var response map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			t.Fatalf("invalid MCP response %q: %v", scanner.Text(), err)
		}
		if response["id"] == float64(3) {
			callResponse = response
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if callResponse == nil {
		t.Fatal("missing tools/call response")
	}
	result, ok := callResponse["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/call response has no result: %#v", callResponse)
	}
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("Weft handler was reported as an MCP error: %#v", result)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("tools/call response has no content: %#v", result)
	}
	textBlock, _ := content[0].(map[string]any)
	if !strings.Contains(textBlock["text"].(string), "ok") {
		t.Fatalf("handler response = %#v, want echoed value", textBlock)
	}
}
