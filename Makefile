VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/otonielsilva/api-specter/internal/cli.version=$(VERSION)

.PHONY: all build test race cover vet lint fmt tidy zoo check hooks clean

all: check build

build:
	go build -ldflags "$(LDFLAGS)" -o bin/specter ./cmd/specter
	go build -o bin/zoo ./cmd/zoo

test:
	go test ./...

race:
	go test -race ./...

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -n 1

vet:
	go vet ./...

# CI runs the latest golangci-lint release (see .github/workflows/ci.yml), so a
# newer local or CI version can report issues an older one does not. If lint
# passes here but fails in CI, update with:
#   go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
lint:
	golangci-lint run

fmt:
	golangci-lint fmt

tidy:
	go mod tidy

zoo:
	go run ./cmd/zoo

check: vet race lint

# Use the versioned hooks in .githooks (run once per clone).
hooks:
	git config core.hooksPath .githooks

clean:
	rm -rf bin coverage.out
