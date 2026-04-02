# Frontend Real-time SDK

Jua generates a TypeScript SDK under `apps/web/jua/realtime/` that wraps SSE and WebSocket into React hooks and context.

## Setup

Wrap your app (or the portion that needs real-time) with `RealtimeProvider`:

```tsx
// apps/web/app/layout.tsx
import { RealtimeProvider } from "@/jua/realtime/realtime-provider";

export default function RootLayout({ children }) {
  return (
    <html>
      <body>
        <RealtimeProvider>{children}</RealtimeProvider>
      </body>
    </html>
  );
}
```

`RealtimeProvider` opens a single SSE connection for the entire React tree and routes events to registered handlers. The connection is authenticated automatically using the JWT stored in `localStorage`.

## `useRealtime` — Subscribe to Events

```tsx
import { useRealtime } from "@/jua/realtime/realtime-provider";

export function OrderStatus({ orderId }) {
  const { on } = useRealtime();
  const [status, setStatus] = useState("pending");

  useEffect(() => {
    // on() returns an unsubscribe function
    return on("order.updated", (event) => {
      if (event.payload.order_id === orderId) {
        setStatus(event.payload.status);
      }
    });
  }, [orderId]);

  return <span>{status}</span>;
}
```

## `useSSE` — Raw SSE Hook

For cases where you need a dedicated SSE connection outside the shared provider:

```tsx
import { useSSE } from "@/jua/realtime/use-sse";

const { events, connected } = useSSE(["payment.completed", "payment.failed"]);
```

Options:

```ts
useSSE(eventTypes, {
  onEvent?: (event: RealtimeEvent) => void;
  onConnect?: () => void;
  onDisconnect?: () => void;
  autoReconnect?: boolean;  // default: true
  reconnectDelay?: number;  // ms, default: 3000
})
```

## `useWebSocket` — WebSocket Hook

```tsx
import { useWebSocket } from "@/jua/realtime/use-websocket";

const { connected, send, subscribe, unsubscribe } = useWebSocket();

// Subscribe to a channel
useEffect(() => {
  subscribe("order:42");
  return () => unsubscribe("order:42");
}, []);

// Send a custom event
send({ type: "custom_event", event: "cursor.moved", payload: { x, y } });
```

## `ConnectionStatus` — Visual Indicator

```tsx
import { ConnectionStatus } from "@/jua/realtime/connection-status";

// Shows green dot when connected, grey + "Reconnecting..." when not
<ConnectionStatus />
```

## `NotificationBell`

```tsx
import { NotificationBell } from "@/jua/notifications/notification-bell";

// Bell icon with unread badge, dropdown list, mark-read on click
<NotificationBell />
```

## File Reference

| File | Purpose |
|------|---------|
| `jua/realtime/realtime-provider.tsx` | `RealtimeProvider` context + `useRealtime()` hook |
| `jua/realtime/use-sse.ts` | Low-level `useSSE()` hook with auto-reconnect |
| `jua/realtime/use-websocket.ts` | `useWebSocket()` hook (subscribe/unsubscribe/send) |
| `jua/realtime/connection-status.tsx` | Connection indicator component |
| `jua/notifications/notification-bell.tsx` | Bell icon with dropdown |
| `jua/notifications/notification-preferences.tsx` | Channel toggles + quiet hours settings page |
| `jua/notifications/use-notifications.ts` | Unread count hook, SSE listener for new notifications |
