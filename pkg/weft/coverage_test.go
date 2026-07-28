package weft

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

// --- finetune ---

func TestTruncate(t *testing.T) {
	if truncate("hello", 10) != "hello" {
		t.Fatal("short")
	}
	if truncate("hello world", 5) != "hello…" {
		t.Fatal("long")
	}
}

func TestFinetuneOpenAIDryRun(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "chat.jsonl"), []byte(`{"messages":[]}`+"\n"), 0644)
	err := finetuneOpenAI(FinetuneOptions{
		DataDir:       dir,
		Model:         "gpt-4o-mini",
		OpenAIBaseURL: "https://api.openai.com/v1",
		OpenAIAPIKey:  "sk-test",
		DryRun:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestOpenAIUploadAndCreateJob(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/files"):
			json.NewEncoder(w).Encode(map[string]string{"id": "file-123"})
		case strings.Contains(r.URL.Path, "/fine_tuning/jobs"):
			if r.Method == "POST" {
				json.NewEncoder(w).Encode(map[string]string{"id": "ftjob-456"})
			} else {
				json.NewEncoder(w).Encode(map[string]any{
					"status":           "succeeded",
					"fine_tuned_model": "ft:gpt-4o-mini:custom",
				})
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "chat.jsonl"), []byte(`{"messages":[]}`+"\n"), 0644)

	// Upload
	fid, err := openaiUploadFile(srv.URL, "sk-test", filepath.Join(dir, "chat.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if fid != "file-123" {
		t.Fatal("file id")
	}

	// Create job
	jid, err := openaiCreateJob(srv.URL, "sk-test", "gpt-4o-mini", fid)
	if err != nil {
		t.Fatal(err)
	}
	if jid != "ftjob-456" {
		t.Fatal("job id")
	}

	// Get job
	st, model, err := openaiGetJob(srv.URL, "sk-test", jid)
	if err != nil {
		t.Fatal(err)
	}
	if st != "succeeded" || model != "ft:gpt-4o-mini:custom" {
		t.Fatal("job status")
	}
}

func TestFinetuneStatusNoKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	err := FinetuneStatus("", "", "ftjob-123")
	if err == nil {
		t.Fatal("should require API key")
	}
}

func TestFinetuneStatusMock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status":           "running",
			"fine_tuned_model": "",
		})
	}))
	defer srv.Close()

	err := FinetuneStatus(srv.URL, "sk-test", "ftjob-123")
	if err != nil {
		t.Fatal(err)
	}
}

// --- gen ---

func TestGenDryRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "fn main { say(1) }"}},
			},
		})
	}))
	defer srv.Close()

	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("OPENAI_BASE_URL", srv.URL)
	dir := t.TempDir()
	out := filepath.Join(dir, "gen.weft")
	err := Gen(GenOptions{
		Task: "print hello",
		Out:  out,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	if !strings.Contains(string(data), "fn main") {
		t.Fatal("generated file should contain fn main")
	}
}

// --- eval suite ---

func TestEvalDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.weft"), []byte(`fn main { say("hello") }`+"\n"), 0644)
	cases, err := EvalDir(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("no cases")
	}
}

func TestPrintEvalReport(t *testing.T) {
	cases := []EvalCase{
		{Path: "a.weft", OK: true},
		{Path: "b.weft", OK: false, Err: "parse error"},
	}
	code := PrintEvalReport(cases)
	if code == 0 {
		t.Fatal("should return non-zero with failures")
	}
}

// --- bench ---

func TestRunBench(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "math_bench.weft"), []byte(`
fn bench_add {
    mut s := 0
    mut i := 0
    while i < 1000 {
        s = s + i
        i = i + 1
    }
}
`), 0644)
	rep, err := RunBench(BenchOptions{Paths: []string{dir}, N: 10})
	if err != nil {
		t.Fatal(err)
	}
	if rep == nil {
		t.Fatal("nil report")
	}
}

