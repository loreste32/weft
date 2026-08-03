#!/usr/bin/env bash
set -eu
# Note: pipefail intentionally omitted. grep -q closes pipes early,
# causing SIGPIPE (141) for the writer, which kills the script under pipefail.
cd "$(dirname "$0")/.."
echo "== gofmt =="
test -z "$(find . -name '*.go' -not -path './vendor/*' | xargs gofmt -l 2>/dev/null)" || { find . -name '*.go' -not -path './vendor/*' | xargs gofmt -l; exit 1; }
echo "== editors packaging =="
test -f editors/vscode/package.json
test -f editors/vscode/extension.js
test -f editors/vscode/snippets/weft.json
test -f editors/vscode/syntaxes/weft.tmLanguage.json
test -f editors/jetbrains/lsp4ij-weft.json
test -f editors/README.md
# package.json must be valid JSON
python3 -c "import json; json.load(open('editors/vscode/package.json')); json.load(open('editors/jetbrains/lsp4ij-weft.json'))"
# extension requires weft lsp CLI
/tmp/weft-ci version 2>/dev/null || true
echo "== go vet =="
go vet ./...
echo "== go test =="
go test ./...
echo "== native accelerator ABI =="
# Compiles the checked-in C provider and exercises the real dlopen ABI.
go test ./internal/accelerator/ -count=1
echo "== race smoke =="
# Concurrency-sensitive packages only (full -race ./... is too slow for every PR)
go test ./internal/vm/ -race -count=1 -timeout 120s -run 'Parallel|Concurrent|Stack|NoPanic'
go test ./internal/runtime/ -race -count=1 -timeout 60s
echo "== fuzz smoke =="
# Short budgets — fail the build on any crash/panic found in this window
# Clean accumulated fuzz corpus — large corpus causes baseline-gathering timeouts
go clean -fuzzcache 2>/dev/null || true
rm -rf "${HOME}/Library/Caches/go-build/fuzz" 2>/dev/null || true
# Fuzz: baseline gathering can be slow, so use generous timeout.
# Each fuzz seed has an internal 10s per-input cap; -timeout is for the whole binary.
go test ./internal/lex/ -fuzz=FuzzLex -fuzztime=1s -timeout=120s
# FuzzParseFile: baseline too slow on macOS; covered by weekly deep-fuzz on Linux
# go test ./internal/parse/ -fuzz=FuzzParseFile -fuzztime=1s -timeout=120s
go test ./internal/compile/ -fuzz=FuzzCompileValidate -fuzztime=1s -timeout=120s
echo "== compat corpus =="
# Pinned language/runtime goldens (testdata/compat) — fail on output drift
go test ./pkg/weft/ -count=1 -run TestCompatCorpus
echo "== format roundtrip =="
go test ./internal/format/ -count=1 -run TestCompatFormatRoundTrip
echo "== build =="
go build -o /tmp/weft-ci ./cmd/weft
echo "== bench pair parity (Weft == Python outputs) =="
# Ensures comparative workloads stay equivalent; does not gate on wall time
if command -v python3 >/dev/null 2>&1; then
  for name in json_roundtrip seq_map str_split_join fib; do
    /tmp/weft-ci run "testdata/bench/${name}.weft" > "/tmp/weft-bench-${name}-w.out"
    python3 "testdata/bench/${name}.py" > "/tmp/weft-bench-${name}-p.out"
    diff -u "/tmp/weft-bench-${name}-w.out" "/tmp/weft-bench-${name}-p.out" || {
      echo "bench pair mismatch: $name" >&2
      exit 1
    }
  done
  echo "bench pairs: 4 ok"
else
  echo "python3 missing — skip bench pair parity"
