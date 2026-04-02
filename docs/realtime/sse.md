# Server-Sent Events (SSE)

SSE is the primary real-time transport in Jua. It uses a standard HTTP connection that the server keeps open and streams events over. The browser reconnects automatically if the connection drops.

## Endpoint

```
GET /api/realtime/stream?token=<jwt>
```

Authentication uses a JWT passed as a query parameter because browsers cannot set custom headers on `EventSource` connections.

## Event Format

Each event follows the SSE wire format:

```
id: 01HX...
event: payment.completed
data: {"tenantID":"t1","userID":"u42","payload":{"amount":5000}}

```

- `id` — used by the browser as `Last-Event-ID` for missed-event replay on reconnect
- `event` — the `EventType` string (e.g. `payment.completed`)
- `data` — JSON-encoded event payload

Keepalive comments are sent every 30 seconds to keep proxies from closing idle connections:

```
: keepalive

```

## Missed Event Replay

When a client reconnects after a disconnect, the browser sends the `Last-Event-ID` header. The SSE handler fetches all events newer than that ID from a Redis sorted set (`jua:events:history:{tenantID}`) and replays them before resuming the live stream.

This means clients never miss events during brief network interruptions.

## Nginx Configuration

Add `X-Accel-Buffering: no` passthrough (already set by the handler) and increase the proxy read timeout:

```nginx
location /api/realtime/stream {
    proxy_pass         http://api;
    proxy_read_timeout 3600s;
    proxy_buffering    off;
    proxy_cache        off;
}
```

## Usage in Go (publishing)

```go
// In any service or handler:
events.Publish(events.OrderUpdated, tenantID, map[string]interface{}{
    "order_id": 42,
    "status":   "shipped",
})
```

The real-time hub receives the event and fans it out to all SSE connections belonging to that tenant.
