SHELL := /bin/bash

CORE_DIR := nexus-core
AI_OPERATORS_DIR := nexus-ai-operators
CONTROL_OPERATORS_DIR := nexus-control-operators
TOWER_DIR := nexus-tower
SIM_ENGINE_DIR := sim-engine
CORE_SERVICE := nexus-core
COMPOSE := docker compose

.PHONY: up build down clean logs migrate-up migrate-down cleanup-idempotency seed \
	core-test control-operators-test ai-operators-test tower-test tower-qa qa e2e jwt-e2e quickstart-admin \
	core-dev control-dev ai-operators-dev tower-dev reset-nexus logs-tail up-ready status \
	qa-sim-engine migrate-sim-engine demo-doorjam replay \
	sdk-test-python sdk-test

up:
	$(COMPOSE) up -d --remove-orphans --wait

up-ready: up

build:
	$(COMPOSE) build

down:
	$(COMPOSE) down --remove-orphans

clean:
	$(COMPOSE) down -v --remove-orphans

logs:
	$(COMPOSE) logs -f

logs-tail:
	$(COMPOSE) logs --tail=$${TAIL:-200} $${SERVICE:-}

status:
	$(COMPOSE) ps

migrate-up:
	$(COMPOSE) up -d --wait $(CORE_SERVICE)
	$(COMPOSE) exec -T $(CORE_SERVICE) /app/migrate -cmd up

migrate-down:
	$(COMPOSE) exec -T $(CORE_SERVICE) /app/migrate -cmd down -steps 1

migrate-sim-engine:
	@$(COMPOSE) up -d --wait postgres
	$(COMPOSE) exec -T postgres psql -U postgres -d nexus < sim-engine/migrations/0001_sim_engine.up.sql

cleanup-idempotency:
	$(COMPOSE) exec -T $(CORE_SERVICE) /app/cleanup-idempotency

seed:
	@echo "Waiting for core to be healthy..."
	@$(COMPOSE) up -d --wait $(CORE_SERVICE) postgres
	bash scripts/seed_demo.sh

core-test:
	cd $(CORE_DIR) && go test ./...

control-operators-test:
	cd $(CONTROL_OPERATORS_DIR) && go test ./...

ai-operators-test:
	cd $(AI_OPERATORS_DIR) && \
		if [ ! -d .venv ]; then python3 -m venv .venv; fi && \
		. .venv/bin/activate && \
		if ! python -c "import importlib.util,sys;mods=['fastapi','httpx','pydantic','pytest','prometheus_client'];sys.exit(0 if all(importlib.util.find_spec(m) for m in mods) else 1)"; then \
			if ! pip install -q -e '.[dev]'; then \
				echo "ai-operators-test: dependencies unavailable; skipping (set NEXUS_QA_STRICT=1 to fail)"; \
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
	$(MAKE) control-operators-test
	$(MAKE) ai-operators-test
	$(MAKE) tower-qa

qa-sim-engine:
	cd $(SIM_ENGINE_DIR) && GOCACHE=/tmp/go-build GOMODCACHE=/home/pablo/go/pkg/mod GOPROXY=off GOSUMDB=off go test ./...
	cd $(CORE_DIR) && GOCACHE=/tmp/go-build go test ./internal/world ./internal/gateway ./pkg/utils -run 'TestHandler_|TestServiceListRuns_|TestRun_SSRFAllowlist_|TestRun_SimEngineInternalHeaders|TestRun_NonSimEngineDoesNotGetInternalKey|TestValidateEgressURLWithAllowlist|TestRun_WorldPolicyDenied_EmitsEnforcementEvent|TestRun_WorldRateLimited_EmitsEnforcementEvent'

reset-nexus:
	$(MAKE) clean
	$(MAKE) up-ready
	$(MAKE) migrate-up
	$(MAKE) seed
	$(MAKE) status

e2e:
	bash scripts/e2e.sh

e2e-first:
	@# Primer caso: run echo (requiere: make up, make migrate-up, make seed)
	bash scripts/e2e/01_run_echo.sh

jwt-e2e:
	bash scripts/e2e_jwt.sh

quickstart-admin:
	bash scripts/quickstart_admin.sh

demo-doorjam:
	$(MAKE) migrate-sim-engine
	bash scripts/seed_sim_engine_demo.sh
	python scripts/demo_doorjam.py

replay:
	@if [ -z "$(RUN_ID)" ]; then echo "RUN_ID is required. Usage: make replay RUN_ID=<run-id>"; exit 1; fi
	python scripts/replay_sim_engine.py --run-id "$(RUN_ID)"

core-dev:
	cd $(CORE_DIR) && go run ./cmd/api

ai-operators-dev:
	cd $(AI_OPERATORS_DIR) && \
		if [ ! -d .venv ]; then python3 -m venv .venv; fi && \
		. .venv/bin/activate && \
		uvicorn app.main:app --host 0.0.0.0 --port 8000

control-dev:
	cd $(CONTROL_OPERATORS_DIR) && go run ./cmd/ops-workers

tower-dev:
	cd $(TOWER_DIR) && npm run dev

sdk-test-python:
	cd sdks/python-sdk && \
		if [ ! -d .venv ]; then python3 -m venv .venv; fi && \
		. .venv/bin/activate && \
		pip install -q -e '.[dev]' && \
		pytest -q

sdk-test: sdk-test-python
