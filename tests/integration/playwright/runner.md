# Playwright Test Runner

This document provides instructions for running Playwright MCP tests.

## Quick Start

To run Playwright tests, a Claude agent should:

1. **Ensure the web server is running:**
   ```bash
   make run-web-dev
   ```

2. **Open the test scenarios file and execute each scenario.**

## Example Test Run

Below is an example of running Scenario 1 (Inbox Page Load):

### Step 1: Navigate to inbox
```
mcp__playwright__browser_navigate
url: http://localhost:8080/inbox
```

### Step 2: Take snapshot
```
mcp__playwright__browser_snapshot
```

### Step 3: Verify elements in snapshot
Check that the snapshot contains:
- Text "Inbox" in navigation
- Message list elements
- Stats showing unread counts

### Step 4: Check for errors
```
mcp__playwright__browser_console_messages
level: error
```

## Automated Test Execution

For automated testing, a Claude agent can be spawned with a prompt like:

```
You are a QA tester. Execute the Playwright integration tests for the Subtrate
web frontend. Follow these steps:

1. Read tests/integration/playwright/scenarios.md
2. Execute each scenario using the Playwright MCP tools
3. Record pass/fail for each scenario
4. Report any errors or unexpected behavior

The web server is running at http://localhost:8080.
```

## Test Results Format

Report test results in this format:

```markdown
# Playwright Test Results - [Date]

## Summary
- Total: N scenarios
- Passed: N
- Failed: N

## Results

### Scenario 1: Inbox Page Load
**Status**: PASS / FAIL
**Notes**: [Any observations]

### Scenario 2: Message Row Interaction
**Status**: PASS / FAIL
**Notes**: [Any observations]

...
```

## Troubleshooting

### Browser not installed
If you get "browser not installed" error:
```
mcp__playwright__browser_install
```

### Page not loading
1. Check if server is running: `curl http://localhost:8080`
2. Check server logs for errors
3. Verify port 8080 is not blocked

### Elements not found
1. Take a fresh snapshot
2. Check element refs in the snapshot
3. Verify HTMX has loaded (check for hx-* attributes)
