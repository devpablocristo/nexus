SHELL := /bin/bash

.PHONY: run test build qa

run:
	go run ./cmd/sim-engine

test:
	go test ./...

build:
	go build ./cmd/sim-engine

qa:
	go test ./...
