# Subtrate Makefile
# Common commands for building, testing, and development

# Default target
.DEFAULT_GOAL := build

# SQLite FTS5 requires CGO with specific flags.
# Export these so all go commands in this Makefile use them.
export CGO_ENABLED := 1
export CGO_CFLAGS := -DSQLITE_ENABLE_FTS5

# Variables
PKG := ./...
TIMEOUT := 5m
FRONTEND_DIR := web/frontend

GOTEST := go test
GOLIST := go list $(PKG)
XARGS := xargs -L 1

# Test commands: pipe package list through xargs to test one package at a
# time, avoiding parallel compilation races with the race detector.
UNIT := $(GOLIST) | $(XARGS) env $(GOTEST) -v -timeout $(TIMEOUT)
UNIT_RACE := $(UNIT) -race
COVER_FLAGS := -coverprofile=coverage.out -covermode=atomic

# Build targets
.PHONY: build
build:
	go build $(PKG)

.PHONY: build-cli
build-cli:
	go build -o substrate ./cmd/substrate

.PHONY: build-daemon
build-daemon:
	go build -o substrated ./cmd/substrated

.PHONY: build-all
build-all: build-cli build-daemon

# Frontend targets
.PHONY: bun-install
bun-install:
	cd $(FRONTEND_DIR) && bun install

.PHONY: bun-build
bun-build:
	cd $(FRONTEND_DIR) && bun run build

.PHONY: bun-dev
bun-dev:
	cd $(FRONTEND_DIR) && bun run dev

.PHONY: bun-test
bun-test:
	cd $(FRONTEND_DIR) && bun run test

.PHONY: bun-test-e2e
bun-test-e2e:
	cd $(FRONTEND_DIR) && bun run test:e2e

# E2E test with full rebuild (frontend + daemon).
# Cleans test data and runs E2E tests against production build.
.PHONY: bun-test-e2e-full
bun-test-e2e-full: bun-build build-daemon
	@rm -rf .test-data
	cd $(FRONTEND_DIR) && PLAYWRIGHT_USE_PRODUCTION=true bun run test:e2e

# E2E test specific file with full rebuild.
# Usage: make bun-test-e2e-file file=tests/e2e/inbox/message-delete.spec.ts
.PHONY: bun-test-e2e-file
bun-test-e2e-file: bun-build build-daemon
	@rm -rf .test-data
	cd $(FRONTEND_DIR) && PLAYWRIGHT_USE_PRODUCTION=true bun run test:e2e $(file) --project=chromium

# E2E test without rebuild (for iterating on tests only).
.PHONY: bun-test-e2e-quick
bun-test-e2e-quick:
	@rm -rf .test-data
	cd $(FRONTEND_DIR) && PLAYWRIGHT_USE_PRODUCTION=true bun run test:e2e --project=chromium

.PHONY: bun-lint
bun-lint:
	cd $(FRONTEND_DIR) && bun run lint

# Build daemon with embedded frontend (production build).
.PHONY: build-production
build-production: bun-build build-daemon
	@echo "Production build complete with embedded frontend."

# Full test suite (Go + frontend).
.PHONY: test-all
test-all: test bun-test
	@echo "All tests passed (Go + frontend)."

# Full lint (Go + frontend).
.PHONY: lint-all
lint-all: lint bun-lint
	@echo "All linting passed (Go + frontend)."

# Development mode: run Go backend and frontend dev server concurrently.
# Use with 'make dev' - requires terminal multiplexer or run in separate terminals.
.PHONY: dev
dev:
	@echo "Starting development servers..."
	@echo "  Backend:  http://localhost:$(WEB_PORT)"
	@echo "  Frontend: http://localhost:5174 (proxies to backend)"
	@echo ""
	@echo "Run these in separate terminals:"
	@echo "  Terminal 1: make run"
	@echo "  Terminal 2: make bun-dev"

# CI targets - used by GitHub Actions.
.PHONY: ci-go
ci-go: tidy-check lint test-race
	@echo "Go CI checks passed."

.PHONY: ci-frontend
ci-frontend: bun-install bun-lint bun-test
	@echo "Frontend CI checks passed."

.PHONY: ci
ci: ci-go ci-frontend
	@echo "All CI checks passed."

.PHONY: install
install: bun-install bun-build
	go install ./cmd/substrate
	go install ./cmd/substrated

# Testing targets
.PHONY: test
test:
	$(UNIT)

.PHONY: test-race
test-race:
	env GORACE="history_size=7 halt_on_errors=1" $(UNIT_RACE)

.PHONY: test-cover
test-cover:
	$(GOTEST) $(COVER_FLAGS) -v $(PKG)

