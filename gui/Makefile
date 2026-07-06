# Praetor GUI (Wails) build helpers.
#
# Ubuntu 25.10 (and other distros that dropped webkit2gtk-4.0) require the
# `webkit2_41` build tag so Wails links against webkit2gtk-4.1. The tag is
# harmless on macOS/Windows (those platforms don't compile the webkit cgo
# files), so it is always passed — you never have to remember it.

WAILS_TAGS ?= webkit2_41
# Stamp the same git-describe version the TUI's root Makefile uses, so the two
# clients always report the same version. Override with VERSION=v1.2.3.
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS    := -ldflags "-X main.version=$(VERSION)"

.PHONY: dev build installer deps check clean

## dev: run the live-reloading Wails dev server
dev:
	wails dev -tags "$(WAILS_TAGS)" $(LDFLAGS)

## build: produce build/bin/praetor
build:
	wails build -tags "$(WAILS_TAGS)" $(LDFLAGS)

## installer: Windows only — build + NSIS installer
installer:
	wails build -tags "$(WAILS_TAGS)" -nsis $(LDFLAGS)

## check: toolchain-free checks (no webview needed): frontend + dev-mode compile
check:
	cd frontend && npm run build && npm test
	go build -o /dev/null ./...
	go vet ./...

## deps: install the Wails CLI and frontend dependencies
deps:
	go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
	cd frontend && npm install

## clean: remove build output
clean:
	rm -rf build/bin
