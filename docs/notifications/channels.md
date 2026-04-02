# Notification Channels

Jua's notification engine supports five delivery channels. Each channel is wired to a provider interface so you can swap implementations without changing application code.

## SMS

Delivered via the `SMSProvider` interface (same as the USSD SMS provider). Configure your provider in `apps/api/internal/providers/sms/`.

**Template variable:** `Body` — plain text, max 160 characters per segment.

```go
SMS: &notifications.SMSTemplate{
    Body: "Your OTP is {{otp}}. Valid for 5 minutes.",
},
```

**Best for:** OTPs, payment confirmations, security alerts.

## Email

Delivered via the Resend provider (configured in Phase 4). HTML templates live in `apps/api/emails/`.

```go
Email: &notifications.EmailTemplate{
    Subject:  "Your payment of {{amount}} was received",
    HTMLFile: "payment_received.html",
    Text:     "Payment of {{amount}} received. Reference: {{reference}}.",
},
```

HTML templates use Go's `html/template` syntax for variable substitution.

**Best for:** Receipts, weekly digests, onboarding sequences.

## Push

Delivered via the `PushProvider` interface. Supports FCM (Android/web) and APNS (iOS). Push tokens are stored per user-device in `jua_push_tokens`.

```go
Push: &notifications.PushTemplate{
    Title: "Payment received",
    Body:  "UGX {{amount}} received from {{sender}}",
    Icon:  "/icons/payment.png",
    URL:   "/payments/{{payment_id}}",
},
```

Register a token from the frontend:

```http
POST /api/notifications/push-tokens
{"token": "<fcm-token>", "platform": "web", "device_id": "<uuid>"}
```

**Best for:** Mobile/PWA real-time alerts while the app is in the background.

## WhatsApp

Delivered via the `WhatsAppProvider` interface. Requires a WhatsApp Business API account.

```go
WhatsApp: &notifications.WhatsAppTemplate{
    Body: "Hello {{name}}, your order #{{order_id}} has shipped.",
},
```

WhatsApp is **opt-in only** — users must explicitly enable it in preferences. Off by default.

**Best for:** High-value transactional messages in markets where WhatsApp is dominant.

## In-App

Delivered instantly via the event bus and SSE/WebSocket. No external provider needed. Stored in `jua_notification_records` with `channel = "inapp"`.

```go
InApp: &notifications.InAppTemplate{
    Title:   "Payment received",
    Message: "UGX {{amount}} from {{sender}}",
    Icon:    "check-circle",
    URL:     "/payments",
},
```

The `NotificationBell` component listens for `notification.new` events and updates the unread count in real time. Clicking a notification marks it as read.

**Best for:** Dashboard alerts, status updates, anything the user can see while logged in.

## Delivery Order

When `Engine.Send()` is called:

1. **In-App** — synchronous, delivered before `Send()` returns
2. **SMS** — goroutine, fires immediately
3. **Email** — goroutine, fires immediately
4. **Push** — goroutine, fires immediately
5. **WhatsApp** — goroutine, fires immediately

Each goroutine writes a `NotificationLogRecord` with status `sent` or `failed` and an error message if applicable.

## Retry Failed Deliveries

Admins can retry a failed delivery from the admin panel (`/admin/notifications`) or via API:

```http
POST /api/admin/notifications/:id/retry
Authorization: Bearer <admin-jwt>
```
