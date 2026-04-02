# WebSockets

The WebSocket transport provides a bidirectional channel. Use it when the client needs to send messages to the server in real time (collaborative features, chat, live cursors).

## Endpoint

```
GET /api/realtime/ws?token=<jwt>   (upgrades to ws://)
```

Uses `gorilla/websocket` under the hood. JWT authentication follows the same query-param pattern as SSE.

## Client → Server Messages

| Type | Payload | Effect |
|------|---------|--------|
| `subscribe` | `{"channel":"order:42"}` | Add this connection to the `order:42` channel |
| `unsubscribe` | `{"channel":"order:42"}` | Remove from channel |
| `ping` | — | Server responds with `{"type":"pong"}` |
| `custom_event` | `{"event":"my.event","payload":{...}}` | Publishes to event bus (tenant-scoped) |

## Server → Client Messages

All events are JSON-encoded `Event` structs:

```json
{
  "id":        "01HX...",
  "type":      "order.updated",
  "tenantID":  "t1",
  "userID":    "u42",
  "channelID": "order:42",
  "payload":   { "status": "shipped" },
  "timestamp": "2026-04-02T12:00:00Z"
}
```

## Channel Subscriptions

Channels allow clients to subscribe to a narrow stream of events (e.g. a single order, a document, a chat room):

```json
// Subscribe
{"type":"subscribe","channel":"order:42"}

// Server now routes "order.updated" events for channel "order:42" to this connection
// Unsubscribe
{"type":"unsubscribe","channel":"order:42"}
```

Publishing to a channel from Go:

```go
events.PublishToChannel("order:42", tenantID, map[string]interface{}{
    "status": "delivered",
})
```

## Connection Lifecycle

1. Client opens `wss://api.example.com/api/realtime/ws?token=<jwt>`
2. Middleware validates JWT, injects `userID` and `tenantID` into context
3. Hub registers the connection in `byTenant`, `byUser` indexes
4. `wsReadPump` goroutine reads client messages; `wsWritePump` goroutine sends server events
5. On close: hub unregisters the connection, all channel subscriptions are cleared

## Ping / Pong

The server sends a WebSocket ping frame every 30 seconds. The client must respond with a pong (most WebSocket libraries do this automatically). If no pong is received within 60 seconds, the connection is closed.