.PHONY: test-cover-html
test-cover-html:
	$(GOTEST) -coverprofile=coverage.out $(PKG)
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Single package testing.
# Usage: make unit pkg=./internal/mail case=TestService_SendMail
.PHONY: unit
unit:
ifdef case
	$(GOTEST) -v -timeout $(TIMEOUT) -run $(case) $(pkg)
else
	$(GOTEST) -v -timeout $(TIMEOUT) $(pkg)
endif

# Run a specific test with verbose output.
# Usage: make run-test test=TestThreadFSM_UnreadToRead pkg=./internal/mail
.PHONY: run-test
run-test:
	$(GOTEST) -v -timeout $(TIMEOUT) -run $(test) $(pkg)

# Code generation
.PHONY: sqlc
sqlc:
	sqlc generate

.PHONY: sqlc-docker
sqlc-docker:
	docker run --rm -v $$(pwd):/src -w /src sqlc/sqlc generate

.PHONY: proto
proto:
	cd internal/api/grpc && ./gen_protos.sh

.PHONY: proto-check
proto-check:
	@command -v protoc >/dev/null 2>&1 || (echo "Error: protoc not found. Install with: brew install protobuf" && exit 1)
	@command -v protoc-gen-go >/dev/null 2>&1 || (echo "Error: protoc-gen-go not found. Install with: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest" && exit 1)
	@command -v protoc-gen-go-grpc >/dev/null 2>&1 || (echo "Error: protoc-gen-go-grpc not found. Install with: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest" && exit 1)
	@echo "All proto tools are installed."

.PHONY: proto-install
proto-install:
	@echo "Installing protobuf compiler and Go plugins..."
	brew install protobuf || true
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Done. Tools installed to GOPATH/bin."

.PHONY: gen
gen: sqlc proto
	@echo "All code generation complete."

# Code quality
.PHONY: lint
lint:
	golangci-lint run

.PHONY: fmt
fmt:
	go fmt $(PKG)
	gofumpt -w .

.PHONY: vet
vet:
	go vet $(PKG)

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: tidy-check
tidy-check:
	go mod tidy
	git diff --exit-code go.mod go.sum

# Database
.PHONY: migrate-up
migrate-up:
	@echo "Run migrations manually with: sqlite3 path/to/db.sqlite < internal/db/migrations/000001_init.up.sql"

.PHONY: migrate-down
migrate-down:
	@echo "Run migrations manually with: sqlite3 path/to/db.sqlite < internal/db/migrations/000001_init.down.sql"

# Clean
.PHONY: clean
clean:
	rm -f substrate substrated
	rm -f coverage.out coverage.html
	rm -rf $(FRONTEND_DIR)/dist
	rm -rf $(FRONTEND_DIR)/node_modules/.vite

# Server execution
WEB_PORT ?= 8080

# Default run target: web-only mode (no MCP/stdio)
.PHONY: run
run: build-daemon
	./substrated --web-only --web :$(WEB_PORT)

# Run with MCP support (for Claude Code integration via stdio)
.PHONY: run-mcp
run-mcp: build-daemon
	./substrated --web :$(WEB_PORT)

# Aliases for backwards compatibility
.PHONY: run-web
run-web: run

.PHONY: run-web-dev
run-web-dev:
	go run ./cmd/substrated --web-only --web :$(WEB_PORT)

# Start web server in background (via substrated in web-only mode).
.PHONY: start
start: build-daemon
	@echo "Starting Substrate web server on port $(WEB_PORT)..."
	@./substrated -web-only -web :$(WEB_PORT) &
	@sleep 1
	@echo "Server started. PID: $$(lsof -ti :$(WEB_PORT))"

# Stop web server.
.PHONY: stop
stop:
	@echo "Stopping web server on port $(WEB_PORT)..."
	@-lsof -ti :$(WEB_PORT) | xargs kill 2>/dev/null || true
	@sleep 1
	@-lsof -ti :$(WEB_PORT) | xargs kill -9 2>/dev/null || true
	@echo "Server stopped."

# Restart web server (stop, rebuild, start).
.PHONY: restart
restart: stop build-daemon
	@echo "Starting Substrate web server on port $(WEB_PORT)..."
	@./substrated -web-only -web :$(WEB_PORT) &
	@sleep 1
	@echo "Server restarted. PID: $$(lsof -ti :$(WEB_PORT))"

# Integration testing
.PHONY: test-integration
test-integration:
	go test -v -timeout 10m ./tests/integration/...

.PHONY: test-integration-sdk
test-integration-sdk:
	go test -v -timeout 10m ./tests/integration/sdk/...

.PHONY: test-integration-e2e
test-integration-e2e:
	go test -v -timeout 2m ./tests/integration/e2e/...

.PHONY: test-integration-short
test-integration-short:
	go test -v -short ./tests/integration/sdk/...

