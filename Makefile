GO_IMAGE := golang:1.24.4
LINT_IMAGE := golangci/golangci-lint:v2.1.6
NODE_IMAGE := node:24.22.1-alpine@sha256:06d7adf3e0ef61a13f1d28682726366f97a15aa1888c934d3ecec86c33918b53
FRONTEND_RUN := docker run --rm -v "$(CURDIR)/frontend:/app" -v subscope_frontend_modules:/app/node_modules -v subscope_frontend_npm_cache:/root/.npm -w /app $(NODE_IMAGE)
FRONTEND_INSTALL := npm install --global npm@12.2.1 --no-audit --no-fund && npm ci --no-audit --no-fund
COMPOSE_ENV ?= .env
COMPOSE := docker compose --env-file "$(COMPOSE_ENV)"
DEMO_DURATION ?= 3m
DEMO_RPS ?= 10

.PHONY: help up down build logs test test-race test-integration lint check deploy frontend-build frontend-dev frontend-check frontend-test frontend-lint

help:
	@echo "up          build and start the full Docker Compose stack"
	@echo "down        stop containers and keep database volumes"
	@echo "logs        follow application logs"
	@echo "test        run Go unit tests in Docker"
	@echo "test-race   run Go unit tests with the race detector in Docker"
	@echo "test-integration  run Go race tests with disposable PostgreSQL and Redis"
	@echo "lint        run golangci-lint in Docker"
	@echo "check       run Go integration/race tests, lint, frontend checks and validate Compose"
	@echo "deploy      rebuild and start on the current Docker host"
	@echo "frontend-build  build React and export assets for local Go embed"
	@echo "frontend-dev    start Vite in Docker on localhost:5173"
	@echo "frontend-check  typecheck, lint, unit tests and production build in Docker"
	@echo "frontend-test   run frontend unit tests in Docker"
	@echo "frontend-lint   typecheck and lint the frontend in Docker"
	@echo "demo-traffic    send bounded read-only API traffic for Grafana"

up:
	$(COMPOSE) --profile observability up --build -d --wait

down:
	$(COMPOSE) --profile observability down

build:
	$(COMPOSE) build

logs:
	$(COMPOSE) logs -f analytics-service users-service subscriptions-service payments-service

test:
	docker run --rm -v "$(CURDIR):/src" -w /src $(GO_IMAGE) go test ./...

test-race:
	docker run --rm -e CGO_ENABLED=1 -v "$(CURDIR):/src" -w /src $(GO_IMAGE) go test -race ./...

# The runner also joins bridge so Go can download modules from outside the test network.
test-integration:
	@set -eu; \
	project=subscription-analytics-test-$$$$; \
	compose() { POSTGRES_DB=subscription_test POSTGRES_PASSWORD=local-test docker compose --env-file .env.example -p "$$project" "$$@"; }; \
	trap 'compose down --volumes' EXIT; \
	compose up -d --wait --wait-timeout 90 postgres redis; \
	docker run --rm --network "$${project}_backend" --network bridge \
		-e CGO_ENABLED=1 -e 'TEST_DATABASE_URL=postgres://postgres:local-test@postgres:5432/subscription_test?sslmode=disable' \
		-e TEST_REDIS_ADDR=redis:6379 -v "$(CURDIR):/src:ro" -w /src $(GO_IMAGE) go test -race -count=1 ./...

lint:
	docker run --rm -v "$(CURDIR):/src" -w /src $(LINT_IMAGE) golangci-lint run

check: test-integration lint frontend-check
	$(COMPOSE) --profile observability config --quiet

frontend-build:
	docker build --file docker/analytics-service.Dockerfile --target frontend-export --output type=local,dest=internal/app/web/dist .

frontend-dev:
	docker run --rm -p 127.0.0.1:5173:5173 --add-host host.docker.internal:host-gateway -e API_PROXY_TARGET=http://host.docker.internal:8080 -v "$(CURDIR)/frontend:/app" -v subscope_frontend_modules:/app/node_modules -v subscope_frontend_npm_cache:/root/.npm -w /app $(NODE_IMAGE) sh -c '$(FRONTEND_INSTALL) && npm run dev -- --host 0.0.0.0'

frontend-check:
	$(FRONTEND_RUN) sh -c '$(FRONTEND_INSTALL) && npm run typecheck && npm run lint && npm test && npm run build'

frontend-test:
	$(FRONTEND_RUN) sh -c '$(FRONTEND_INSTALL) && npm test'

frontend-lint:
	$(FRONTEND_RUN) sh -c '$(FRONTEND_INSTALL) && npm run typecheck && npm run lint'

deploy:
	$(COMPOSE) --profile observability up --build -d --wait

.PHONY: up-observability check-observability
up-observability:
	$(COMPOSE) --profile observability up --build -d --wait

check-observability:
	$(COMPOSE) --profile observability config --quiet
	$(COMPOSE) --profile observability exec -T prometheus promtool check config /etc/prometheus/prometheus.yml
	$(COMPOSE) --profile observability exec -T prometheus promtool check rules /etc/prometheus/alerts.yml

.PHONY: demo-traffic
demo-traffic:
	$(COMPOSE) --profile demo run --build --rm demo-traffic -duration "$(DEMO_DURATION)" -rps "$(DEMO_RPS)"
