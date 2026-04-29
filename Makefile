VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
BINARY  := vigil
CMD     := ./cmd/vigil

.PHONY: build test lint fmt

build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

test:
	go test ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -l -w .
	goimports -l -w .
