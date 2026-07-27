package weft

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/llmpack"
)

func TestResolvePreset(t *testing.T) {
	p, ok := ResolvePreset("qwen-7b")
	if !ok || !strings.Contains(p.Model, "Qwen") {
		t.Fatalf("got %+v ok=%v", p, ok)
	}
	if _, ok := ResolvePreset("nope"); ok {
		t.Fatal("expected unknown")
	}
}

func TestIsCloudOpenAI(t *testing.T) {
	if !isCloudOpenAI("https://api.openai.com/v1") {
		t.Fatal("expected cloud")
	}
	if !isCloudOpenAI("") {
		t.Fatal("empty defaults to cloud")
	}
	if isCloudOpenAI("https://ft.internal.corp/v1") {
		t.Fatal("internal should not be cloud")
	}
	if isCloudOpenAI("http://127.0.0.1:8000/v1") {
		t.Fatal("localhost should not be cloud")
	}
}

func TestRefuseCloudUploadWithoutAllow(t *testing.T) {
	dir := t.TempDir()
	// minimal chat.jsonl so prepare is skipped
	chat := filepath.Join(dir, "chat.jsonl")
	if err := os.WriteFile(chat, []byte(`{"messages":[{"role":"user","content":"x"},{"role":"assistant","content":"y"}]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Finetune(FinetuneOptions{
		Backend:     "openai",
		DataDir:     dir,
		SkipPrepare: true,
		// no AllowUpload
		OpenAIAPIKey:  "sk-test",
		OpenAIBaseURL: "https://api.openai.com/v1",
	})
	if err == nil {
		t.Fatal("expected privacy refusal")
	}
	if !strings.Contains(err.Error(), "refusing to upload") {
		t.Fatalf("got %v", err)
	}
}

func TestPrivateDryRun(t *testing.T) {
	dir := t.TempDir()
	chat := filepath.Join(dir, "chat.jsonl")
	if err := os.WriteFile(chat, []byte(`{"messages":[{"role":"user","content":"x"},{"role":"assistant","content":"y"}]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Finetune(FinetuneOptions{
		Private:     true,
		DataDir:     dir,
		SkipPrepare: true,
		DryRun:      true,
		Model:       "Qwen/Qwen2.5-1.5B-Instruct",
	})
	// dry-run may still need python path — if python missing, that's ok if error mentions it;
	// prefer success when python exists
	if err != nil && !strings.Contains(err.Error(), "python") {
		t.Fatal(err)
	}
}

func TestLoadPrivateDomainJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "domain.jsonl")
	body := `{"instruction":"say hi","output":"fn main { say(\"hi\") }","tags":["x"]}
{"messages":[{"role":"user","content":"sum"},{"role":"assistant","content":"fn main { say(1+2) }"}]}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	exs, err := llmpack.LoadExamplesFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(exs) != 2 {
		t.Fatalf("got %d", len(exs))
	}
	if exs[0].Instruction != "say hi" {
		t.Fatalf("%+v", exs[0])
	}
}

func TestOfflinePack(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "airgap")
	if err := PrepareOfflinePack(OfflinePackOptions{OutDir: out, Expand: false}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"chat.jsonl", "PRIVACY.md", "train_private.sh", "train_trl.py", "SYSTEM.md"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}

func TestPrivateOverridesOpenAIBackend(t *testing.T) {
	// --private rewrites openai → local private path (never uploads)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "chat.jsonl"), []byte(`{"messages":[]}`+"\n"), 0o644)
	err := Finetune(FinetuneOptions{
		Private:      true,
		Backend:      "openai",
		DataDir:      dir,
		SkipPrepare:  true,
		DryRun:       true,
		OpenAIAPIKey: "sk-x",
		AllowUpload:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
}
