# Notification Preferences

Every user has a `NotificationPreferences` record that controls which channels they receive notifications on and when.

## Model

```go
type NotificationPreferences struct {
    ID        uint      `gorm:"primarykey"`
    TenantID  string    `gorm:"not null;index"`
    UserID    string    `gorm:"not null;uniqueIndex:idx_tenant_user_prefs"`

    // Channel opt-ins (Operational + Marketing only)
    SMSEnabled       bool `gorm:"default:true"`
    EmailEnabled     bool `gorm:"default:true"`
    PushEnabled      bool `gorm:"default:true"`
    WhatsAppEnabled  bool `gorm:"default:false"`
    InAppEnabled     bool `gorm:"default:true"`

    // Marketing opt-in (explicit, separate from operational)
    MarketingEnabled bool `gorm:"default:false"`

    // Quiet hours
    QuietHoursEnabled bool   `gorm:"default:false"`
    QuietHoursStart   string `gorm:"default:'22:00'"` // HH:MM
    QuietHoursEnd     string `gorm:"default:'07:00'"` // HH:MM
    Timezone          string `gorm:"default:'Africa/Kampala'"`
}
```

## Quiet Hours

When `QuietHoursEnabled` is true, Operational and Marketing notifications are held until quiet hours end. Transactional and Security notifications **always** bypass quiet hours.

Quiet hours are stored in the user's local timezone. The engine converts to UTC for comparison. Midnight crossover is handled correctly:

- `22:00 → 07:00` (crosses midnight): delivery blocked if `current >= 22:00 OR current < 07:00`
- `09:00 → 17:00` (same day): delivery blocked if `current >= 09:00 AND current < 17:00`

## API

### Get preferences
```http
GET /api/notifications/preferences
Authorization: Bearer <jwt>
```

### Update preferences
```http
PUT /api/notifications/preferences
Authorization: Bearer <jwt>
Content-Type: application/json

{
  "sms_enabled": true,
  "email_enabled": true,
  "push_enabled": false,
  "whatsapp_enabled": false,
  "in_app_enabled": true,
  "marketing_enabled": false,
  "quiet_hours_enabled": true,
  "quiet_hours_start": "22:00",
  "quiet_hours_end": "07:00",
  "timezone": "Africa/Nairobi"
}
```

## Frontend Settings Page

`jua/notifications/notification-preferences.tsx` renders a full preferences page with:
- Toggle switches for each channel
- Quiet hours time pickers
- Timezone selector (IANA timezone list)

Mount it at any route in your settings section:

```tsx
// apps/web/app/settings/notifications/page.tsx
import NotificationPreferences from "@/jua/notifications/notification-preferences";

export default function NotificationSettingsPage() {
  return <NotificationPreferences />;
}
```
