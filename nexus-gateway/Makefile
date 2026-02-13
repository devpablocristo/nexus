SHELL := /bin/bash

.PHONY: dev down logs migrate-up migrate-down seed test e2e fmt wire

dev:
	docker compose up --build

down:
	docker compose down -v

logs:
	docker compose logs -f

migrate-up:
	docker compose exec -T nexus-gateway /app/migrate -cmd up

migrate-down:
	docker compose exec -T nexus-gateway /app/migrate -cmd down -steps 1

seed:
	bash scripts/seed_demo.sh

test:
	go test ./...

e2e:
	bash scripts/e2e.sh

fmt:
	gofmt -w .

wire:
	@echo "Installing wire (if needed): go install github.com/google/wire/cmd/wire@latest"
	@echo "Running wire..."
	wire ./wire
