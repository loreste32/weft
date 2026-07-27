# Weft — weave agents into code
.PHONY: build test ci install clean fmt vet doctor examples

PREFIX ?= $(HOME)/.local
BIN    ?= weft

build:
	go build -o $(BIN) ./cmd/weft

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

clean:
	rm -f $(BIN) /tmp/weft-ci
	rm -rf /tmp/ci-weft-sft
