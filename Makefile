VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/otonielsilva/api-specter/internal/cli.version=$(VERSION)

.PHONY: all build test race cover vet lint fmt tidy zoo check clean

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

lint:
	golangci-lint run

fmt:
	golangci-lint fmt

tidy:
	go mod tidy

zoo:
	go run ./cmd/zoo

check: vet race lint

clean:
	rm -rf bin coverage.out
