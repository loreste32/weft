package weft_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestTrainEvalGoldCorpus(t *testing.T) {
	rep, err := weft.TrainEval(weft.TrainEvalOptions{Quiet: true, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total == 0 {
		t.Fatal("empty gold")
	}
	if rep.GoldOK != rep.Total {
		t.Fatalf("gold accuracy %d/%d", rep.GoldOK, rep.Total)
	}
	if rep.GoldAcc != 1 {
		t.Fatalf("acc %v", rep.GoldAcc)
	}
}

func TestTrainEvalFromJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gold.jsonl")
	row := "{\"id\":\"hi\",\"instruction\":\"print hi\",\"output\":\"fn main { say(\\\"hi\\\") }\\n\"}\n"
	row += "{\"id\":\"bad\",\"instruction\":\"broken\",\"output\":\"not weft at all\"}\n"
	if err := os.WriteFile(path, []byte(row), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := weft.TrainEval(weft.TrainEvalOptions{From: path, Quiet: true})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 2 || rep.GoldOK != 1 {
		t.Fatalf("total=%d ok=%d cases=%+v", rep.Total, rep.GoldOK, rep.Cases)
	}
	if code := weft.PrintTrainEval(rep); code != 1 {
		t.Fatalf("want exit 1 on incomplete gold, got %d", code)
	}
}

func TestTrainEvalRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "g.jsonl")
	_ = os.WriteFile(path, []byte(`{"id":"sum","instruction":"x","output":"fn main { say(1+1) }\n"}`+"\n"), 0o644)
	rep, err := weft.TrainEval(weft.TrainEvalOptions{From: path, Run: true, Quiet: true})
	if err != nil {
		t.Fatal(err)
	}
	if rep.GoldOK != 1 {
		t.Fatalf("%+v", rep)
	}
}

func TestTrainEvalRunDoesNotHideRuntimeFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime-error.jsonl")
	row := `{"id":"runtime-error","instruction":"fail","output":"fn main -> Result { fs.read(\"/definitely/missing/weft-eval-file\")? }"}` + "\n"
	if err := os.WriteFile(path, []byte(row), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := weft.TrainEval(weft.TrainEvalOptions{From: path, Run: true, Quiet: true})
	if err != nil {
		t.Fatal(err)
	}
	if rep.GoldOK != 0 || len(rep.Cases) != 1 || rep.Cases[0].Err == "" {
		t.Fatalf("runtime failure was not reported: %+v", rep)
	}
}
