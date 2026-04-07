.PHONY: test build run vet fmt lint check clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

test:
	go test ./... -count=1 -timeout=60s

build:
	go build -ldflags "-X main.version=$(VERSION)" -o praetor ./cmd/praetor/

run: build
	./praetor

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
