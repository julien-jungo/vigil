VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
BINARY  := vigil
CMD     := ./cmd/vigil

.PHONY: build test lint fmt test-container

build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

test:
	go test ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -l -w .
	goimports -l -w .

DOCKER_CACHE_FLAGS ?=

test-container:
	docker buildx build $(DOCKER_CACHE_FLAGS) --load -t vigil:test .
	docker run --rm vigil:test --version
	docker run --rm --entrypoint playwright-mcp vigil:test --help > /dev/null
