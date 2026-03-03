SHELL := /bin/bash

CORE_DIR := nexus-core
EXTERNAL_OPERATORS_DIR := nexus-operator
TOWER_DIR := nexus-tower
CORE_SERVICE := nexus-core

.PHONY: up build down clean logs migrate-up migrate-down cleanup-idempotency seed \
	core-test operator-test tower-test tower-qa qa e2e jwt-e2e quickstart-admin \
	core-dev control-dev operator-dev external-operators-dev tower-dev reset-nexus logs-tail \
	sdk-test-python sdk-test

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

cleanup-idempotency:
	docker compose exec -T $(CORE_SERVICE) /app/cleanup-idempotency

seed:
	@echo "Waiting for core to be healthy..."
	@docker compose up -d --wait $(CORE_SERVICE) postgres
	cd $(CORE_DIR) && NEXUS_COMPOSE_FILE=../docker-compose.yml bash scripts/seed_demo.sh

core-test:
	cd $(CORE_DIR) && go test ./...

operator-test:
	cd $(EXTERNAL_OPERATORS_DIR) && \
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

reset-nexus:
	$(MAKE) clean
	$(MAKE) build
	$(MAKE) up
	$(MAKE) migrate-up
	$(MAKE) seed
	$(MAKE) logs-tail

e2e:
	cd $(CORE_DIR) && bash scripts/e2e.sh

e2e-first:
	@# Primer caso: run echo (requiere: make up, make migrate-up, make seed)
	bash scripts/e2e/01_run_echo.sh

jwt-e2e:
	cd $(CORE_DIR) && bash scripts/e2e_jwt.sh

quickstart-admin:
	cd $(CORE_DIR) && bash scripts/quickstart_admin.sh

core-dev:
	cd $(CORE_DIR) && go run ./cmd/api

operator-dev:
	cd $(EXTERNAL_OPERATORS_DIR) && . .venv/bin/activate && uvicorn app.main:app --host 0.0.0.0 --port 8000

external-operators-dev: operator-dev

control-dev:
	cd $(CORE_DIR) && go run ./cmd/ops-workers

tower-dev:
	cd $(TOWER_DIR) && npm run dev

sdk-test-python:
	cd sdks/python-sdk && \
		if [ ! -d .venv ]; then python3 -m venv .venv; fi && \
		. .venv/bin/activate && \
		pip install -q -e '.[dev]' && \
		pytest -q

sdk-test: sdk-test-python
