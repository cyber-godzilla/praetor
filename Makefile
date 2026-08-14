.PHONY: test build web web-assets web-run web-dev web-check web-linux-amd64 web-clean run run-sixel run-kitty run-none run-pprof xterm-sixel foot-sixel vet fmt lint check clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# The TUI ships as `praetor-tui`; the GUI (built under gui/) owns the plain
# `praetor` name. Keep these in sync with packaging/ and the release pipeline.
TUI_BIN ?= praetor-tui
WEB_BIN ?= praetor-web
WEB_LISTEN ?= 127.0.0.1:8787

test:
	go test ./... -count=1 -timeout=60s

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(TUI_BIN) ./cmd/praetor/

web-assets:
	cd gui/frontend && npm run build
	mkdir -p internal/webassets/dist
	rm -rf internal/webassets/dist/assets
	cp gui/frontend/dist/index.html internal/webassets/dist/index.html
	cp -R gui/frontend/dist/assets internal/webassets/dist/assets

web: web-assets
	go build -ldflags "-X main.version=$(VERSION)" -o $(WEB_BIN) ./cmd/praetor-web/

web-run: web
	./$(WEB_BIN) --listen $(WEB_LISTEN)

web-dev: web-assets
	go run -ldflags "-X main.version=$(VERSION)" ./cmd/praetor-web/ --listen $(WEB_LISTEN)

web-check: web-assets
	go test -race ./internal/web ./internal/webtls ./internal/gui ./cmd/praetor-web -count=1
	cd gui/frontend && npm test

web-linux-amd64: web-assets
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o praetor-web-linux-amd64 ./cmd/praetor-web/

web-clean:
	rm -f $(WEB_BIN) praetor-web-linux-amd64
	rm -rf internal/webassets/dist/assets internal/webassets/dist/index.html

run: build
	./$(TUI_BIN)

run-sixel: build
	PRAETOR_GRAPHICS=sixel ./$(TUI_BIN)

run-kitty: build
	PRAETOR_GRAPHICS=kitty ./$(TUI_BIN)

run-none: build
	PRAETOR_GRAPHICS=none ./$(TUI_BIN)

# Run with pprof: live HTTP at localhost:6060/debug/pprof/ and
# heap+goroutine dumps written to ~/.local/state/praetor/pprof/ on exit.
run-pprof: build
	./$(TUI_BIN) --pprof

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
	rm -f $(TUI_BIN) $(WEB_BIN) praetor
	rm -rf internal/webassets/dist/assets internal/webassets/dist/index.html
