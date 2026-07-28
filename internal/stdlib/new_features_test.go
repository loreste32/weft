package stdlib_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func runW(t *testing.T, src string) string {
	t.Helper()
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunSource(context.Background(), "test.weft", src); err != nil {
		t.Fatalf("run: %v", err)
	}
	return strings.TrimSpace(out.String())
}

// --- tokenizer ---

func TestTokenizerEstimate(t *testing.T) {
	out := runW(t, `
use tokenizer
fn main { say(tokenizer.estimate("hello world")) }`)
	// ~3 tokens for "hello world" (11 chars / 4)
	if out != "3" {
		t.Fatalf("estimate: %s", out)
	}
}

func TestTokenizerWords(t *testing.T) {
	out := runW(t, `
use tokenizer
fn main { say(len(tokenizer.words("hello, world!"))) }`)
	// "hello" "," " " "world" "!" = 5 tokens
	if out != "5" {
		t.Fatalf("words: %s", out)
	}
}

func TestTokenizerEncodeDecode(t *testing.T) {
	out := runW(t, `
use tokenizer
fn main {
    tok := tokenizer.new()
    tokens := tokenizer.encode(tok, "hello world")
    say(tokenizer.decode(tok, tokens))
}`)
	if out != "hello world" {
		t.Fatalf("encode/decode: %s", out)
	}
}

func TestTokenizerCount(t *testing.T) {
	out := runW(t, `
use tokenizer
fn main {
    tok := tokenizer.new()
    say(tokenizer.count(tok, "hello world"))
}`)
	// "hello" " " "world" = 3
	if out != "3" {
		t.Fatalf("count: %s", out)
	}
}

func TestTokenizerChunk(t *testing.T) {
	out := runW(t, `
use tokenizer
fn main {
    chunks := tokenizer.chunk("one two three four five six seven eight nine ten", 3)
    say(len(chunks))
}`)
	// Should split into multiple chunks
	if out == "0" || out == "1" {
		t.Fatalf("chunk: %s", out)
	}
}

// --- metrics ---

func TestMetricsAccuracy(t *testing.T) {
	out := runW(t, `
use metrics
fn main {
    y := ["cat", "dog", "cat", "cat"]
    p := ["cat", "dog", "dog", "cat"]
    say(metrics.accuracy(y, p))
}`)
	if out != "0.75" {
		t.Fatalf("accuracy: %s", out)
	}
}

func TestMetricsF1(t *testing.T) {
	out := runW(t, `
use metrics
fn main {
    y := ["1", "1", "0", "0"]
    p := ["1", "0", "0", "0"]
    say(metrics.f1(y, p, "1"))
}`)
	// precision=1/1=1, recall=1/2=0.5, f1=2*1*0.5/(1+0.5)=0.667
	if !strings.Contains(out, "0.6") {
		t.Fatalf("f1: %s", out)
	}
}

func TestMetricsConfusionMatrix(t *testing.T) {
	out := runW(t, `
use metrics
fn main {
    y := ["a", "b", "a", "b"]
    p := ["a", "a", "a", "b"]
    cm := metrics.confusion_matrix(y, p)
    say(cm.labels)
}`)
	if !strings.Contains(out, "a") || !strings.Contains(out, "b") {
		t.Fatalf("confusion: %s", out)
	}
}

func TestMetricsBLEU(t *testing.T) {
	out := runW(t, `
use metrics
fn main {
    ref := "the cat sat on the mat"
    hyp := "the cat sat on the mat"
    say(metrics.bleu(ref, hyp, null))
}`)
	if !strings.Contains(out, "1") {
		t.Fatalf("bleu: %s", out)
	}
}

func TestMetricsCosine(t *testing.T) {
	out := runW(t, `
use metrics
fn main {
    say(metrics.cosine([1, 0, 0], [1, 0, 0]))
}`)
	if out != "1" {
		t.Fatalf("cosine: %s", out)
	}
}

func TestMetricsMSE(t *testing.T) {
	out := runW(t, `
use metrics
fn main {
    say(metrics.mse([1, 2, 3], [1, 2, 3]))
}`)
	if out != "0" {
		t.Fatalf("mse: %s", out)
	}
}

func TestMetricsR2(t *testing.T) {
	out := runW(t, `
use metrics
fn main {
    say(metrics.r2([1, 2, 3, 4], [1, 2, 3, 4]))
}`)
	if out != "1" {
		t.Fatalf("r2: %s", out)
	}
}

// --- dataset ---

func TestDatasetStream(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.jsonl")
	os.WriteFile(path, []byte(`{"name":"Ada"}
{"name":"Bob"}
{"name":"Cy"}
`), 0644)
	out := runW(t, `
use dataset
fn main -> Result {
    mut n := 0
    for row in dataset.stream("`+path+`")? {
        n = n + 1
    }
    say(n)
}`)
	if out != "3" {
		t.Fatalf("stream: %s", out)
	}
}

func TestDatasetCount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.jsonl")
	os.WriteFile(path, []byte("{\"a\":1}\n{\"a\":2}\n{\"a\":3}\n"), 0644)
	out := runW(t, `
use dataset
fn main -> Result {
    n := dataset.count("`+path+`")?
    say(n)
}`)
	if out != "3" {
		t.Fatalf("count: %s", out)
	}
}

func TestDatasetHead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.jsonl")
	os.WriteFile(path, []byte("{\"x\":1}\n{\"x\":2}\n{\"x\":3}\n{\"x\":4}\n{\"x\":5}\n"), 0644)
	out := runW(t, `
use dataset
fn main -> Result {
    rows := dataset.head("`+path+`", 2)?
    say(len(rows))
}`)
	if out != "2" {
		t.Fatalf("head: %s", out)
	}
}

// --- ratelimit ---

func TestRatelimitTry(t *testing.T) {
	out := runW(t, `
use ratelimit
fn main {
    rl := ratelimit.new(5, "second")
    say(ratelimit.acquire(rl))
    say(ratelimit.acquire(rl))
}`)
	if !strings.Contains(out, "true") {
		t.Fatalf("ratelimit: %s", out)
	}
}

// --- env.load ---

func TestEnvLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	os.WriteFile(path, []byte("MY_KEY=hello\nMY_SECRET=\"world\"\n# comment\nexport MY_FLAG=1\n"), 0644)
	out := runW(t, `
use env
fn main -> Result {
    env.load("`+path+`")?
    say(env.get("MY_KEY"))
    say(env.get("MY_SECRET"))
    say(env.get("MY_FLAG"))
}`)
	if !strings.Contains(out, "hello") || !strings.Contains(out, "world") || !strings.Contains(out, "1") {
		t.Fatalf("env.load: %s", out)
	}
}

func TestEnvLoadYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte("database-url: postgres://localhost/app\nport: 8080\n"), 0644)
	out := runW(t, `
use env
fn main -> Result {
    env.load("`+path+`")?
    say(env.get("DATABASE_URL"))
    say(env.get("PORT"))
}`)
	if !strings.Contains(out, "postgres://localhost/app") || !strings.Contains(out, "8080") {
		t.Fatalf("env.load yaml: %s", out)
	}
}
