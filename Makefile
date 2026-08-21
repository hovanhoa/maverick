SHELL := /bin/bash

APP_NAME := api
API_CMD := ./cmd/api
SEED_CMD := ./cmd/seed
GO_PACKAGES := ./...
ENV_FILE := deployment/.env
ENV_EXAMPLE := deployment/.env.example
COMPOSE_FILE := deployment/docker-compose.yml

WEB_DIR := web
WEB_ENV_FILE := $(WEB_DIR)/.env.local
WEB_ENV_EXAMPLE := $(WEB_DIR)/.env.example

.PHONY: help env-check tidy fmt vet lint test test-race build run seed generate generate-graphql generate-openapi docker-up docker-down docker-logs clean web-env-check web-install web-dev web-build web-lint web-clean

help:
	@echo "Available targets:"
	@echo "  make help               - Show this help"
	@echo "  make env-check          - Verify deployment/.env exists"
	@echo "  make tidy               - Run go mod tidy"
	@echo "  make fmt                - Format Go code"
	@echo "  make vet                - Run go vet"
	@echo "  make lint               - Run fmt and vet"
	@echo "  make test               - Run all tests"
	@echo "  make test-race          - Run all tests with race detector"
	@echo "  make build              - Build API binary into ./bin/api"
	@echo "  make run                - Run API locally with deployment/.env"
	@echo "  make seed               - Seed the first OWNER account + API key"
	@echo "  make generate           - Run all code generation scripts"
	@echo "  make generate-graphql   - Run GraphQL generation script"
	@echo "  make generate-openapi   - Run OpenAPI generation script"
	@echo "  make docker-up          - Start docker compose services"
	@echo "  make docker-down        - Stop docker compose services"
	@echo "  make docker-logs        - Tail docker compose logs"
	@echo "  make clean              - Remove local build artifacts"
	@echo "  make web-install        - Install web console dependencies"
	@echo "  make web-dev            - Run the web console locally (:5173)"
	@echo "  make web-build          - Build the web console for production"
	@echo "  make web-lint           - Type-check the web console"
	@echo "  make web-clean          - Remove web console build artifacts"

env-check:
	@if [ ! -f "$(ENV_FILE)" ]; then \
		echo "Missing $(ENV_FILE). Copy $(ENV_EXAMPLE) first."; \
		exit 1; \
	fi

tidy:
	go mod tidy

fmt:
	go fmt $(GO_PACKAGES)

vet:
	go vet $(GO_PACKAGES)

lint: fmt vet

test:
	go test $(GO_PACKAGES)

test-race:
	go test -race $(GO_PACKAGES)

build:
	@mkdir -p bin
	go build -o bin/$(APP_NAME) $(API_CMD)

run: env-check
	clear
	@set -a; source "$(ENV_FILE)"; set +a; go run $(API_CMD)

seed: env-check
	@set -a; source "$(ENV_FILE)"; set +a; go run $(SEED_CMD) $(ARGS)

generate: generate-graphql generate-openapi

generate-graphql:
	bash scripts/gqlgen.sh

generate-openapi:
	bash scripts/openapi.sh

docker-up: env-check
	docker compose --env-file "$(ENV_FILE)" -f "$(COMPOSE_FILE)" up -d

docker-down: env-check
	docker compose --env-file "$(ENV_FILE)" -f "$(COMPOSE_FILE)" down

docker-logs: env-check
	docker compose --env-file "$(ENV_FILE)" -f "$(COMPOSE_FILE)" logs -f

clean:
	rm -rf bin

web-env-check:
	@if [ ! -f "$(WEB_ENV_FILE)" ]; then \
		cp "$(WEB_ENV_EXAMPLE)" "$(WEB_ENV_FILE)"; \
		echo "Created $(WEB_ENV_FILE) from $(WEB_ENV_EXAMPLE)."; \
	fi

web-install:
	cd $(WEB_DIR) && npm install

web-dev: web-env-check
	cd $(WEB_DIR) && npm run dev

web-build: web-env-check
	cd $(WEB_DIR) && npm run build

web-lint:
	cd $(WEB_DIR) && npm run lint

web-clean:
	rm -rf $(WEB_DIR)/dist $(WEB_DIR)/node_modules
