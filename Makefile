# Weft — weave agents into code
.PHONY: build build-slim test ci install clean fmt vet doctor examples wasm wasm-test wasm-serve race-smoke fuzz-smoke bench bench-glue release-smoke

PREFIX ?= $(HOME)/.local
BIN    ?= weft

# Single version source: pkg/weft/weft.go (const Version).
VERSION := $(shell sed -n 's/^const Version = "\(.*\)"/\1/p' pkg/weft/weft.go)

build:
	go build -o $(BIN) ./cmd/weft

# Slim binary: no SQL/broker drivers (~19MB vs ~42MB full). Agent/HTTP/ops still work.
build-slim:
	go build -tags slim -o $(BIN)-slim ./cmd/weft

install: build
	install -d $(PREFIX)/bin
	install -m 755 $(BIN) $(PREFIX)/bin/$(BIN)

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

doctor: build
	./$(BIN) doctor

ci:
	bash scripts/ci.sh

examples: build
	./$(BIN) run examples/hello.weft
	./$(BIN) run examples/fib.weft
	./$(BIN) run examples/weft_style.weft
	./$(BIN) check examples/fib.weft
	./$(BIN) train validate

# Browser Wasm runtime (GOOS=js GOARCH=wasm). Browser fetch + virtual fs; host-only capabilities return errors.
wasm:
	mkdir -p wasm
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" wasm/wasm_exec.js 2>/dev/null || \
		cp "$$(go env GOROOT)/misc/wasm/wasm_exec.js" wasm/wasm_exec.js
	GOOS=js GOARCH=wasm go build -ldflags "-X main.weftVersion=$(VERSION)-wasm" -o wasm/weft.wasm ./cmd/weft-wasm
	@ls -lah wasm/weft.wasm
	@echo "Open wasm/playground.html via a local server, e.g.: make wasm-serve"

wasm-test: wasm
	node wasm/weft_test.js
	node wasm/integration_test.js

wasm-serve: wasm
	@echo "Serving ./wasm on http://127.0.0.1:8765/playground.html"
	cd wasm && python3 -m http.server 8765

# Reliability: race detector on concurrency-sensitive packages (also in scripts/ci.sh)
race-smoke:
	go test ./internal/vm/ -race -count=1 -timeout 120s -run 'Parallel|Concurrent|Stack|NoPanic'
	go test ./internal/runtime/ -race -count=1 -timeout 60s

# Short local fuzz (also in scripts/ci.sh with 8s budgets)
fuzz-smoke:
	go test ./internal/lex/ -fuzz=FuzzLex -fuzztime=8s
	go test ./internal/parse/ -fuzz=FuzzParseFile -fuzztime=8s
	go test ./internal/compile/ -fuzz=FuzzCompileValidate -fuzztime=8s

# Micro-benchmarks (glue-script oriented, Go harness)
bench:
	go test ./internal/vm/ ./internal/compile/ -bench=. -benchmem -count=1 -run=^$$

# Comparative Weft vs Python3 glue workloads (wall time; not PR-gated on numbers)
bench-glue:
	bash scripts/bench-glue.sh

# Full + slim build, compat goldens, GOOS compile matrix
release-smoke:
	bash scripts/release-smoke.sh

clean:
	rm -f $(BIN) /tmp/weft-ci
	rm -rf /tmp/ci-weft-sft
	rm -f wasm/weft.wasm
