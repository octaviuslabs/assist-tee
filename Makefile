.PHONY: help build build-api build-runtime build-runtime-deno build-runtime-bun run stop clean test-setup test-setup-bun test-execute logs

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build-api: ## Build the API service image for native platform
	cd services/api && docker build --platform linux/arm64 -t tee-api:latest .

build-runtime-deno: ## Build the Deno runtime image for native platform
	cd services/runtime && docker build --platform linux/arm64 -t deno-runtime:latest .

build-runtime-bun: ## Build the Bun runtime image for native platform
	cd services/runtime-bun && docker build --platform linux/arm64 -t bun-runtime:latest .

build-runtime: build-runtime-deno build-runtime-bun ## Build all runtime images

build: build-runtime build-api ## Build all service images

run: build ## Build and start all services (with gVisor)
	docker-compose up -d
	@echo "Waiting for services to be ready..."
	@sleep 5
	@echo "Services are running!"
	@echo "API: http://localhost:8080"
	@echo "Health check: curl http://localhost:8080/health"

run-dev: build ## Build and start services in dev mode (gVisor disabled - macOS/Windows)
	docker-compose -f docker-compose.dev.yml up -d
	@echo "Waiting for services to be ready..."
	@sleep 5
	@echo ""
	@echo "⚠️  WARNING: Running in DEV mode with gVisor DISABLED!"
	@echo "⚠️  This is NOT secure - only use for development!"
	@echo ""
	@echo "Services are running!"
	@echo "API: http://localhost:8080"
	@echo "Health check: curl http://localhost:8080/health"

stop: ## Stop all services
	docker-compose down || docker-compose -f docker-compose.dev.yml down

clean: ## Stop services and remove volumes
	docker-compose down -v || docker-compose -f docker-compose.dev.yml down -v
	docker volume prune -f

logs: ## Follow logs from all services
	docker-compose logs -f

logs-api: ## Follow logs from API service only
	docker-compose logs -f tee-api

test-setup: ## Test creating an environment (Deno runtime)
	@echo "Creating test environment (Deno)..."
	@curl -X POST http://localhost:8080/environments/setup \
		-H "Content-Type: application/json" \
		-d '{"mainModule":"main.ts","modules":{"main.ts":"export async function handler(event,context){return {sum:event.data.a+event.data.b,executionId:context.executionId};}"},"ttlSeconds":3600}' \
		| jq

test-setup-bun: ## Test creating an environment (Bun runtime)
	@echo "Creating test environment (Bun)..."
	@curl -X POST http://localhost:8080/environments/setup \
		-H "Content-Type: application/json" \
		-d '{"mainModule":"main.ts","runtime":"bun","modules":{"main.ts":"export async function handler(event,context){return {sum:event.data.a+event.data.b,executionId:context.executionId};}"},"ttlSeconds":3600}' \
		| jq

test-execute: ## Execute in an environment (set ENV_ID first)
	@if [ -z "$(ENV_ID)" ]; then \
		echo "Error: ENV_ID not set. Usage: make test-execute ENV_ID=<uuid>"; \
		exit 1; \
	fi
	@echo "Executing in environment $(ENV_ID)..."
	@curl -X POST http://localhost:8080/environments/$(ENV_ID)/execute \
		-H "Content-Type: application/json" \
		-d '{"data":{"a":5,"b":3},"env":{"DEBUG":"true"},"limits":{"timeoutMs":5000,"memoryMb":128}}' \
		| jq

list: ## List all environments
	@curl -s http://localhost:8080/environments | jq

delete: ## Delete an environment (set ENV_ID first)
	@if [ -z "$(ENV_ID)" ]; then \
		echo "Error: ENV_ID not set. Usage: make delete ENV_ID=<uuid>"; \
		exit 1; \
	fi
	@curl -X DELETE http://localhost:8080/environments/$(ENV_ID)
	@echo "Deleted environment $(ENV_ID)"

dev: ## Run in development mode with auto-reload
	docker-compose up --build

check-gvisor: ## Check if gVisor is installed and working
	@echo "Checking gVisor installation..."
	@which runsc || (echo "runsc not found. Please install gVisor." && exit 1)
	@docker run --rm --runtime=runsc hello-world && echo "gVisor is working!" || echo "gVisor test failed"
