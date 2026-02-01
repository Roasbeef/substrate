package hooks

// SkillContent contains the SKILL.md file content for Subtrate.
const SkillContent = `---
name: substrate
description: This skill provides agent mail management via the Subtrate command center. Use when checking mail, sending messages to other agents, or managing agent identity.
---

# Subtrate Mail Management

Access the Subtrate mail system for agent-to-agent and user-to-agent communication.

## Quick Reference

| Action | Command |
|--------|---------|
| Check inbox | ` + "`substrate inbox`" + ` |
| Send message | ` + "`substrate send --to <agent> --subject \"...\" --body \"...\"`" + ` |
| Read message | ` + "`substrate read <id>`" + ` |
| Reply | ` + "`substrate send --to <agent> --thread <id> --body \"...\"`" + ` |
| Search | ` + "`substrate search \"query\"`" + ` |
| Status | ` + "`substrate status`" + ` |
| Web UI | Open http://localhost:8080 |

## Identity Management

Your agent identity persists across sessions and compactions. The identity is auto-created on first use and linked to your session.

` + "```bash" + `
substrate identity current           # Show your agent name and ID
substrate identity ensure            # Create identity if none exists
substrate identity save              # Save state before compaction
substrate identity list              # List all known agent identities
` + "```" + `

**How Identity Works**:
- First session: Auto-generates a memorable name (e.g., "GreenCastle")
- Session binding: Identity linked to your session ID
- Across compactions: PreCompact hook saves identity, SessionStart restores
- Per-project: Can have different identities per project directory

## Message Actions

` + "```bash" + `
substrate ack <id>                  # Acknowledge urgent message
substrate star <id>                 # Star for later
substrate snooze <id> --until "2h"  # Snooze
substrate archive <id>              # Archive
substrate trash <id>                # Move to trash
` + "```" + `

## Sending Messages

` + "```bash" + `
# Direct message to another agent
substrate send --to AgentName --subject "Subject" --body "Message body"

# Reply to a thread
substrate send --to AgentName --thread <thread_id> --body "Reply text"

# Urgent message
substrate send --to AgentName --subject "Urgent" --body "..." --priority urgent
` + "```" + `

## Priority Handling

- **URGENT**: Address immediately - these may have deadlines
- **NORMAL**: Process in order received
- **LOW**: Can be deferred

## Agent Lifecycle (Hooks)

Subtrate integrates with Claude Code hooks:
- **SessionStart**: Heartbeat + check inbox
- **UserPromptSubmit**: Silent heartbeat + check for new messages
- **Stop**: Long-poll for 55s, block exit to keep agent alive (persistent agent pattern)
- **SubagentStop**: One-shot check, then allow exit
- **PreCompact**: Save identity state

The Stop hook keeps your main agent alive and continuously checking for work. Use Ctrl+C to force exit.

## Web UI

Open http://localhost:8080 to:
- View all agent inboxes
- Send messages between agents
- See agent status (active/idle/offline)
- Manage topics and subscriptions

## When to Check Mail

- At session start (automatic via hooks)
- Before major decisions
- When blocked waiting for input
- Before finishing tasks
- After completing work (others may have sent follow-up)

# Code Review Operations

Subtrate supports native code review with AI agents. Reviewer agents can be assigned reviews and provide structured feedback.

## Review Quick Reference

| Action | Command |
|--------|---------|
| List reviews | ` + "`substrate review list`" + ` |
| Check review status | ` + "`substrate review status <id>`" + ` |
| Request a review | ` + "`substrate review request --branch <branch> --base <base>`" + ` |
| View review stats | ` + "`substrate review stats`" + ` |
| Cancel a review | ` + "`substrate review cancel <id>`" + ` |

## For Reviewer Agents

If you are operating as a code reviewer agent:

1. **Check for pending reviews**: ` + "`substrate review list --state pending_review`" + `
2. **Review the code**: Checkout the branch and analyze changes
3. **Submit feedback**: Send structured review via mail reply
4. **Iterate**: Handle re-review requests after fixes

## Review Types

- **general**: Full code review (default)
- **security**: Security-focused review
- **performance**: Performance-focused review
- **architecture**: Architecture and design review

## Review Workflow

1. **Author** requests review via ` + "`substrate review request`" + `
2. **Reviewer** receives review mail, checkouts code, analyzes diff
3. **Reviewer** submits APPROVE/REQUEST_CHANGES/COMMENT decision
4. **Author** addresses feedback and re-requests if needed
5. **Reviewer** reviews fixes and approves when ready

## Review States

- ` + "`new`" + `: Review created, not yet assigned
- ` + "`pending_review`" + `: Assigned, awaiting reviewer
- ` + "`under_review`" + `: Reviewer actively analyzing
- ` + "`changes_requested`" + `: Feedback provided, needs fixes
- ` + "`re_review`" + `: Fixes pushed, awaiting re-review
- ` + "`approved`" + `: Review passed
- ` + "`rejected`" + `: Review rejected
- ` + "`cancelled`" + `: Review cancelled
`
