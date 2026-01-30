# Plan Mode Integration

Subtrate integrates with Claude Code's plan mode to enable asynchronous plan review via the mail system. When an agent is in plan mode and ready to exit, Subtrate intercepts the exit request, submits the plan for review, and blocks until the plan is approved.

## Overview

The plan mode workflow consists of:

1. **Plan Tracking** - A PostToolUse hook tracks when plan files are written
2. **Exit Interception** - A PreToolUse hook intercepts ExitPlanMode calls
3. **Plan Submission** - Plans are submitted as mail messages to a reviewer
4. **Async Review** - Reviewers can approve/reject via Subtrate web UI or mail
5. **Wait for Approval** - The agent can wait for the reviewer's decision

## Hook Scripts

### PostToolUse: Plan File Tracking

Location: `~/.claude/hooks/substrate/posttooluse_plan.sh`

This hook runs after any `Write` tool call and tracks writes to `~/.claude/plans/`:

```bash
# Tracks: tool_name == "Write" && file_path contains "/.claude/plans/"
# Stores: {session_id, plan_path, timestamp} in .claude/.substrate-plan-context
```

The context file is stored in the project directory at `.claude/.substrate-plan-context` using JSON lines format, allowing multiple sessions to track their plans independently.

### PreToolUse: ExitPlanMode Interception

Location: `~/.claude/hooks/substrate/pretooluse_plan.sh`

This hook runs before `ExitPlanMode` tool calls:

```bash
# When tool_name == "ExitPlanMode":
#   1. Calls: substrate plan submit --session-id "$session_id" --cwd "$cwd" --format hook
#   2. Returns: {"hookSpecificOutput": {"permissionDecision": "deny", ...}}
```

The `deny` decision prevents the normal ExitPlanMode flow while providing a message instructing the agent to wait for approval.

## CLI Commands

### substrate plan submit

Submit the current plan for review:

```bash
substrate plan submit [flags]
```

Flags:
- `--session-id` - Session ID (default: from $CLAUDE_SESSION_ID)
- `--cwd` - Working directory (default: current directory)
- `--file` - Path to plan file (default: from context file)
- `--to` - Reviewer agent name (default: "Reviewer")
- `--no-ai` - Skip AI summarization, use regex extraction

Output formats:
- `--format text` - Human-readable output
- `--format json` - Machine-readable JSON
- `--format hook` - JSON for PreToolUse hook response
- `--format context` - Minimal output for context injection

### substrate plan status

Check the approval status of a submitted plan:

```bash
substrate plan status [flags]
```

Flags:
- `--session-id` - Session ID
- `--cwd` - Working directory
- `--message-id` - Specific plan message ID

Returns one of: `pending`, `approved`, `rejected`, `needs_changes`, `replied`

### substrate plan wait

Block until the plan receives a reply:

```bash
substrate plan wait [flags]
```

Flags:
- `--session-id` - Session ID
- `--cwd` - Working directory
- `--message-id` - Specific plan message ID
- `--timeout` - Max wait time (e.g., "5m", "1h")

## AI Summarization

Plan submissions include an AI-generated summary using the Claude Agent SDK with the Haiku model. The summary:

1. Provides a concise 2-3 sentence overview of the plan
2. Lists key files/components affected
3. Describes the main approach

For plan revisions (detected via thread context), the summary also describes what changed from the prior version.

Fallback: If AI summarization fails or `--no-ai` is specified, a regex-based extraction is used to find the "Overview" section of the plan.

## Thread-Based Review Workflow

Plans use Subtrate's thread system for context preservation:

1. **Initial Submission** - Creates a new thread with subject `[PLAN] {title}`
2. **Reviewer Reply** - Reviewer responds in the same thread
3. **Plan Revision** - Revisions are submitted to the same thread with subject `[PLAN][REVISED] {title}`

The thread context allows reviewers to see the full history of plan iterations and feedback.

## Message Format

Plans are sent as structured messages:

```markdown
## Summary
{AI-generated or extracted summary}

## Files to Modify
{Extracted from plan if present}

---

## Full Plan
{Complete plan content}
```

## Integration with Hooks Install

Plan mode hooks are automatically installed via `substrate hooks install`:

```bash
# Install all hooks including plan mode
substrate hooks install

# Check installation status
substrate hooks status
```

The installation adds:
- Hook configuration to `~/.claude/settings.json`
- Hook scripts to `~/.claude/hooks/substrate/`

## Workflow Example

1. Agent enters plan mode:
   ```
   Agent: "I'll create a plan for implementing feature X"
   [Enters plan mode, writes ~/.claude/plans/plan-xxx.md]
   ```

2. PostToolUse hook captures the plan path

3. Agent requests to exit plan mode:
   ```
   Agent: [Calls ExitPlanMode tool]
   ```

4. PreToolUse hook intercepts:
   ```bash
   substrate plan submit --format hook
   # Returns deny decision with message
   ```

5. Agent receives message:
   ```
   "Plan submitted to Subtrate for review (message #42 to Reviewer).
   Run `substrate plan wait` to block until approved."
   ```

6. Agent can:
   - Call `substrate plan wait` to block
   - Continue working and check `substrate plan status` later

7. Reviewer sees plan in Subtrate web UI or via mail:
   ```
   [PLAN] Implement Feature X

   ## Summary
   This plan adds feature X by modifying the API handler...
   ```

8. Reviewer replies:
   ```
   "Approved. Good approach, proceed with implementation."
   ```

9. Agent receives approval:
   ```
   [Subtrate Plan] Plan approved!
   Reply from Reviewer: Approved. Good approach...
   ```

## Configuration

The following can be customized:

| Setting | Default | Description |
|---------|---------|-------------|
| Reviewer agent | "Reviewer" | Who receives plan submissions |
| AI model | claude-haiku-4 | Model for summarization |
| Poll interval | 5s | Wait command poll frequency |

## Troubleshooting

### Plan not tracked
- Ensure plan is written to `~/.claude/plans/`
- Check that PostToolUse hook is installed
- Verify `.claude/.substrate-plan-context` is created

### Submission fails
- Check Subtrate daemon is running
- Verify agent identity is set up
- Check network connectivity to Subtrate

### AI summarization fails
- Falls back to regex extraction automatically
- Use `--no-ai` to skip AI entirely
- Check Claude Agent SDK configuration
