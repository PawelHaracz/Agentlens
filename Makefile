# AgentLens Makefile
# Targets: lint, format, build, test, run

BINARY_NAME := agentlens
BUILD_DIR := bin
GO := go
CGO_ENABLED := 1
GOFLAGS := -v
COVERAGE_FILE := coverage.out
COVERAGE_HTML := coverage.html

# Go source files
GO_FILES := $(shell find . -name '*.go' -not -path './web/*')

.PHONY: all build test lint format run clean help \
        test-coverage test-race vet \
        web-install web-build web-lint \
        deps tools

## help: Show this help message
help:
	@echo "AgentLens Makefile targets:"
	@echo ""
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/  /'

## all: Run format, lint, test, and build
all: format lint test build

# ---------------------------------------------------------------------------
# Go targets
# ---------------------------------------------------------------------------

## build: Build the agentlens binary (CGO enabled for SQLite)
build:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/agentlens

## test: Run all Go tests
test:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test ./... $(GOFLAGS)

## test-coverage: Run tests with coverage report
test-coverage:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test ./... -coverprofile=$(COVERAGE_FILE) -covermode=atomic $(GOFLAGS)
	$(GO) tool cover -func=$(COVERAGE_FILE)

## test-coverage-html: Generate HTML coverage report
test-coverage-html: test-coverage
	$(GO) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)

## test-race: Run tests with race detector
test-race:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test ./... -race $(GOFLAGS)

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## vet: Run go vet
vet:
	$(GO) vet ./...

## format: Format Go source files
format:
	$(GO) fmt ./...
	gofmt -s -w .

## run: Build and run the server
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

## clean: Remove build artifacts and coverage files
clean:
	rm -rf $(BUILD_DIR)
	rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)

## deps: Download Go module dependencies
deps:
	$(GO) mod download
	$(GO) mod tidy

## tools: Install development tools (golangci-lint)
tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# ---------------------------------------------------------------------------
# Frontend targets
# ---------------------------------------------------------------------------

## web-install: Install frontend dependencies
web-install:
	cd web && npm ci

## web-build: Build the frontend
web-build:
	cd web && npm run build

## web-lint: Lint frontend (TypeScript check)
web-lint:
	cd web && npx tsc --noEmit
