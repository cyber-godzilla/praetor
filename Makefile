.PHONY: test build run run-sixel run-kitty run-none xterm-sixel foot-sixel vet fmt lint check clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

test:
	go test ./... -count=1 -timeout=60s

build:
	go build -ldflags "-X main.version=$(VERSION)" -o praetor ./cmd/praetor/

run: build
	./praetor

run-sixel: build
	PRAETOR_GRAPHICS=sixel ./praetor

run-kitty: build
	PRAETOR_GRAPHICS=kitty ./praetor

run-none: build
	PRAETOR_GRAPHICS=none ./praetor

# Launch xterm in VT340-emulation mode with enough color registers to
# render sixel graphics, drop into a shell. From that shell you can
# `cd` back here and run `make run-sixel` to test the sixel path.
xterm-sixel:
	xterm -ti vt340 -xrm "XTerm*decTerminalID: vt340" -xrm "XTerm*numColorRegisters: 256" -e bash

# Launch foot — Wayland-native, sixel enabled by default. Drop into a
# shell so you can cd back here and `make run-sixel` to test sixel
# rendering with a modern, well-supported encoder.
foot-sixel:
	foot bash

vet:
	go vet ./...

fmt:
	@test -z "$$(gofmt -l . 2>/dev/null)" || (gofmt -l . && echo "Run gofmt to fix" && exit 1)

lint:
	@which staticcheck > /dev/null 2>&1 || (echo "staticcheck not installed: go install honnef.co/go/tools/cmd/staticcheck@latest" && exit 1)
	staticcheck ./...

check: vet fmt test

clean:
	rm -f praetor
