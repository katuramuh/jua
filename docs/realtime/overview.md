# Real-time Engine — Overview

Jua's real-time engine gives every generated project a production-ready push layer from day one. It ships two transports (SSE and WebSocket), a multi-tenant event bus, and a React SDK — no configuration required.

## Architecture

```
Browser
  │  SSE  (EventSource)          ── primary push channel
  │  WS   (WebSocket)            ── bidirectional channel
  ▼
apps/api/jua/realtime/
  ├── hub.go          actor-model hub, 4 connection indexes
  ├── sse.go          SSE handler with missed-event replay
  ├── ws.go           WebSocket handler (gorilla/websocket)
  ├── middleware.go   JWT auth for EventSource/WebSocket
  └── routes.go       /api/realtime/* route registration

apps/api/jua/events/
  ├── bus.go          Bus interface + EventType constants
  ├── memory_bus.go   in-process bus (single instance)
  ├── redis_bus.go    Redis Pub/Sub bus (multi-instance)
  ├── publisher.go    Publish / PublishForUser / PublishToChannel
  └── init.go         reads REALTIME_BUS env, sets DefaultBus
```

## Event Bus

The event bus decouples publishers from consumers. Any part of your code can fire an event; the hub delivers it to all matching connections.

```go
// Publish a tenant-scoped event
events.Publish(events.PaymentCompleted, tenantID, map[string]interface{}{
    "amount": 5000,
    "currency": "UGX",
})

// Publish to a specific user
events.PublishForUser(events.OrderUpdated, tenantID, userID, payload)

// Publish to a named channel (e.g. a specific order room)
events.PublishToChannel("order:42", tenantID, payload)
```

Built-in event types:

| Constant | Value |
|----------|-------|
| `events.PaymentCompleted` | `payment.completed` |
| `events.PaymentFailed` | `payment.failed` |
| `events.SubscriptionActivated` | `subscription.activated` |
| `events.SubscriptionCancelled` | `subscription.cancelled` |
| `events.UserLoggedIn` | `user.logged_in` |
| `events.USSDSessionCompleted` | `ussd.session_completed` |
| `events.NotificationNew` | `notification.new` |
| `events.OrderUpdated` | `order.updated` |

## Hub

The hub is an actor-model fan-out router. It maintains four indexes for efficient delivery:

| Index | Used for |
|-------|----------|
| `byTenant` | broadcast to every connection in a tenant |
| `byUser` | deliver to all devices of one user |
| `byChannel` | deliver to a named room (e.g. `order:42`) |
| `all` | global broadcast (admin only) |

All mutation operations (`Register`, `Unregister`, `Broadcast`) go through buffered channels — the main `Run()` loop owns the maps, no mutex contention on the hot path.

## Transport Selection

| Scenario | Recommended transport |
|----------|-----------------------|
| Dashboard updates, notifications | SSE (simpler, works everywhere) |
| Collaborative editing, chat | WebSocket (bidirectional) |
| Behind Nginx / load balancer | SSE (HTTP/1.1 friendly) |

## Multi-instance Deployment

Set `REALTIME_BUS=redis` and all API pods share the same event stream via Redis Pub/Sub. Each pod routes events to its local connections.

```
Pod A ──publish──▶ Redis channel ──subscribe──▶ Pod A (local deliver)
                                 ──subscribe──▶ Pod B (local deliver)
                                 ──subscribe──▶ Pod C (local deliver)
```

With `REALTIME_BUS=memory` (default), events stay in-process — suitable for single-instance deployments and local development.

## Environment Variables

```env
REALTIME_BUS=redis            # "redis" | "memory" (default: memory)
REALTIME_MAX_CONNECTIONS=1000 # max simultaneous connections per pod
REALTIME_EVENT_TTL=300        # seconds to keep events in history (Redis)
```
