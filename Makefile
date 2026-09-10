.PHONY: help tidy test vet build run example redis up down logs clean

APP          := estimation-service
REDIS_ADDR   ?= 127.0.0.1:6379
ADDR         ?= :8080
LOG_LEVEL    ?= info
LOG_FORMAT   ?= text
DEFAULT_MODE ?= approximate
RETENTION_DAYS ?= 14
SEGMENTS     ?= sports:approximate,premium_users:exact,high_value_buyers:exact,casual_readers:approximate
BIN_DIR      := bin

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?##"}; {printf "  %-12s %s\n", $$1, $$2}'

tidy: ## Download and tidy Go modules
	go mod tidy

test: ## Run all unit tests
	go test ./...

vet: ## Run go vet
	go vet ./...

build: ## Build server binary to bin/
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -o $(BIN_DIR)/$(APP) ./cmd/server

run: ## Run ES (reads .env if present; needs Redis)
	go run ./cmd/server

example: ## Run programmatic Add/Count example (needs Redis)
	go run ./examples/usage.go

redis: ## Start Redis via docker compose
	docker compose up -d redis

up: ## Start Redis + Estimation Service via docker compose
	docker compose up -d --build

down: ## Stop docker compose services
	docker compose down

logs: ## Tail docker compose logs
	docker compose logs -f

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) coverage.out
