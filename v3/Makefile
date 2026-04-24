# Nexus v3
.PHONY: test qa smoke e2e acceptance up down build logs dev up-ponti-local down-ponti-local logs-ponti-local

LOCAL_INFRA_DIR := /home/pablo/Projects/Pablo/local-infra
DC := docker compose --project-directory $(CURDIR) -f $(LOCAL_INFRA_DIR)/docker-compose.yml -f $(LOCAL_INFRA_DIR)/docker-compose.ollama.yml -f $(CURDIR)/docker-compose.yml
DC_PONTI := docker compose --project-directory $(CURDIR) -f $(LOCAL_INFRA_DIR)/docker-compose.yml -f $(LOCAL_INFRA_DIR)/docker-compose.ollama.yml -f $(CURDIR)/docker-compose.yml -f $(CURDIR)/docker-compose.ponti.yml

# --- Quality ---
test:
	cd review && go test ./... -count=1
	cd companion && go test ./... -count=1

qa:
	bash scripts/quality/check-api.sh

# --- Docker ---
up:
	@test -f .env || cp .env.example .env
	$(DC) up -d --build

down:
	$(DC) down

build:
	$(DC) build

logs:
	$(DC) logs -f

# --- Tests contra API corriendo ---
smoke:
	bash scripts/smoke/run-policies-crud.sh
	bash scripts/smoke/run-requests-flow.sh
	bash scripts/smoke/run-companion-review-flow.sh
	bash scripts/smoke/run-companion-execution-flow.sh

e2e:
	bash scripts/e2e/run-full-lifecycle.sh

acceptance: smoke e2e

# --- Dev (hot reload) ---
dev:
	@test -f .env || cp .env.example .env
	docker compose --project-directory $(CURDIR) -f /home/pablo/Projects/Pablo/local-infra/docker-compose.yml -f /home/pablo/Projects/Pablo/local-infra/docker-compose.ollama.yml -f docker-compose.yml -f docker-compose.dev.yml up -d --build

dev-logs:
	docker compose --project-directory $(CURDIR) -f /home/pablo/Projects/Pablo/local-infra/docker-compose.yml -f /home/pablo/Projects/Pablo/local-infra/docker-compose.ollama.yml -f docker-compose.yml -f docker-compose.dev.yml logs -f

dev-down:
	docker compose --project-directory $(CURDIR) -f /home/pablo/Projects/Pablo/local-infra/docker-compose.yml -f /home/pablo/Projects/Pablo/local-infra/docker-compose.ollama.yml -f docker-compose.yml -f docker-compose.dev.yml down

# --- Ponti local (infra + nexus + ponti-backend + ponti-ai + frontend) ---
# Levanta todo el stack necesario para probar ponti contra Nexus local.
# Primera ejecucion puede tardar varios minutos (yarn install, go build, pip install).
up-ponti-local:
	@test -f .env || cp .env.example .env
	$(DC_PONTI) up -d --build
	@bash scripts/seed-ponti-policies.sh
	@echo ""
	@echo "Stack listo:"
	@echo "  - Nexus review   : http://localhost:18084"
	@echo "  - Nexus console  : http://localhost:13001"
	@echo "  - Ponti backend  : http://localhost:8080"
	@echo "  - Ponti AI       : http://localhost:8090"
	@echo "  - Ponti BFF      : http://localhost:3000"
	@echo "  - Ponti UI       : http://localhost:5173"

down-ponti-local:
	$(DC_PONTI) down

logs-ponti-local:
	$(DC_PONTI) logs -f ponti-backend ponti-ai ponti-ui ponti-bff review
