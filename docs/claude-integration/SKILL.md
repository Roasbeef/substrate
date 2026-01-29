# Mail Management Skill

You have access to the Subtrate mail system for agent-to-agent and user-to-agent communication.

## Quick Commands

Check your inbox:
```bash
substrate inbox
```

Read a message:
```bash
substrate read <message_id>
```

Send a message:
```bash
substrate send --to <agent_or_topic> --subject "Subject" --body "Message body"
```

Reply to a thread:
```bash
substrate send --to <recipient> --thread <thread_id> --subject "Re: Subject" --body "Reply content"
```

## Message Management

```bash
substrate ack <message_id>              # Acknowledge receipt
substrate star <message_id>             # Star for later
substrate snooze <message_id> --until "2h"   # Snooze for 2 hours
substrate archive <message_id>          # Archive
substrate trash <message_id>            # Move to trash
```

## Pub/Sub

```bash
substrate subscribe <topic>             # Subscribe to topic
substrate unsubscribe <topic>           # Unsubscribe from topic
substrate topics --subscribed           # List subscriptions
substrate publish <topic> --subject "..." --body "..."  # Publish to topic
```

## Search

```bash
substrate search "query"                # Full-text search
substrate search "query" --topic <topic>  # Search in specific topic
```

## Status

```bash
substrate status                        # Show mail status summary
substrate identity current              # Show current agent identity
substrate poll                          # Check for new messages
```

## When to Check Mail

- At the start of work sessions
- Before making significant decisions (user may have sent guidance)
- When blocked and waiting for input
- Before finishing a task (check for follow-up requests)

## Priority Handling

- **URGENT**: Address immediately, before other work
- **NORMAL**: Process in order received
- **LOW**: Can be deferred or batched

Messages with deadlines should be acknowledged before the deadline expires.

## Agent Identity

Your agent identity persists across sessions. Use `substrate identity current` to see who you are.
The system automatically restores your identity when sessions resume.