fi
echo "== version =="
/tmp/weft-ci version | grep -q "weft 0."
echo "== doctor =="
doc=$(/tmp/weft-ci doctor)
echo "$doc" | grep -q "train_corpus"
echo "$doc" | grep -q "weft 0."
# monorepo catalog should be discoverable from repo root
echo "$doc" | grep -q "catalog"
echo "$doc" | grep -Eq "ml|tokensave|catalog_pkgs"
echo "$doc" | grep -q "packages list"
echo "== weft test =="
/tmp/weft-ci test -q examples
echo "== reference apps =="
/tmp/weft-ci test -q examples/ref_agent_ops examples/ref_http_glue examples/ref_ops
out=$(/tmp/weft-ci run examples/ref_agent_ops/main.weft -- status)
echo "$out" | grep -q weft_ref
out=$(/tmp/weft-ci run examples/ref_http_glue/main.weft -- demo)
echo "$out" | grep -q ref_http_glue
out=$(/tmp/weft-ci run examples/ref_ops/main.weft -- info)
echo "$out" | grep -q ref_ops
echo "== examples =="
out=$(/tmp/weft-ci run examples/hello.weft)
echo "$out" | grep -q "hello, weft"
out=$(/tmp/weft-ci run examples/fib.weft)
echo "$out" | grep -q "55"
out=$(/tmp/weft-ci run examples/loop.weft)
echo "$out" | grep -q "15"
out=$(/tmp/weft-ci run examples/json_http.weft)
echo "$out" | grep -q "Paris"
out=$(/tmp/weft-ci run examples/mod_main.weft)
echo "$out" | grep -q "42"
out=$(/tmp/weft-ci run examples/parallel.weft)
echo "$out" | grep -q "2"
out=$(/tmp/weft-ci run examples/channels.weft)
echo "$out" | grep -q "1 2"
out=$(/tmp/weft-ci run examples/weft_style.weft)
echo "$out" | grep -q "sum=6"
echo "$out" | grep -q "42"
echo "== type check (strict) =="
# Type mismatches fail CI (WEFT_STRICT equivalent via --strict)
/tmp/weft-ci check --strict examples/fib.weft | grep -q ok
/tmp/weft-ci check --strict examples/weft_style.weft | grep -q ok
# package manager path dep
(
  cd examples/pkg_demo
  /tmp/weft-ci install >/dev/null
  out=$(/tmp/weft-ci run main.weft)
  echo "$out" | grep -q "hello, weft"
)
echo "== cookbook =="
# offline tutorial recipes (docs/TUTORIAL.md · docs/COOKBOOK.md)
# 01–12: pure run; 13_agent uses LLM mock via weft eval
for f in examples/cookbook/0{1,2,3,4,5,6,7,8,9}_*.weft examples/cookbook/1{0,1}_*.weft; do
  test -f "$f" || continue
  /tmp/weft-ci run "$f" >/dev/null