.PHONY: test-integration-seed
test-integration-seed:
	@echo "Seeding test database..."
	@mkdir -p /tmp/subtrate-test
	@rm -f /tmp/subtrate-test/test.db
	sqlite3 /tmp/subtrate-test/test.db < internal/db/migrations/000001_init.up.sql
	sqlite3 /tmp/subtrate-test/test.db < tests/integration/fixtures/seed_data.sql
	@echo "Test database created at /tmp/subtrate-test/test.db"

# Development helpers
.PHONY: check
check: fmt vet lint test

.PHONY: pre-commit
pre-commit: tidy fmt vet lint test-race
	@echo "All pre-commit checks passed!"

# Quick build check (just compile, no tests)
.PHONY: quick
quick:
	go build $(PKG)
	@echo "Build successful!"

# Help
.PHONY: help
help:
	@echo "Subtrate Makefile"
	@echo ""
	@echo "Build targets:"
	@echo "  build          Build all packages (default)"
	@echo "  build-cli      Build CLI binary (./substrate)"
	@echo "  build-daemon   Build daemon binary (./substrated with web UI)"
	@echo "  build-all      Build all binaries (CLI + daemon)"
	@echo "  build-production Build daemon with embedded frontend"
	@echo "  install        Install binaries to GOPATH/bin"
	@echo "  quick          Quick build check (compile only)"
	@echo ""
	@echo "Testing targets:"
	@echo "  test           Run all tests with verbose output"
	@echo "  test-cover     Run tests with coverage summary"
	@echo "  test-cover-html Generate HTML coverage report"
	@echo "  unit           Run tests for a single package"
	@echo "                 Usage: make unit pkg=./internal/mail"
	@echo "                 Usage: make unit pkg=./internal/mail case=TestService"
	@echo "  run-test       Run a specific test"
	@echo "                 Usage: make run-test test=TestThreadFSM pkg=./internal/mail"
	@echo ""
	@echo "Integration testing:"
	@echo "  test-integration       Run all integration tests (SDK + e2e)"
	@echo "  test-integration-sdk   Run SDK integration tests (requires claude CLI)"
	@echo "  test-integration-e2e   Run e2e backend tests (no external deps)"
	@echo "  test-integration-short Run short integration tests (no API calls)"
	@echo "  test-integration-seed  Create seeded test database"
	@echo ""
	@echo "Code generation:"
	@echo "  sqlc           Generate sqlc code (requires sqlc installed)"
	@echo "  sqlc-docker    Generate sqlc code via Docker"
	@echo "  proto          Generate gRPC code from protos"
	@echo "  proto-check    Verify proto tools are installed"
	@echo "  proto-install  Install protoc and Go plugins"
	@echo "  gen            Run all code generation (sqlc + proto)"
	@echo ""
	@echo "Code quality:"
	@echo "  lint           Run golangci-lint"
	@echo "  fmt            Format Go code"
	@echo "  vet            Run go vet"
	@echo "  tidy           Run go mod tidy"
	@echo "  tidy-check     Check if go mod tidy would change anything"
	@echo ""
	@echo "Server execution:"
	@echo "  run            Build and run in web-only mode (default, port 8080)"
	@echo "  run-mcp        Build and run with MCP support (for Claude Code)"
	@echo "  run-web-dev    Run in web-only mode without building (for dev)"
	@echo "  start          Build and start in background"
	@echo "  stop           Stop running server"
	@echo "  restart        Stop, rebuild, and start server"
	@echo "                 Usage: make restart WEB_PORT=8081"
	@echo ""
	@echo "Frontend targets:"
	@echo "  bun-install    Install frontend dependencies"
	@echo "  bun-build      Build frontend for production"
	@echo "  bun-dev        Start frontend dev server"
	@echo "  bun-test       Run frontend unit/integration tests"
	@echo "  bun-test-e2e   Run frontend E2E tests with Playwright"
	@echo "  bun-lint       Lint frontend code"
	@echo ""
	@echo "Combined targets:"
	@echo "  dev            Show instructions for development mode"
	@echo "  test-all       Run all tests (Go + frontend)"
	@echo "  lint-all       Run all linting (Go + frontend)"
	@echo "  ci             Run full CI pipeline (Go + frontend)"
	@echo "  ci-go          Run Go CI checks only"
	@echo "  ci-frontend    Run frontend CI checks only"
	@echo ""
	@echo "Development:"
	@echo "  check          Run fmt, vet, lint, and tests"
	@echo "  pre-commit     Run all pre-commit checks"
	@echo "  clean          Remove build artifacts"
	@echo ""
	@echo "Note: CGO_CFLAGS for SQLite FTS5 is set automatically by this Makefile."
	@echo "      You don't need to set it manually when using make commands."
