# Playwright MCP Test Scenarios

These test scenarios are designed to be executed by a Claude agent using the
Playwright MCP tools (`mcp__playwright__*`).

## Prerequisites

1. Start the web server:
   ```bash
   go run ./cmd/subtrate-web
   ```

2. Server runs on `http://localhost:8080` by default.

## Test Scenarios

### Scenario 1: Inbox Page Load

**Objective**: Verify the inbox page loads correctly with messages.

**Steps**:
1. Navigate to `http://localhost:8080/inbox`
2. Take a snapshot to verify page structure
3. Verify "Inbox" appears in navigation
4. Verify message list container exists
5. Verify at least one message row is displayed

**Expected Results**:
- Page title contains "Inbox"
- Navigation shows "Inbox" as active
- Message list has content
- Stats panel shows unread counts

**MCP Tools Used**:
- `mcp__playwright__browser_navigate`
- `mcp__playwright__browser_snapshot`

---

### Scenario 2: Message Row Interaction

**Objective**: Verify message rows are interactive.

**Steps**:
1. Navigate to inbox
2. Click on first message row
3. Verify thread view opens
4. Verify thread content displays

**Expected Results**:
- Clicking message opens thread view
- Thread shows sender name and body
- Back navigation works

**MCP Tools Used**:
- `mcp__playwright__browser_navigate`
- `mcp__playwright__browser_click`
- `mcp__playwright__browser_snapshot`

---

### Scenario 3: Star Message

**Objective**: Verify starring a message works.

**Steps**:
1. Navigate to inbox
2. Find star button on first message
3. Click star button
4. Verify visual feedback (star filled)

**Expected Results**:
- Star icon toggles state
- Message reflects starred status

**MCP Tools Used**:
- `mcp__playwright__browser_navigate`
- `mcp__playwright__browser_snapshot`
- `mcp__playwright__browser_click`

---

### Scenario 4: Compose Modal

**Objective**: Verify compose modal opens and functions.

**Steps**:
1. Navigate to inbox
2. Click "Compose" button
3. Verify modal appears
4. Fill in recipient, subject, body fields
5. Click "Send" (or close modal)

**Expected Results**:
- Modal opens with form fields
- Fields are editable
- Modal can be closed

**MCP Tools Used**:
- `mcp__playwright__browser_navigate`
- `mcp__playwright__browser_click`
- `mcp__playwright__browser_snapshot`
- `mcp__playwright__browser_fill_form`

---

### Scenario 5: Agents Dashboard

**Objective**: Verify agents dashboard displays agent information.

**Steps**:
1. Navigate to `http://localhost:8080/agents`
2. Verify dashboard stats are displayed
3. Verify agent cards are rendered
4. Verify activity feed shows entries

**Expected Results**:
- Stats cards show: Active Agents, Running Sessions, Pending Messages
- Agent cards display with status indicators
- Activity feed shows recent actions

**MCP Tools Used**:
- `mcp__playwright__browser_navigate`
- `mcp__playwright__browser_snapshot`

---

### Scenario 6: Agent Card Interaction

**Objective**: Verify agent cards are interactive.

**Steps**:
1. Navigate to agents dashboard
2. Click on an agent card
3. Verify agent details expand or navigate

**Expected Results**:
- Agent cards are clickable
- Details show session info if active

**MCP Tools Used**:
- `mcp__playwright__browser_navigate`
- `mcp__playwright__browser_click`
- `mcp__playwright__browser_snapshot`

---

### Scenario 7: Navigation Between Pages

**Objective**: Verify navigation works correctly.

**Steps**:
1. Navigate to inbox
2. Click "Agents" in sidebar
3. Verify agents dashboard loads
4. Click "Inbox" in sidebar
5. Verify inbox loads

**Expected Results**:
- Navigation updates URL
- Correct page content loads
- Active nav indicator updates

**MCP Tools Used**:
- `mcp__playwright__browser_navigate`
- `mcp__playwright__browser_click`
- `mcp__playwright__browser_snapshot`

---

### Scenario 8: HTMX Partial Loading

**Objective**: Verify HTMX partials load without full page refresh.

**Steps**:
1. Navigate to inbox
2. Observe initial page load
3. Trigger an HTMX request (e.g., filter messages)
4. Verify only partial content updates

**Expected Results**:
- Page does not fully reload
- Only target element updates
- Loading indicator shows during request

**MCP Tools Used**:
- `mcp__playwright__browser_navigate`
- `mcp__playwright__browser_click`
- `mcp__playwright__browser_snapshot`
- `mcp__playwright__browser_network_requests`

---

### Scenario 9: Thread View Navigation

**Objective**: Verify thread navigation controls work.

**Steps**:
1. Open a thread with multiple messages
2. Click "Next" to view next message
3. Click "Previous" to go back
4. Close thread view

**Expected Results**:
- Message index updates
- Content changes for each message
- Close returns to inbox

**MCP Tools Used**:
- `mcp__playwright__browser_navigate`
- `mcp__playwright__browser_click`
- `mcp__playwright__browser_snapshot`

---

### Scenario 10: Filter Agents by Status

**Objective**: Verify agent filtering works.

**Steps**:
1. Navigate to agents dashboard
2. Click "Active" filter tab
3. Verify only active agents shown
4. Click "Idle" filter tab
5. Verify only idle agents shown
6. Click "All" to reset

**Expected Results**:
- Filter tabs are interactive
- Agent list updates based on filter
- Counts reflect filtered results

**MCP Tools Used**:
- `mcp__playwright__browser_navigate`
- `mcp__playwright__browser_click`
- `mcp__playwright__browser_snapshot`

---

## Running a Test Scenario

To run a scenario, a Claude agent should:

1. Start the browser:
   ```
   mcp__playwright__browser_navigate to http://localhost:8080
   ```

2. Execute the steps in order, using appropriate MCP tools.

3. Capture snapshots at key points for verification.

4. Report any failures or unexpected behavior.

## Example Test Execution

```markdown
# Test: Inbox Page Load

## Step 1: Navigate to inbox
[mcp__playwright__browser_navigate url="http://localhost:8080/inbox"]

## Step 2: Capture snapshot
[mcp__playwright__browser_snapshot]

## Step 3: Verify elements
- [x] Page title contains "Inbox"
- [x] Navigation shows active indicator
- [x] Message list has content
- [x] Stats panel visible

## Result: PASS
```

## Assertions

Since Playwright MCP doesn't have built-in assertions, verification is done by:

1. Taking snapshots and analyzing the accessibility tree
2. Checking for expected text content
3. Verifying element presence via refs
4. Monitoring console for errors

Use `mcp__playwright__browser_console_messages` to check for JavaScript errors
after interactions.
