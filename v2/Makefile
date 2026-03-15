SHELL := /bin/bash

COMPOSE := docker compose

.PHONY: test qa smoke e2e e2e-incidents smoke-run e2e-run smoke-control-plane smoke-control-workers smoke-all e2e-all acceptance up down logs status dev-up dev-down data-plane-dev control-plane-dev control-workers-dev echo-dev

test:
	cd data-plane && go test ./...
	cd control-plane && go test ./...
	cd control-workers && go test ./...

qa:
	./scripts/quality/check-crud-pattern.sh
	./scripts/quality/check-data-plane.sh
	./scripts/quality/check-control-plane.sh
	./scripts/quality/check-control-workers.sh

smoke:
	./scripts/smoke/run-actions-basic-flow.sh

smoke-run:
	./scripts/smoke/run-basic-flow.sh

smoke-control-plane:
	./scripts/smoke/run-control-plane-resources-flow.sh

smoke-control-workers:
	./scripts/smoke/run-control-workers-incidents-flow.sh

e2e:
	./scripts/e2e/run-actions-read-flow.sh

e2e-incidents:
	./scripts/e2e/run-actions-incidents-flow.sh

e2e-run:
	./scripts/e2e/run-approval-flow.sh

smoke-all: smoke smoke-run smoke-control-plane smoke-control-workers

e2e-all: e2e e2e-incidents e2e-run

acceptance: smoke-all e2e-all

up:
	cp -n .env.example .env >/dev/null 2>&1 || true
	$(COMPOSE) up -d --build --remove-orphans --wait

down:
	$(COMPOSE) down --remove-orphans

logs:
	$(COMPOSE) logs -f

status:
	$(COMPOSE) ps

dev-up:
	cp -n .env.example .env >/dev/null 2>&1 || true
	$(COMPOSE) -f docker-compose.yml -f docker-compose.dev.yml up -d --build --remove-orphans

dev-down:
	$(COMPOSE) -f docker-compose.yml -f docker-compose.dev.yml down --remove-orphans

data-plane-dev:
	cd data-plane && air -c .air.toml

control-plane-dev:
	./scripts/dev/run-control-plane.sh

control-workers-dev:
	./scripts/dev/run-control-workers.sh

echo-dev:
	./scripts/dev/run-echo-upstream.sh