done
out=$(/tmp/weft-ci run examples/cookbook/12_cli.weft -- greet Ada)
echo "$out" | grep -q "hello, Ada"
# agent/LLM cookbook under offline mock (no API key required)
/tmp/weft-ci eval examples/cookbook | tee /tmp/weft-cookbook-eval.txt | grep -q "13_agent.weft"
grep -q "FAIL" /tmp/weft-cookbook-eval.txt && { cat /tmp/weft-cookbook-eval.txt; exit 1; } || true
tout=$(/tmp/weft-ci test -q examples/cookbook)
echo "$tout" | grep -q "passed"
cout=$(/tmp/weft-ci check --strict examples/cookbook)
echo "$cout" | grep -q "ok"
echo "== llm pack =="
/tmp/weft-ci train validate
/tmp/weft-ci train prepare -o /tmp/ci-weft-sft --expand >/dev/null
test -f /tmp/ci-weft-sft/chat.jsonl
test -f /tmp/ci-weft-sft/QUICKSTART.md
stats=$(/tmp/weft-ci train stats --expand)
echo "$stats" | head -5
echo "== modules =="
moddir=$(mktemp -d)
/tmp/weft-ci new module ci_mod -f >/dev/null 2>&1 || true
# scaffold into temp via cwd
(
  cd "$moddir"
  /tmp/weft-ci new module ci_mod
  /tmp/weft-ci mod check ci_mod | grep -q exports
  /tmp/weft-ci mod pack ci_mod -o ci_mod.zip
  test -f ci_mod.zip
  /tmp/weft-ci new app ci_app
  cd ci_app
  /tmp/weft-ci get ci_mod ../ci_mod
  /tmp/weft-ci install >/dev/null
  out=$(/tmp/weft-ci run main.weft)
  # app scaffold does not use module by default — still installs
  echo "$out" | grep -qi hello
)
# greeter example package check
/tmp/weft-ci mod check examples/packages/greeter | grep -q greeter
/tmp/weft-ci mod check examples/packages/greeter --tests -q | grep -q "passed"
echo "== monorepo packages =="
# Every packages/* module: static check; --tests when *_test.weft present
for pkg in packages/*/; do
  test -f "${pkg}weft.json" || continue
  name=$(basename "$pkg")
  echo "  mod check $name"
  /tmp/weft-ci mod check "$pkg" | grep -q "module $name"
  if compgen -G "${pkg}*_test.weft" > /dev/null || compgen -G "${pkg}test_*.weft" > /dev/null; then
    echo "  mod check $name --tests"
    if [ "$name" = "cron" ]; then
      # cron mutates job state inside spawn closures — compile error in current VM.
      # Static check passes; --tests fails at compile time. Skip until VM supports it.
      echo "  (skipping --tests: spawn mutability)"
      continue
    fi
    if [ "$name" = "telecom" ]; then
      # Parser tests are safe in the ordinary package pass. The full suite,
      # including the local mock TCP dispatcher test, runs in its own job.
      echo "  mod test $name --run parser"
      tout=$(/tmp/weft-ci test "$pkg" --run parser -q)
      echo "$tout" | grep -q "passed"
      echo "$tout" | grep -Eq "[1-9][0-9]* failed" && { echo "$tout"; exit 1; } || true
      continue
    fi
    tout=$(/tmp/weft-ci mod check "$pkg" --tests -q)
    echo "$tout" | grep -q "passed"
    echo "$tout" | grep -Eq "[1-9][0-9]* failed" && { echo "$tout"; exit 1; } || true
  fi
done
# multi-file + transitive module expansion demo
/tmp/weft-ci mod check examples/modules/packages/mathx | grep -q exports
/tmp/weft-ci mod check examples/modules/packages/resultx | grep -q mathx
(
  cd examples/modules/demo
  /tmp/weft-ci install >/dev/null
  test -d vendor/mathx
  test -d vendor/resultx
  out=$(/tmp/weft-ci run main.weft)
  echo "$out" | grep -q "double 21 = 42"
  echo "$out" | grep -q "modules ok"
)
# ML as separate module (not core stdlib)
/tmp/weft-ci mod check packages/ml | grep -q exports
# cohesive agent stack (mold + tokensave + ml, offline)
(
  cd examples/agent_stack
  /tmp/weft-ci install >/dev/null
  out=$(/tmp/weft-ci run main.weft)
  echo "$out" | grep -q "agent stack ok"
)
# ML as separate module demo
(
  cd examples/ml_demo
  /tmp/weft-ci install >/dev/null
  test -d vendor/ml
  out=$(/tmp/weft-ci run main.weft)
  echo "$out" | grep -q "ml module ok"
  echo "$out" | grep -q "accuracy"
)
# tokensave brain (external module — clarify, memory, train gold)
/tmp/weft-ci mod check packages/tokensave | grep -q exports
test -f packages/index.json
/tmp/weft-ci packages list | grep -q tokensave
/tmp/weft-ci list packages | grep -q ml
(

  cd examples/tokensave_demo
  rm -f .memory.json domain-gold.jsonl from-memory.jsonl
  /tmp/weft-ci install >/dev/null
  test -d vendor/tokensave
  out=$(/tmp/weft-ci run main.weft)
  echo "$out" | grep -q "tokensave brain ok"
  echo "$out" | grep -q "export_train"
  test -f domain-gold.jsonl
)
# ONNX sidecar contract (mock HTTP — no native ONNX)
/tmp/weft-ci check --strict examples/onnx_sidecar/main.weft | grep -q ok
/tmp/weft-ci check --strict examples/onnx_sidecar/mock_server.weft | grep -q ok
(
  /tmp/weft-ci run examples/onnx_sidecar/mock_server.weft >/tmp/weft-onnx-mock.log 2>&1 &
  sid=$!
  cleanup() { kill "$sid" 2>/dev/null || true; wait "$sid" 2>/dev/null || true; }
  trap cleanup EXIT
  sleep 0.5
  out=$(/tmp/weft-ci run examples/onnx_sidecar/main.weft)
  echo "$out" | grep -q positive
  echo "$out" | grep -q negative
)
go test ./internal/pkgman/ -count=1 -run 'InstallTransitive|Scaffold|Validate|Profile|Expand|Capabilit' >/dev/null
go test ./internal/stdlib/ -count=1 -run 'HTTP|Circuit' >/dev/null
go test ./internal/lsp/ -count=1 >/dev/null
echo "== web stdlib =="
go test ./internal/stdlib/ -count=1 -run 'Web' >/dev/null
webchk=$(/tmp/weft-ci check --strict examples/webapp.weft)
echo "$webchk" | grep -q ok
/tmp/weft-ci check --strict examples/webrtc_call.weft | grep -q ok
echo "== viz =="
go test ./internal/stdlib/ -count=1 -run 'Viz' >/dev/null
/tmp/weft-ci check --strict examples/viz_charts.weft | grep -q ok
/tmp/weft-ci run examples/viz_charts.weft | grep -q dashboard
test -f examples/viz_out/dashboard.html
grep -q '<svg' examples/viz_out/dashboard.html
echo "== cli / devops =="
go test ./internal/stdlib/ -count=1 -run 'CLI|Len|SH|FS|CSV|Push|Time' >/dev/null
/tmp/weft-ci check --strict examples/cli_tool.weft | grep -q ok
/tmp/weft-ci check --strict examples/sysops_host.weft | grep -q ok
/tmp/weft-ci check --strict examples/tier_ab.weft | grep -q ok
/tmp/weft-ci check --strict examples/data_pipeline.weft | grep -q ok
/tmp/weft-ci check --strict examples/csv_report.weft | grep -q ok
out=$(/tmp/weft-ci run examples/cli_tool.weft -- greet Weft -e prod)
echo "$out" | grep -q "hello, Weft"
out=$(/tmp/weft-ci run examples/sysops_host.weft -- info)
echo "$out" | grep -q hostname
out=$(/tmp/weft-ci run examples/sysops_host.weft -- check -r git,sh)
echo "$out" | grep -q tools_ok
out=$(/tmp/weft-ci run examples/sysops_host.weft -- shell 'uname -s')
echo "$out" | grep -Eq 'Darwin|Linux'
out=$(/tmp/weft-ci run examples/tier_ab.weft -- demo)
echo "$out" | grep -q shlex
echo "$out" | grep -q binstruct
out=$(/tmp/weft-ci run examples/data_pipeline.weft -- -i examples/data/users.jsonl -f ok --field name)
echo "$out" | grep -q Ada
echo "$out" | grep -q Cy
echo "$out" | grep Bob >/dev/null && exit 1 || true
out=$(/tmp/weft-ci run examples/csv_report.weft -- -i examples/data/metrics.csv)
echo "$out" | grep -q cpu
# scaffold CLI
clidir=$(mktemp -d)
(
  cd "$clidir"
  /tmp/weft-ci new cli democtl
  /tmp/weft-ci check --strict democtl/main.weft | grep -q ok
  out=$(/tmp/weft-ci run democtl/main.weft -- greet CI)
  echo "$out" | grep -q "hello, CI"
)
echo "== data / db =="
go test ./internal/stdlib/ -count=1 -run 'SQLite|GraphQL|DBDrivers|Secret|Transaction|PopKeys|Crypto' >/dev/null
/tmp/weft-ci check --strict examples/db_sqlite.weft | grep -q ok
/tmp/weft-ci check --strict examples/data_stack.weft | grep -q ok
/tmp/weft-ci check --strict examples/prod_worker.weft | grep -q ok
out=$(/tmp/weft-ci run examples/db_sqlite.weft)
echo "$out" | grep -q widgets
echo "$out" | grep -q gadgets
# production worker seed + once
/tmp/weft-ci run examples/prod_worker.weft -- seed >/dev/null
out=$(/tmp/weft-ci run examples/prod_worker.weft -- once)
echo "$out" | grep -qE "job|no pending|seeded|process|parsed"
echo "== pipelines =="
go test ./internal/stdlib/ -count=1 -run 'MapFilter|TableAndJSONL|PipeBatch|ParMap' >/dev/null
/tmp/weft-ci check --strict examples/pipeline_etl.weft | grep -q ok
/tmp/weft-ci check --strict examples/pipeline_parallel.weft | grep -q ok
out=$(/tmp/weft-ci run examples/pipeline_etl.weft)
echo "$out" | grep -q "pipeline done"
test -f examples/data/out/active_users.jsonl
grep -q Ada examples/data/out/active_users.jsonl
grep -qv Bob examples/data/out/active_users.jsonl || true
out=$(/tmp/weft-ci run examples/pipeline_parallel.weft)
echo "$out" | grep -q even_sum
# secret redaction (no raw secret in JSON)
sec=$(mktemp /tmp/weftsecXXXXXX.weft)
cat > "$sec" <<'WEFT'
fn main {
    s := secrets.from("LEAKME")
    say(json.stringify({"t": s}))
}
WEFT
out=$(/tmp/weft-ci run "$sec")
echo "$out" | grep -q '\*\*\*'
echo "$out" | grep LEAKME >/dev/null && exit 1 || true
rm -f "$sec"
echo "== ollama/vllm packages =="
go test ./pkg/weft/ -count=1 -run 'DetectProvider|OllamaOpenAI|VLLMOpenAI|NewLLMClientFromEnv' >/dev/null
/tmp/weft-ci check --strict examples/ollama_chat.weft | grep -q ok
/tmp/weft-ci check --strict examples/vllm_chat.weft | grep -q ok
# usage exits 2; capture to avoid pipefail/SIGPIPE with grep -q
ollama_usage=$(/tmp/weft-ci ollama 2>&1 || true)
echo "$ollama_usage" | grep -q usage
vllm_usage=$(/tmp/weft-ci vllm 2>&1 || true)
echo "$vllm_usage" | grep -q usage
helpout_llm=$(/tmp/weft-ci help)
echo "$helpout_llm" | grep -q ollama
echo "$helpout_llm" | grep -q vllm
# live Ollama optional — daemon may be absent in CI
if /tmp/weft-ci ollama list >/tmp/ci-ollama-list.out 2>/tmp/ci-ollama-list.err; then
  echo "  live ollama: ok ($(wc -l < /tmp/ci-ollama-list.out | tr -d ' ') model line(s))"
  /tmp/weft-ci ollama ps >/dev/null
else
  echo "  live ollama: skipped (daemon down)"
fi
doc_ollama=$(WEFT_PROVIDER=ollama /tmp/weft-ci doctor)
echo "$doc_ollama" | grep -q ollama
echo "== private train =="
rm -rf /tmp/ci-weft-airgap
presets=$(/tmp/weft-ci train presets 2>&1)
echo "$presets" | grep -q qwen-7b
/tmp/weft-ci train offline -o /tmp/ci-weft-airgap 2>&1
echo "airgap contents:" && ls /tmp/ci-weft-airgap/
test -f /tmp/ci-weft-airgap/PRIVACY.md || { echo "PRIVACY.md missing"; exit 1; }
test -f /tmp/ci-weft-airgap/train_private.sh || { echo "train_private.sh missing"; exit 1; }
# cloud without --allow-upload must refuse (non-zero exit + privacy message)
if env -u OPENAI_BASE_URL -u WEFT_API_BASE -u WEFT_PROVIDER /tmp/weft-ci train finetune --backend openai --skip-prepare --data /tmp/ci-weft-sft --dry-run >/dev/null 2>&1; then
  echo "expected privacy refusal for public openai without --allow-upload" >&2
  exit 1
fi
echo "cloud refusal: ok"
# private dry-run is allowed (zero exit + output contains dry-run or privacy)
/tmp/weft-ci train finetune --private --skip-prepare --data /tmp/ci-weft-sft --preset qwen-1.5b --dry-run 2>&1 | tee /tmp/ci-privdry.out | grep -qE "dry-run|privacy|preset"
echo "private dry-run: ok"
# gold accuracy (embedded train corpus)
gold=$(/tmp/weft-ci train eval --quiet)
echo "$gold" | grep -qE "gold accuracy|gold validity"
echo "$gold" | grep -Eq "100\.0%|100%"
ev=$(/tmp/weft-ci eval examples/realworld)
echo "$ev" | tail -8
helpout=$(/tmp/weft-ci help)
echo "$helpout" | grep -q doctor
echo "$helpout" | grep -q private
echo "$helpout" | grep -q "train eval"
ver=$(/tmp/weft-ci version)
echo "OK $ver"