func TestPrintBenchReport(t *testing.T) {
	rep := &BenchReport{
		Results: []BenchResult{
			{File: "x.weft", Name: "bench_add", N: 100, NsOp: 500000, OK: true},
		},
	}
	PrintBenchReport(rep, false)
}

// --- doctor ---

func TestDoctor(t *testing.T) {
	// Doctor prints to stdout and should not crash
	err := Doctor()
	if err != nil {
		t.Fatal(err)
	}
}

// --- prompt ---

func TestWritePrompt(t *testing.T) {
	var buf bytes.Buffer
	if err := WritePrompt(&buf, 3); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Fatal("empty prompt")
	}
}

// --- pkgcmd ---

func TestPkgInit(t *testing.T) {
	dir := t.TempDir()
	if err := PkgInit(dir, "myapp"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "weft.json")); err != nil {
		t.Fatal("missing weft.json")
	}
}

func TestNewModule(t *testing.T) {
	dir := t.TempDir()
	root, err := NewModule(dir, "mymod", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "lib.weft")); err != nil {
		t.Fatal("missing lib.weft")
	}
}

func TestNewApp(t *testing.T) {
	dir := t.TempDir()
	root, err := NewApp(dir, "myapp", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "main.weft")); err != nil {
		t.Fatal("missing main.weft")
	}
}

func TestNewCLI(t *testing.T) {
	dir := t.TempDir()
	root, err := NewCLI(dir, "mytool", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "main.weft")); err != nil {
		t.Fatal("missing main.weft")
	}
}

func TestPkgGetAndInstall(t *testing.T) {
	project := t.TempDir()
	PkgInit(project, "testapp")

	// Create a local package
	pkg := t.TempDir()
	os.WriteFile(filepath.Join(pkg, "weft.json"), []byte(`{"name":"helper","version":"0.1.0"}`), 0644)
	os.WriteFile(filepath.Join(pkg, "lib.weft"), []byte(`pub fn greet { "hi" }`+"\n"), 0644)

	if err := PkgGet(project, "helper", pkg); err != nil {
		t.Fatal(err)
	}

	if err := PkgInstall(project); err != nil {
		t.Fatal(err)
	}
}

func TestPkgList(t *testing.T) {
	dir := t.TempDir()
	PkgInit(dir, "testapp")
	if err := PkgList(dir); err != nil {
		t.Fatal(err)
	}
}

func TestModCheck(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "weft.json"), []byte(`{"name":"mymod","version":"0.1.0"}`), 0644)
	os.WriteFile(filepath.Join(dir, "lib.weft"), []byte(`pub fn hello { "hi" }`+"\n"), 0644)

	if err := ModCheck(dir); err != nil {
		t.Fatal(err)
	}
}

// --- train prepare ---

func TestTrainPrepare(t *testing.T) {
	dir := t.TempDir()
	err := PrepareTrainBundle(PrepareOptions{
		OutDir: dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Check output files
	if _, err := os.Stat(filepath.Join(dir, "chat.jsonl")); err != nil {
		t.Fatal("missing chat.jsonl")
	}
}

// --- tooling ---

func TestFmtCheck(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "good.weft"), []byte("fn main { say(1) }\n"), 0644)
	os.WriteFile(filepath.Join(dir, "bad.weft"), []byte("fn main{say(  1  )}\n"), 0644)

	dirty, err := FmtCheck([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(dirty) == 0 {
		t.Fatal("bad.weft should need formatting")
	}
}

func TestFmtFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ugly.weft"), []byte("fn main{say(  1  )}\n"), 0644)
	n, err := FmtFiles([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("should format at least 1 file")
	}
}

// --- registry ---

func TestRegistrySearchOffline(t *testing.T) {
	// This will fail to connect but should not panic
	t.Setenv("WEFT_REGISTRY", "http://127.0.0.1:1")
	err := RegistrySearch("test", false)
	if err == nil {
		t.Fatal("should error on unreachable registry")
	}
}

// --- weft context ---

func TestNewContext(t *testing.T) {
	ctx := New(Options{})
	if ctx == nil {
		t.Fatal("nil context")
	}
}

func TestRunFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.weft")
	os.WriteFile(path, []byte(`fn main { say("ok") }`+"\n"), 0644)

	var out bytes.Buffer
	ctx := New(Options{Stdout: &out})
	if err := ctx.RunFile(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ok") {
		t.Fatal(out.String())
	}
}

func TestRunFileMissing(t *testing.T) {
	ctx := New(Options{})
	err := ctx.RunFile(context.Background(), "/nonexistent.weft")
	if err == nil {
		t.Fatal("should error on missing file")
	}
}

func TestDetectProjectDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "weft.json"), []byte(`{"name":"test"}`), 0644)
	sub := filepath.Join(dir, "src")
	os.MkdirAll(sub, 0755)

	pd := DetectProjectDir(sub)
	if pd != dir {
		t.Fatalf("expected %s, got %s", dir, pd)
	}
}

