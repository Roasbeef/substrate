# Subtrate Roadmap

This document outlines future improvements and alternative approaches for Subtrate.

## Current Architecture

```
Agents → CLI/MCP → Mail Service → SQLite
               ↓
          gRPC Server
```

## Future Improvements

### Transport Layer: NATS RPC

**Current**: gRPC for backend communication
**Future**: Consider NATS RPC as an alternative

#### Why NATS RPC?

1. **Built-in Pub/Sub**: NATS is inherently a pub/sub system, which aligns with Subtrate's messaging model
2. **Request-Reply**: Native request-reply pattern simpler than gRPC streaming
3. **JetStream**: Durable message storage with consumer offsets (exactly what Subtrate needs)
4. **Lightweight**: Single binary, no code generation needed
5. **Subject-Based Routing**: Natural fit for topic-based message routing

#### Migration Path

```
Phase 1 (Current): gRPC + Custom SQLite storage
Phase 2: Add NATS JetStream for real-time messaging
Phase 3: Consider replacing gRPC with NATS RPC entirely
```

#### NATS RPC Example

```go
// Server
nc, _ := nats.Connect("nats://localhost:4222")
nc.Subscribe("mail.send", func(m *nats.Msg) {
    var req SendMailRequest
    json.Unmarshal(m.Data, &req)

    resp := handleSendMail(req)
    respData, _ := json.Marshal(resp)
    m.Respond(respData)
})

// Client
resp, _ := nc.Request("mail.send", reqData, 5*time.Second)
```

#### Benefits for Subtrate

- **Topic subscriptions**: Map directly to NATS subjects
- **Consumer offsets**: JetStream provides durable consumer state
- **Broadcast**: NATS fan-out to multiple subscribers
- **Queue groups**: Load-balanced message delivery

### Frontend Improvements

#### Phase 1: HTMX (Current Plan)
- Server-rendered HTML with HTMX for interactivity
- Simpler deployment, works without JavaScript

#### Phase 2: React Components
- Inbox view with threading
- Real-time updates via SSE or WebSocket
- Gmail-like design

#### Phase 3: Native Apps
- Consider Tauri or Wails for desktop app
- Mobile app using React Native

### Scaling Considerations

#### Single-Node (Current)
- SQLite with WAL mode
- Good for ~10 agents, ~100K messages

#### Multi-Node (Future)
- Replace SQLite with PostgreSQL or CockroachDB
- Use NATS JetStream for message transport
- Consider embedding NATS server in substrated

### Security Enhancements

1. **Agent Authentication**: API keys or macaroons
2. **Message Encryption**: End-to-end encryption for sensitive messages
3. **Audit Logging**: Track all message operations
4. **Rate Limiting**: Prevent abuse from rogue agents

### Integration Improvements

1. **Claude Code Hooks**: Better integration with Claude Code lifecycle
2. **IDE Extensions**: VSCode extension for mail notifications
3. **Webhook Support**: HTTP callbacks for external integrations
4. **Email Gateway**: Bridge to traditional email systems

## Version Milestones

### v0.1.0 (Current)
- [x] Core mail service with actor pattern
- [x] SQLite storage with FTS5 search
- [x] CLI tool
- [x] MCP server for Claude Code
- [x] gRPC API
- [ ] HTMX frontend

### v0.2.0
- [ ] React inbox components
- [ ] Real-time message streaming
- [ ] Improved search
- [ ] Agent presence/status

### v0.3.0
- [ ] NATS JetStream integration
- [ ] Multi-node support
- [ ] Authentication/authorization
- [ ] Audit logging

### v1.0.0
- [ ] Production-ready
- [ ] Comprehensive documentation
- [ ] Performance benchmarks
- [ ] Migration tools

## Contributing

See CONTRIBUTING.md for how to contribute to Subtrate development.
