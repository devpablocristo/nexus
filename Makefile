SHELL := /bin/bash

CORE_DIR := nexus-core
OPERATOR_DIR := nexus-operator
TOWER_DIR := nexus-tower
WORLDSIM_DIR := world-sim
CORE_SERVICE := nexus-core

.PHONY: up build down clean logs migrate-up migrate-down cleanup-idempotency seed \
	core-test operator-test tower-test qa e2e jwt-e2e quickstart-admin \
	core-dev operator-dev tower-dev qa-worldsim migrate-worldsim demo-doorjam replay reset-nexus logs-tail

up:
	docker compose up -d

build:
	docker compose build

down:
	docker compose down

clean:
	docker compose down -v

logs:
	docker compose logs -f

logs-tail:
	docker compose logs --tail=$${TAIL:-200}

migrate-up:
	docker compose exec -T $(CORE_SERVICE) /app/migrate -cmd up

migrate-down:
	docker compose exec -T $(CORE_SERVICE) /app/migrate -cmd down -steps 1

migrate-worldsim:
	@docker compose up -d --wait postgres
	docker compose exec -T postgres psql -U postgres -d nexus < world-sim/migrations/0001_worldsim.up.sql

cleanup-idempotency:
	docker compose exec -T $(CORE_SERVICE) /app/cleanup-idempotency

seed:
	@echo "Waiting for core to be healthy..."
	@docker compose up -d --wait $(CORE_SERVICE) postgres
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

qa-worldsim:
	cd $(WORLDSIM_DIR) && GOCACHE=/tmp/go-build GOMODCACHE=/home/pablo/go/pkg/mod GOPROXY=off GOSUMDB=off go test ./...
	cd $(CORE_DIR) && GOCACHE=/tmp/go-build go test ./internal/world ./internal/gateway ./pkg/utils -run 'TestHandler_|TestServiceListRuns_|TestRun_SSRFAllowlist_|TestRun_WorldSimInternalHeaders|TestRun_NonWorldSimDoesNotGetInternalKey|TestValidateEgressURLWithAllowlist'

demo-doorjam:
	$(MAKE) migrate-worldsim
	bash scripts/seed_worldsim_demo.sh
	python scripts/demo_doorjam.py

replay:
	@if [ -z "$(RUN_ID)" ]; then echo "RUN_ID is required. Usage: make replay RUN_ID=<run-id>"; exit 1; fi
	python scripts/replay_worldsim.py --run-id "$(RUN_ID)"

reset-nexus:
	$(MAKE) clean
	$(MAKE) build
	$(MAKE) up
	$(MAKE) migrate-up
	$(MAKE) seed
	$(MAKE) demo-doorjam
	$(MAKE) logs-tail

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
