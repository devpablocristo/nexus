SHELL := /bin/bash

CORE_DIR := nexus-core
SAAS_DIR := nexus-saas
AI_OPERATORS_DIR := nexus-ai-operators
CONTROL_OPERATORS_DIR := nexus-control-operators
TOWER_DIR := nexus-tower
CORE_SERVICE := nexus-core
COMPOSE := docker compose

.PHONY: up build down clean logs migrate-up migrate-down cleanup-idempotency seed \
	core-test saas-test control-operators-test ai-operators-test tower-test tower-qa qa contracts-check \
	e2e e2e-first e2e-core e2e-jwt e2e-operators e2e-all quickstart-admin \
	core-dev saas-dev control-dev ai-operators-dev tower-dev reset-nexus logs-tail up-ready status \
	bootstrap demo sdk-test-python sdk-test-go sdk-test-typescript sdk-test infra-validate e2e-stack e2e-ai-operators

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

cleanup-idempotency:
	$(COMPOSE) exec -T $(CORE_SERVICE) /app/cleanup-idempotency

seed:
	@echo "Waiting for core to be healthy..."
	@$(COMPOSE) up -d --wait $(CORE_SERVICE) postgres
	bash scripts/seed/seed_demo.sh

bootstrap:
	bash scripts/bootstrap/bootstrap.sh

demo:
	bash scripts/demo/demo.sh

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
	$(MAKE) contracts-check
	$(MAKE) infra-validate
	$(MAKE) core-test
	$(MAKE) saas-test
	$(MAKE) control-operators-test
	$(MAKE) ai-operators-test
	$(MAKE) tower-qa

contracts-check:
	bash scripts/contracts/check_sync.sh

reset-nexus:
	$(MAKE) clean
	$(MAKE) up-ready
	$(MAKE) migrate-up
	$(MAKE) seed
	$(MAKE) status

e2e:
	bash scripts/e2e/04_core_gateway_isolated.sh

e2e-stack:
	$(MAKE) up-ready
	$(MAKE) migrate-up
	$(MAKE) seed

e2e-first:
	bash scripts/e2e/01_run_echo.sh

e2e-core:
	bash scripts/e2e/03_full_core_e2e.sh

e2e-jwt:
	bash scripts/e2e/05_core_jwt_auth.sh

e2e-operators:
	$(MAKE) e2e-stack
	bash scripts/e2e/06_control_operators.sh

e2e-ai-operators:
	$(MAKE) e2e-stack
	bash scripts/e2e/07_ai_operators.sh

e2e-all:
	$(MAKE) e2e-stack
	bash scripts/e2e/01_run_echo.sh
	bash scripts/e2e/03_full_core_e2e.sh
	bash scripts/e2e/04_core_gateway_isolated.sh
	bash scripts/e2e/05_core_jwt_auth.sh
	bash scripts/e2e/06_control_operators.sh
	bash scripts/e2e/07_ai_operators.sh

quickstart-admin:
	bash scripts/admin/quickstart_admin.sh

core-dev:
	cd $(CORE_DIR) && go run ./cmd/api

saas-test:
	cd $(SAAS_DIR) && go test ./...

saas-dev:
	cd $(SAAS_DIR) && go run ./cmd/nexus-saas

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

sdk-test-go:
	cd sdks/go-sdk && GOWORK=off go test ./...

sdk-test-typescript:
	cd sdks/typescript-sdk && \
		npm install && \
		npm test && \
		npm run build

sdk-test: sdk-test-python sdk-test-go sdk-test-typescript

infra-validate:
	bash scripts/infra/validate.sh