func TestDetectProjectDirNoManifest(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "deep", "nested")
	os.MkdirAll(sub, 0755)
	pd := DetectProjectDir(sub)
	// May or may not find something depending on temp dir structure
	_ = pd
}

// --- repl helpers ---

func TestBraceDepth(t *testing.T) {
	cases := []struct {
		s    string
		want int
	}{
		{"fn main {", 1},
		{"fn main { say(1) }", 0},
		{"{{{", 3},
		{"}}}", 0}, // negative clamped to 0
		{`"{"`, 0}, // brace inside string
	}
	for _, tc := range cases {
		if got := braceDepth(tc.s); got != tc.want {
			t.Errorf("braceDepth(%q) = %d, want %d", tc.s, got, tc.want)
		}
	}
}

func TestUnbalancedQuotes(t *testing.T) {
	if unbalancedQuotes(`"hello"`) {
		t.Fatal("balanced")
	}
	if !unbalancedQuotes(`"hello`) {
		t.Fatal("unbalanced")
	}
	if unbalancedQuotes(`"esc\"ape"`) {
		t.Fatal("escaped")
	}
}

func TestShouldPrint(t *testing.T) {
	if shouldPrint(runtime.Unit()) {
		t.Fatal("unit")
	}
	if shouldPrint(runtime.Null()) {
		t.Fatal("null")
	}
	if !shouldPrint(runtime.Int(42)) {
		t.Fatal("int")
	}
}

func TestAppendHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	appendHistory("test line")
	p := historyPath()
	if p == "" {
		t.Fatal("empty path")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "test line") {
		t.Fatal("history not written")
	}
}

// --- REPL via pipe (no blocking stdin) ---

func TestREPLPipe(t *testing.T) {
	input := "1 + 2\n:quit\n"
	var out bytes.Buffer
	ctx := New(Options{Stdout: &out})
	err := ctx.RunREPL(strings.NewReader(input), &out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "3") {
		t.Fatalf("REPL output: %q", out.String())
	}
}

func TestREPLMultiLine(t *testing.T) {
	input := "fn double(x) {\n  x * 2\n}\ndouble(21)\n:quit\n"
	var out bytes.Buffer
	ctx := New(Options{Stdout: &out})
	ctx.RunREPL(strings.NewReader(input), &out)
	if !strings.Contains(out.String(), "42") {
		t.Fatalf("REPL multiline: %q", out.String())
	}
}

func TestREPLHelp(t *testing.T) {
	input := ":help\n:q\n"
	var out bytes.Buffer
	ctx := New(Options{Stdout: &out})
	ctx.RunREPL(strings.NewReader(input), &out)
	if !strings.Contains(out.String(), "commands") {
		t.Fatalf("REPL help: %q", out.String())
	}
}

func TestREPLEmptyLine(t *testing.T) {
	input := "\n\n1\n:quit\n"
	var out bytes.Buffer
	ctx := New(Options{Stdout: &out})
	ctx.RunREPL(strings.NewReader(input), &out)
	if !strings.Contains(out.String(), "1") {
		t.Fatalf("REPL empty: %q", out.String())
	}
}

