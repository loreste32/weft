# Weft — weave agents into code
.PHONY: build build-slim test ci install clean fmt vet doctor examples wasm wasm-test wasm-serve race-smoke fuzz-smoke bench bench-glue bench-numerical bench-scale vendor-sync vendor-check catalog-check accelerator-report accelerator-conformance capability-matrix capability-matrix-check publish-accelerator-report package-stats release-smoke sbom repro-check

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

# CPU numerical/dataframe wall-time snapshots (not PR-gated on numbers)
bench-numerical:
	bash scripts/bench-numerical.sh

# Keep example vendor trees aligned with packages/
vendor-sync:
	bash scripts/sync-vendor-packages.sh

vendor-check:
	bash scripts/check-vendor-sync.sh

catalog-check:
	bash scripts/check-catalog-sync.sh

# Hardware/provider capability report (safe without GPUs)
accelerator-report:
	bash scripts/accelerator-report.sh

# CPU reference build + external provider JSON/tensor tests (vendors optional)
accelerator-conformance:
	bash scripts/accelerator-conformance.sh

# Honest Warp/DataFrame/ML claim matrix → reports/capability-matrix.{md,json}
capability-matrix:
	python3 scripts/capability-matrix.py --json reports/capability-matrix.json

# Fail CI when committed capability reports drift from the generator
capability-matrix-check:
	python3 scripts/capability-matrix.py --check

# Publish the accelerator capability report (see docs/ACCELERATORS.md)
publish-accelerator-report:
	bash scripts/publish-accelerator-report.sh

# Per-package statistics for packages/* (see scripts/package-stats.sh)
package-stats:
	bash scripts/package-stats.sh

# Scale budgets (warp matmul/elementwise + dataframe); soft budgets unless WEFT_SCALE_STRICT=1
bench-scale:
	bash scripts/bench-scale.sh

# Dependency SBOM (module graph + go.sum hashes) → stdout or file
sbom:
	bash scripts/sbom.sh

# Offline install + byte-reproducible build gate (N6)
repro-check:
	bash scripts/reproducible-build-check.sh

# Full + slim build, compat goldens, GOOS compile matrix
release-smoke:
	bash scripts/release-smoke.sh

clean:
	rm -f $(BIN) /tmp/weft-ci
	rm -rf /tmp/ci-weft-sft
	rm -f wasm/weft.wasm
