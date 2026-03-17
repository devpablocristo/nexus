# Nexus Review v1 — PoC (RFC 3.0)
.PHONY: build run test lint

build:
	go build -o bin/nexus-review ./cmd/api

run: build
	./bin/nexus-review

test:
	go test ./...

lint:
	golangci-lint run ./...
