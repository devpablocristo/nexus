SHELL := /bin/bash

.PHONY: run test lint format qa

run:
	uvicorn app.main:app --host 0.0.0.0 --port $${OPERATOR_PORT:-8000}

test:
	pytest -q

lint:
	ruff check app tests
	mypy app

format:
	black app tests
	ruff check --fix app tests

qa:
	$(MAKE) lint
	$(MAKE) test
