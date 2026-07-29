package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/loreste/weft/pkg/weft"
)

type runRequest struct {
	Code    string `json:"code"`
	Timeout int    `json:"timeout"` // seconds, max 10
}

type runResponse struct {
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

func main() {
	addr := ":9200"
	if a := os.Getenv("PLAYGROUND_ADDR"); a != "" {
		addr = a
	}
	http.HandleFunc("/api/run", handleRun)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok\n"))
	})
	fmt.Printf("playground API listening on %s\n", addr)
	http.ListenAndServe(addr, nil)
}

func handleRun(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "https://weftproject.dev")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(204)
		return
	}
	if r.Method != "POST" {
		json.NewEncoder(w).Encode(runResponse{Error: "method not allowed"})
		return
	}

	var req runRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(runResponse{Error: "bad request"})
		return
	}
	if len(req.Code) > 10000 {
		json.NewEncoder(w).Encode(runResponse{Error: "code too large (max 10KB)"})
		return
	}

	timeout := time.Duration(req.Timeout) * time.Second
	if timeout <= 0 || timeout > 10*time.Second {
		timeout = 5 * time.Second
	}

	// capture stdout
	var out bytes.Buffer
	ctx := weft.New(weft.Options{
		Stdout: &out,
		Stderr: &out,
	})

	// run with timeout
	runCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ctx.RunSource(runCtx, "playground.weft", req.Code)
	}()

	select {
	case err := <-done:
		if err != nil {
			json.NewEncoder(w).Encode(runResponse{
				Output: out.String(),
				Error:  err.Error(),
			})
		} else {
			json.NewEncoder(w).Encode(runResponse{Output: out.String()})
		}
	case <-runCtx.Done():
		json.NewEncoder(w).Encode(runResponse{Error: "execution timed out (max 5 seconds)"})
	}
}
