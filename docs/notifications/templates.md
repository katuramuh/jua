# Notification Templates

Templates define the content for each channel. A single template can contain variants for SMS, Email, Push, WhatsApp, and In-App.

## Built-in Templates

Jua ships 9 built-in templates registered automatically via `notifications.RegisterBuiltins()`:

| Name | Type | Channels |
|------|------|---------|
| `payment_received` | Transactional | SMS, Email, Push, InApp |
| `payment_failed` | Transactional | SMS, Email, Push, InApp |
| `otp_verification` | Security | SMS, Email |
| `subscription_activated` | Transactional | Email, Push, InApp |
| `subscription_expiring` | Operational | Email, Push, InApp |
| `subscription_expired` | Transactional | Email, Push, InApp |
| `login_new_device` | Security | SMS, Email, Push, InApp |
| `welcome` | Transactional | Email, InApp |
| `password_reset` | Security | Email |

## Template Structure

```go
type Template struct {
    Name        string
    Type        NotificationType
    Channels    []NotificationChannel
    SMS         *SMSTemplate
    Email       *EmailTemplate
    Push        *PushTemplate
    WhatsApp    *WhatsAppTemplate
    InApp       *InAppTemplate
}

type EmailTemplate struct {
    Subject  string   // supports {{variables}}
    HTMLFile string   // path to HTML file in apps/api/emails/
    Text     string   // plain-text fallback
}

type PushTemplate struct {
    Title string
    Body  string
    Icon  string
    URL   string
}

type InAppTemplate struct {
    Title   string
    Message string
    Icon    string
    URL     string
}
```

## Registering a Custom Template

```go
import "{{MODULE}}/jua/notifications/templates"

templates.Register(notifications.Template{
    Name:  "order_shipped",
    Type:  notifications.Transactional,
    Channels: []notifications.NotificationChannel{
        notifications.ChannelSMS,
        notifications.ChannelEmail,
        notifications.ChannelInApp,
    },
    SMS: &notifications.SMSTemplate{
        Body: "Your order #{{order_id}} has shipped. Track: {{tracking_url}}",
    },
    Email: &notifications.EmailTemplate{
        Subject:  "Order #{{order_id}} is on its way!",
        HTMLFile: "order_shipped.html",
    },
    InApp: &notifications.InAppTemplate{
        Title:   "Order shipped",
        Message: "Order #{{order_id}} is on its way.",
        Icon:    "truck",
        URL:     "/orders/{{order_id}}",
    },
})
```

## Generating a Template

Use the CLI to scaffold a new template:

```bash
jua generate notification OrderShipped \
  --type transactional \
  --channels "sms,email,inapp"
```

This creates:
- `apps/api/jua/notifications/templates/order_shipped.go` — Go template struct
- `apps/api/emails/order_shipped.html` — HTML email template

## Variable Substitution

Template strings support `{{variable}}` substitution. Pass variables in the `Data` map when calling `Send()`:

```go
notifications.GlobalEngine.Send(ctx, notifications.Notification{
    TemplateName: "order_shipped",
    Data: map[string]interface{}{
        "order_id":     "ORD-001",
        "tracking_url": "https://track.example.com/ORD-001",
    },
    ...
})
```
