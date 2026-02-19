SHELL := /bin/bash

.PHONY: up down logs migrate-up migrate-down cleanup-idempotency seed test e2e jwt-e2e mcp-e2e secrets-e2e qa fmt wire quickstart-admin

up:
	docker compose up --build

down:
	docker compose down -v

logs:
	docker compose logs -f

migrate-up:
	docker compose exec -T nexus-gateway /app/migrate -cmd up

migrate-down:
	docker compose exec -T nexus-gateway /app/migrate -cmd down -steps 1

cleanup-idempotency:
	docker compose exec -T nexus-gateway /app/cleanup-idempotency

seed:
	bash scripts/seed_demo.sh

test:
	go test ./...

e2e:
	bash scripts/e2e.sh

jwt-e2e:
	bash scripts/e2e_jwt.sh

mcp-e2e:
	bash scripts/e2e.sh

secrets-e2e:
	bash scripts/e2e.sh

qa:
	$(MAKE) down
	docker compose up --build -d
	$(MAKE) migrate-up
	$(MAKE) seed
	$(MAKE) test
	$(MAKE) e2e

quickstart-admin:
	bash scripts/quickstart_admin.sh

fmt:
	gofmt -w .

wire:
	@echo "Installing wire (if needed): go install github.com/google/wire/cmd/wire@latest"
	@echo "Running wire..."
	wire ./wire
