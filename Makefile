SHELL := /bin/bash

CORE_DIR := nexus-core
OPERATOR_DIR := nexus-operator
TOWER_DIR := nexus-tower
CORE_SERVICE := nexus-core

.PHONY: up down logs migrate-up migrate-down cleanup-idempotency seed \
	core-test operator-test tower-test qa e2e jwt-e2e quickstart-admin \
	core-dev operator-dev tower-dev

up:
	docker compose up --build -d

down:
	docker compose down -v

logs:
	docker compose logs -f

migrate-up:
	docker compose exec -T $(CORE_SERVICE) /app/migrate -cmd up

migrate-down:
	docker compose exec -T $(CORE_SERVICE) /app/migrate -cmd down -steps 1

cleanup-idempotency:
	docker compose exec -T $(CORE_SERVICE) /app/cleanup-idempotency

seed:
	cd $(CORE_DIR) && NEXUS_COMPOSE_FILE=../docker-compose.yml bash scripts/seed_demo.sh

core-test:
	cd $(CORE_DIR) && go test ./...

operator-test:
	cd $(OPERATOR_DIR) && \
		if [ ! -d .venv ]; then python3 -m venv .venv; fi && \
		. .venv/bin/activate && \
		pip install -q -e '.[dev]' && \
		pytest -q

tower-test:
	cd $(TOWER_DIR) && npm install && npm run test

qa:
	$(MAKE) core-test
	cd $(OPERATOR_DIR) && \
		if [ ! -d .venv ]; then python3 -m venv .venv; fi && \
		. .venv/bin/activate && \
		pip install -q -e '.[dev]' && \
		pytest -q
	cd $(TOWER_DIR) && npm install && npm run qa

e2e:
	cd $(CORE_DIR) && bash scripts/e2e.sh

jwt-e2e:
	cd $(CORE_DIR) && bash scripts/e2e_jwt.sh

quickstart-admin:
	cd $(CORE_DIR) && bash scripts/quickstart_admin.sh

core-dev:
	cd $(CORE_DIR) && go run ./cmd/api

operator-dev:
	cd $(OPERATOR_DIR) && . .venv/bin/activate && uvicorn app.main:app --host 0.0.0.0 --port 8000

tower-dev:
	cd $(TOWER_DIR) && npm run dev
