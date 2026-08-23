.PHONY: all build build-go web dev-web dev-api run test test-coverage lint fmt tidy clean help

BINARY_NAME=go-harness
BUILD_DIR=.

# Build version injected into binary (nearest git tag, or "dev")
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS=-s -w -X main.version=$(VERSION)

all: build

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

web: ## Build React Vite SPA into web/dist
	@echo "==> Building React frontend SPA..."
	cd web && pnpm install && pnpm build

build: web ## Build React frontend and compile single Go binary with embedded SPA
	@echo "==> Compiling Go binary with embedded web assets..."
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	@echo "==> Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

build-go: ## Compile Go binary without rebuilding web assets
	@echo "==> Compiling Go binary..."
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	@echo "==> Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

run: ## Run compiled Go harness binary
	$(BUILD_DIR)/$(BINARY_NAME)

dev-web: ## Start frontend development server (Vite with HMR)
	cd web && pnpm dev

dev-api: ## Run backend server directly with Go run
	go run main.go

test: ## Run all Go unit tests
	@echo "==> Running all Go package unit tests..."
	go test -v ./...

test-coverage: ## Run unit tests with HTML coverage report
	@echo "==> Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "==> Coverage report written to coverage.html"

lint: ## Run Go vet and Oxlint on frontend
	@echo "==> Running Go vet..."
	go vet ./...
	@echo "==> Running web linter..."
	cd web && pnpm lint

fmt: ## Format Go code and web sources
	@echo "==> Formatting Go code..."
	go fmt ./...

tidy: ## Run go mod tidy to clean up dependencies
	@echo "==> Tidying Go modules..."
	go mod tidy

clean: ## Remove build artifacts and temporary files
	@echo "==> Cleaning build artifacts..."
	rm -f $(BUILD_DIR)/$(BINARY_NAME) coverage.out coverage.html
	rm -rf web/dist
	@echo "==> Clean complete."
