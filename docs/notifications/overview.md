# Notifications Centre — Overview

The Notifications Centre is a multi-channel delivery engine built into every Jua project. It handles SMS, Email, Push, WhatsApp, and In-App notifications with per-user preferences, quiet hours, and a full delivery audit log.

## Architecture

```
apps/api/jua/notifications/
  ├── types.go            NotificationType, NotificationChannel, structs
  ├── model.go            GORM models: NotificationRecord, PushToken
  ├── migration.go        AutoMigrate() for all tables
  ├── preferences.go      NotificationPreferences model + quiet hours
  ├── engine.go           Engine.Send() — orchestrates delivery
  ├── worker.go           per-channel goroutine deliverers
  ├── inapp.go            in-app delivery via event bus
  ├── template_registry.go global template registry
  ├── handlers.go         HTTP handlers (user + admin)
  ├── routes.go           RegisterRoutes()
  └── templates/
      ├── base.go         Template / EmailTemplate / PushTemplate structs
      └── builtin.go      9 built-in templates
```

## Notification Types

| Type | Channels | Can opt out? | Quiet hours |
|------|----------|--------------|-------------|
| Transactional | SMS + InApp + Push | No | Bypassed |
| Security | All channels | No | Bypassed |
| Operational | User preferences | Yes | Respected |
| Marketing | Opted-in only | Yes | Respected |

## Sending a Notification

```go
import "{{MODULE}}/jua/notifications"

err := notifications.GlobalEngine.Send(ctx, notifications.Notification{
    TenantID:     tenantID,
    UserID:       userID,
    Type:         notifications.Transactional,
    TemplateName: "payment_received",
    Data: map[string]interface{}{
        "amount":    "UGX 5,000",
        "reference": "PAY-001",
        "name":      "Alice",
    },
    Phone: "+256771234567",
    Email: "alice@example.com",
})
```

`Send()` will:
1. Persist a `NotificationRecord` to the database
2. Load the user's `NotificationPreferences`
3. Check quiet hours (skipped for Transactional/Security)
4. Deliver in-app immediately (via event bus → SSE/WS)
5. Dispatch other channels (SMS, Email, Push, WhatsApp) concurrently via goroutines
6. Log each delivery attempt in `NotificationLogRecord`

## Database Tables

All tables are prefixed with `jua_` to avoid conflicts with application tables:

| Table | Purpose |
|-------|---------|
| `jua_notification_records` | One row per notification sent |
| `jua_notification_logs` | One row per delivery attempt per channel |
| `jua_push_tokens` | FCM/APNS token per user+device |
| `jua_notification_preferences` | Per-user channel opt-ins + quiet hours |

## HTTP Routes

### User endpoints
```
GET    /api/notifications                  List my notifications (paginated)
GET    /api/notifications/unread-count     Unread badge count
PUT    /api/notifications/:id/read         Mark one as read
PUT    /api/notifications/read-all         Mark all as read
DELETE /api/notifications/:id              Delete one
GET    /api/notifications/preferences      Get my preferences
PUT    /api/notifications/preferences      Update my preferences
GET    /api/notifications/push-tokens      List my push tokens
POST   /api/notifications/push-tokens      Register push token
DELETE /api/notifications/push-tokens/:id  Remove push token
```

### Admin endpoints
```
GET  /api/admin/notifications              All notifications (all tenants)
POST /api/admin/notifications/broadcast    Send to all users in a tenant
GET  /api/admin/notifications/stats        Delivery stats
POST /api/admin/notifications/:id/retry    Retry a failed delivery
```

## Frontend Components

| Component | Location |
|-----------|---------|
| Notification bell + dropdown | `jua/notifications/notification-bell.tsx` |
| Preferences settings page | `jua/notifications/notification-preferences.tsx` |
| Unread count hook | `jua/notifications/use-notifications.ts` |
