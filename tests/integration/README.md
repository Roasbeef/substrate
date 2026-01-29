# Integration Tests

This directory contains two complementary integration testing approaches for Subtrate.

## Overview

### 1. Playwright MCP Tests (`playwright/`)

UI-driven tests using the Playwright MCP tools available in Claude Code. These tests
verify the HTMX frontend works correctly by simulating real user interactions.

**What they test:**
- Inbox page loading and message display
- Message actions (star, archive, snooze)
- Thread viewing
- Agent dashboard navigation
- Real-time SSE updates
- Compose modal functionality

**How to run:**
These tests are designed to be run by a Claude agent using the Playwright MCP tools.
See `playwright/scenarios.md` for the test scenarios.

### 2. Claude Agent SDK Tests (`sdk/`)

Go integration tests that spawn Claude agents via the SDK to interact with Subtrate.
These tests verify the programmatic API works correctly.

**What they test:**
- CLI commands (`substrate inbox`, `substrate send`, etc.)
- MCP tool integration
- Session continuation with SDK resume
- End-to-end message flow (send → receive → read)

**How to run:**
```bash
# Ensure substrated is running
go run ./cmd/substrated &

# Run SDK integration tests
CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go test -v ./tests/integration/sdk/...
```

## Test Fixtures

Both test suites share common fixtures:

- `fixtures/test_db.sql` - Pre-populated test database
- `fixtures/test_config.yaml` - Test configuration

## CI Integration

For CI, both test suites require:

1. **Playwright tests**: A running web server and browser automation setup
2. **SDK tests**: The substrated daemon running with a test database

See `.github/workflows/integration.yml` (to be created) for CI configuration.
