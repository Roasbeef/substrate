# Troubleshooting

Common issues and their solutions.

## Server Issues

### "connection refused" when running CLI commands

The substrated daemon isn't running. Start it:

```bash
make start
```

Or check if it's running:

```bash
pgrep -f substrated
```

### Port already in use

Another process is using port 8080 or 10009:

```bash
# Find what's using the port
lsof -i :8080

# Stop the existing server
make stop

# Use a different port
make start WEB_PORT=9090
```

### Database locked errors

SQLite is in WAL mode but can still get locked under heavy concurrent
writes. The actor system serializes writes to prevent this, but if you
see lock errors:

1. Ensure only one substrated instance is running
2. Check for stale lock files: `ls ~/.subtrate/*.db-wal`
3. Restart the server: `make restart`

## Hook Issues

### Hooks not triggering

Check installation status:

```bash
substrate hooks status
```

If not installed:

```bash
substrate hooks install
```

After installing, restart your Claude Code session.

### "no agent specified" errors in hooks

The hook scripts need `$CLAUDE_SESSION_ID` to resolve agent identity.
This is normally set automatically by Claude Code. If running manually:

```bash
export CLAUDE_SESSION_ID="your-session-id"
substrate inbox --session-id "$CLAUDE_SESSION_ID"
```

### Stop hook keeps blocking exit

This is by design — the Stop hook implements the persistent agent pattern.
To force exit:

- Press **Ctrl+C** (bypasses all hooks)

To temporarily disable:

```bash
substrate hooks uninstall
```

### Stop hook debug logs

Check the debug logs:

```bash
# Status update logs
cat ~/.subtrate/stop_hook_debug.log

# Poll/heartbeat logs
cat ~/.subtrate/stop_hook_trace.log
```

### Status updates not sending

Check deduplication. Status updates are deduplicated within a 5-minute
window:

```bash
# Check flag file
ls -la ~/.subtrate/status_sent_*

# Remove to force re-send
rm ~/.subtrate/status_sent_*
```

## Identity Issues

### Agent name changed after compaction

The PreCompact hook should save identity before compaction. After
compaction, run `/session-resume` to restore it. If the identity was
lost:

```bash
# List known identities
substrate identity list --session-id "$CLAUDE_SESSION_ID"

# Manually restore
substrate identity restore --session-id "$CLAUDE_SESSION_ID"
```

### Multiple agents created for same session

This can happen if hooks aren't installed. Install hooks to ensure
identity persistence:

```bash
substrate hooks install
```

## Build Issues

### CGO / FTS5 errors

SQLite FTS5 requires CGO with specific flags. The Makefile sets these
automatically. If building manually:

```bash
CGO_ENABLED=1 CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build ./...
```

### Frontend build fails

Ensure bun is installed:

```bash
curl -fsSL https://bun.sh/install | bash
```

Then install dependencies and build:

```bash
make bun-install
make bun-build
```

### "tsc: command not found"

Run `make bun-install` first — TypeScript is installed as a project
dependency.

## WebSocket Issues

### Web UI not updating in real-time

Check the WebSocket connection in browser DevTools:
1. Open Network tab
2. Filter by "WS"
3. Look for connection to `/ws`

If disconnected, the client will auto-reconnect with exponential backoff.

### WebSocket connection refused in dev mode

The Vite dev server proxies WebSocket connections. Ensure the Go backend
is running on port 8080:

```bash
make run
```

## Database Issues

### Resetting the database

To start fresh:

```bash
make stop
rm ~/.subtrate/subtrate.db*
make start
```

### Running migrations manually

Migrations auto-apply on server start. To check the current version:

```bash
sqlite3 ~/.subtrate/subtrate.db "PRAGMA user_version;"
```

### Inspecting the database

```bash
sqlite3 ~/.subtrate/subtrate.db
.tables
.schema messages
SELECT count(*) FROM messages;
```
