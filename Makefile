APP_NAME      := promogo
MIGRATIONS_DIR := migrations/sql
DATABASE_URL  ?= postgres://promogo:promogo@localhost:5432/promogo?sslmode=disable

COMPOSE := docker compose -f deployments/docker-compose.yml

.PHONY: build run test lint tidy \
        docker-up docker-down docker-logs \
        migrate-up migrate-down migrate-status migrate-validate

build:
	go build -o bin/$(APP_NAME) ./cmd/$(APP_NAME)

run:
	go run ./cmd/$(APP_NAME)

test:
	go test ./...

lint:
	go vet ./...

tidy:
	go mod tidy

# ── Dev infrastructure ───────────────────────────────────────────────────────

docker-up:
	$(COMPOSE) up -d postgres redis

docker-down:
	$(COMPOSE) down

docker-logs:
	$(COMPOSE) logs -f app

# ── Migrations ───────────────────────────────────────────────────────────────

migrate-up:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" up

migrate-down:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" down

migrate-status:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" status

migrate-validate:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir $(MIGRATIONS_DIR) validate