func TestREPLError(t *testing.T) {
	input := "1 / 0\n:quit\n"
	var out bytes.Buffer
	ctx := New(Options{Stdout: &out})
	ctx.RunREPL(strings.NewReader(input), &out)
	if !strings.Contains(out.String(), "division") {
		t.Fatalf("REPL error: %q", out.String())
	}
}

// --- registry CLI (mock server) ---

func TestRegistryKeygen(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := RegistryKeygen("testkey"); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryListKeysEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := RegistryListKeys(); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryListKeysWithKeys(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	RegistryKeygen("a")
	RegistryKeygen("b")
	if err := RegistryListKeys(); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryInfoMock(t *testing.T) {
	idx := map[string]any{
		"packages": []map[string]any{
			{"name": "foo", "version": "1.0.0", "summary": "a foo"},
		},
	}
	b, _ := json.Marshal(idx)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(b)
	}))
	defer srv.Close()

	t.Setenv("WEFT_REGISTRY", srv.URL)
	if err := RegistryInfo("foo"); err != nil {
		t.Fatal(err)
	}
}

func TestRegistrySearchMock(t *testing.T) {
	idx := map[string]any{
		"packages": []map[string]any{
			{"name": "bar", "version": "0.1.0", "summary": "a bar", "signature": "abc"},
		},
	}
	b, _ := json.Marshal(idx)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(b)
	}))
	defer srv.Close()

	t.Setenv("WEFT_REGISTRY", srv.URL)
	if err := RegistrySearch("bar", false); err != nil {
		t.Fatal(err)
	}
	// empty query
	if err := RegistrySearch("", false); err != nil {
		t.Fatal(err)
	}
}

func TestPublishValidationFail(t *testing.T) {
	dir := t.TempDir()
	// No weft.json → validation fails
	err := Publish(dir, "")
	if err == nil {
		t.Fatal("should fail without weft.json")
	}
}

// --- train eval ---

func TestTrainEval(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.weft"), []byte("fn main { say(1) }\n"), 0644)
	cases, err := EvalDir(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	pass := 0
	for _, c := range cases {
		if c.OK {
			pass++
		}
	}
	if pass == 0 {
		t.Fatal("no passing cases")
	}
}

// --- catalog CLI ---

func TestCatalogListLocal(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "packages")
	os.MkdirAll(pkgDir, 0755)
	os.WriteFile(filepath.Join(pkgDir, "index.json"), []byte(`{
		"packages": [{"name": "foo", "path": "./foo", "version": "0.1.0", "summary": "test"}]
	}`), 0644)
	t.Setenv("WEFT_PACKAGES", pkgDir)
	err := CatalogList(dir, "")
	if err != nil {
		t.Fatal(err)
	}
}

// --- train chat ---

func TestTrainChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "fn main { say(1) }"}},
			},
		})
	}))
	defer srv.Close()
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("OPENAI_BASE_URL", srv.URL)

	err := TrainChat("write hello", "")
	// May fail because model isn't set, but exercises the code path
	_ = err
}

// --- format whitespace ---

func TestFormatWhitespace(t *testing.T) {
	input := []byte("fn main {\r\n  say(1)\r}\n")
	result := formatWhitespace(input)
	if strings.Contains(string(result), "\r") {
		t.Fatal("should strip \\r")
	}
}

// --- mask key ---

func TestMaskKey(t *testing.T) {
	m := maskKey("sk-abc123def456")
	if !strings.HasPrefix(m, "sk-") || !strings.HasSuffix(m, "f456") {
		t.Fatalf("mask: %q", m)
	}
	if maskKey("short") != "***" {
		t.Fatalf("mask short: %q", maskKey("short"))
	}
	if maskKey("") != "(not set)" {
		t.Fatalf("mask empty: %q", maskKey(""))
	}
}
