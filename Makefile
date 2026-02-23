SHELL := /bin/bash

CORE_DIR := nexus-core
OPERATOR_DIR := nexus-operator
TOWER_DIR := nexus-tower
SIM_ENGINE_DIR := sim-engine
CORE_SERVICE := nexus-core

.PHONY: up build down clean logs migrate-up migrate-down cleanup-idempotency seed \
	core-test operator-test tower-test tower-qa qa e2e jwt-e2e quickstart-admin \
	core-dev operator-dev tower-dev qa-sim-engine migrate-sim-engine demo-doorjam replay reset-nexus logs-tail

up:
	docker compose up -d --remove-orphans

build:
	docker compose build

down:
	docker compose down --remove-orphans

clean:
	docker compose down -v --remove-orphans

logs:
	docker compose logs -f

logs-tail:
	docker compose logs --tail=$${TAIL:-200}

migrate-up:
	docker compose exec -T $(CORE_SERVICE) /app/migrate -cmd up

migrate-down:
	docker compose exec -T $(CORE_SERVICE) /app/migrate -cmd down -steps 1

migrate-sim-engine:
	@docker compose up -d --wait postgres
	docker compose exec -T postgres psql -U postgres -d nexus < sim-engine/migrations/0001_sim_engine.up.sql

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
		if ! python -c "import importlib.util,sys;mods=['fastapi','httpx','pydantic','pytest'];sys.exit(0 if all(importlib.util.find_spec(m) for m in mods) else 1)"; then \
			if ! pip install -q -e '.[dev]'; then \
				echo "operator-test: dependencies unavailable; skipping (set NEXUS_QA_STRICT=1 to fail)"; \
				if [ "$${NEXUS_QA_STRICT:-0}" = "1" ]; then exit 1; fi; \
				exit 0; \
			fi; \
		fi && \
		pytest -q

tower-test:
	cd $(TOWER_DIR) && \
		if ! npm install; then \
			echo "tower-test: dependencies unavailable; skipping (set NEXUS_QA_STRICT=1 to fail)"; \
			if [ "$${NEXUS_QA_STRICT:-0}" = "1" ]; then exit 1; fi; \
			exit 0; \
		fi && \
		npm run test

tower-qa:
	cd $(TOWER_DIR) && \
		if ! npm install; then \
			echo "tower-qa: dependencies unavailable; skipping (set NEXUS_QA_STRICT=1 to fail)"; \
			if [ "$${NEXUS_QA_STRICT:-0}" = "1" ]; then exit 1; fi; \
			exit 0; \
		fi && \
		npm run qa

qa:
	$(MAKE) core-test
	$(MAKE) operator-test
	$(MAKE) tower-qa

qa-sim-engine:
	cd $(SIM_ENGINE_DIR) && GOCACHE=/tmp/go-build GOMODCACHE=/home/pablo/go/pkg/mod GOPROXY=off GOSUMDB=off go test ./...
	cd $(CORE_DIR) && GOCACHE=/tmp/go-build go test ./internal/world ./internal/gateway ./pkg/utils -run 'TestHandler_|TestServiceListRuns_|TestRun_SSRFAllowlist_|TestRun_SimEngineInternalHeaders|TestRun_NonSimEngineDoesNotGetInternalKey|TestValidateEgressURLWithAllowlist|TestRun_WorldPolicyDenied_EmitsEnforcementEvent|TestRun_WorldRateLimited_EmitsEnforcementEvent'

demo-doorjam:
	$(MAKE) migrate-sim-engine
	bash scripts/seed_sim_engine_demo.sh
	python scripts/demo_doorjam.py

replay:
	@if [ -z "$(RUN_ID)" ]; then echo "RUN_ID is required. Usage: make replay RUN_ID=<run-id>"; exit 1; fi
	python scripts/replay_sim_engine.py --run-id "$(RUN_ID)"

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
