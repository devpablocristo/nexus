# Nexus v3
.PHONY: test qa smoke e2e acceptance up down build logs dev

# --- Quality ---
test:
	cd review && go test ./... -count=1

qa:
	bash scripts/quality/check-api.sh

# --- Docker ---
up:
	@test -f .env || cp .env.example .env
	docker compose up -d --build

down:
	docker compose down

build:
	docker compose build

logs:
	docker compose logs -f

# --- Tests contra API corriendo ---
smoke:
	bash scripts/smoke/run-policies-crud.sh
	bash scripts/smoke/run-requests-flow.sh

e2e:
	bash scripts/e2e/run-full-lifecycle.sh

acceptance: smoke e2e

# --- Dev (hot reload) ---
dev:
	@test -f .env || cp .env.example .env
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build

dev-logs:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml logs -f

dev-down:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml down
