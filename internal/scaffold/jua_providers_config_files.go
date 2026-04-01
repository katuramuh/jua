package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"
)

func writeJuaProvidersConfigFiles(root string, opts Options) error {
	apiRoot := opts.APIRoot(root)
	module := opts.Module()
	_ = module

	files := map[string]string{
		filepath.Join(apiRoot, "internal", "providers", "config.go"):   providersConfigGo(),
		filepath.Join(apiRoot, "internal", "providers", "README.md"):   providersReadme(),
	}

	for path, content := range files {
		content = strings.ReplaceAll(content, "{{MODULE}}", module)
		if err := writeFile(path, content); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

func providersConfigGo() string {
	return `package providers

import (
	"{{MODULE}}/internal/providers/email"
	"{{MODULE}}/internal/providers/payment"
	"{{MODULE}}/internal/providers/push"
	"{{MODULE}}/internal/providers/sms"
	"{{MODULE}}/internal/providers/ussd"
	"{{MODULE}}/internal/providers/whatsapp"
)

// Config holds all provider instances for the application.
// Initialise with Init() and inject into your Services struct.
type Config struct {
	SMS      sms.SMSProvider
	Payment  payment.PaymentProvider
	WhatsApp whatsapp.WhatsAppProvider
	Email    email.EmailProvider
	Push     push.PushProvider
	USSD     ussd.Provider
}

// Init initialises all providers from environment variables.
// Providers that are not configured (missing env vars) are set to nil — check
// before use or your handlers will panic on the first request.
func Init() (*Config, []error) {
	var errs []error
	cfg := &Config{}

	if p, err := sms.Registry(); err != nil {
		errs = append(errs, err)
	} else {
		cfg.SMS = p
	}

	if p, err := payment.Registry(); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Payment = p
	}

	if p, err := whatsapp.Registry(); err != nil {
		errs = append(errs, err)
	} else {
		cfg.WhatsApp = p
	}

	if p, err := email.Registry(); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Email = p
	}

	if p, err := push.Registry(); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Push = p
	}

	if p, err := ussd.Registry(); err != nil {
		errs = append(errs, err)
	} else {
		cfg.USSD = p
	}

	return cfg, errs
}
`
}

func providersReadme() string {
	return `# Providers

This directory contains Jua's pluggable provider system for African + global services.
Each provider category has a common interface and multiple implementations.
Switch providers by setting a single environment variable — no code changes needed.

---

## SMS

**Interface:** ` + "`" + `internal/providers/sms.SMSProvider` + "`" + `

| Provider | SMTP_PROVIDER value | Required env vars |
|---|---|---|
| Africa's Talking | ` + "`" + `africastalking` + "`" + ` | ` + "`" + `AT_API_KEY` + "`" + `, ` + "`" + `AT_USERNAME` + "`" + `, ` + "`" + `AT_SENDER_ID` + "`" + ` |
| Relworx | ` + "`" + `relworx` + "`" + ` | ` + "`" + `RELWORX_API_KEY` + "`" + `, ` + "`" + `RELWORX_SENDER_ID` + "`" + ` |
| eSMS Africa | ` + "`" + `esmsafrica` + "`" + ` | ` + "`" + `ESMS_API_KEY` + "`" + `, ` + "`" + `ESMS_SENDER_ID` + "`" + ` |
| Twilio | ` + "`" + `twilio` + "`" + ` | ` + "`" + `TWILIO_ACCOUNT_SID` + "`" + `, ` + "`" + `TWILIO_AUTH_TOKEN` + "`" + `, ` + "`" + `TWILIO_PHONE_NUMBER` + "`" + ` |
| Vonage | ` + "`" + `vonage` + "`" + ` | ` + "`" + `VONAGE_API_KEY` + "`" + `, ` + "`" + `VONAGE_API_SECRET` + "`" + ` |
| Custom | ` + "`" + `custom` + "`" + ` | — |

---

## Payment

**Interface:** ` + "`" + `internal/providers/payment.PaymentProvider` + "`" + `

| Provider | PAYMENT_PROVIDER value | Best for |
|---|---|---|
| MTN MoMo | ` + "`" + `mtn` + "`" + ` | Uganda, Ghana, Zambia, Rwanda |
| Airtel Money | ` + "`" + `airtel` + "`" + ` | Uganda, Kenya, Tanzania, Malawi |
| Flutterwave | ` + "`" + `flutterwave` + "`" + ` | Kenya, Ghana, Pan-Africa |
| Paystack | ` + "`" + `paystack` + "`" + ` | Nigeria, Ghana, South Africa |
| Stripe | ` + "`" + `stripe` + "`" + ` | International / cards |
| PawaPay | ` + "`" + `pawapay` + "`" + ` | Pan-Africa aggregator |
| Custom | ` + "`" + `custom` + "`" + ` | — |

**Auto-router:** Use ` + "`" + `payment.NewRouter()` + "`" + ` to auto-select by E.164 phone prefix.

---

## WhatsApp

**Interface:** ` + "`" + `internal/providers/whatsapp.WhatsAppProvider` + "`" + `

| Provider | WHATSAPP_PROVIDER value | Required env vars |
|---|---|---|
| Meta Cloud API | ` + "`" + `meta` + "`" + ` | ` + "`" + `META_WHATSAPP_ACCESS_TOKEN` + "`" + `, ` + "`" + `META_WHATSAPP_PHONE_ID` + "`" + ` |
| Africa's Talking | ` + "`" + `africastalking` + "`" + ` | ` + "`" + `AT_API_KEY` + "`" + `, ` + "`" + `AT_USERNAME` + "`" + `, ` + "`" + `AT_WHATSAPP_FROM` + "`" + ` |
| Twilio | ` + "`" + `twilio` + "`" + ` | ` + "`" + `TWILIO_ACCOUNT_SID` + "`" + `, ` + "`" + `TWILIO_AUTH_TOKEN` + "`" + `, ` + "`" + `TWILIO_WHATSAPP_FROM` + "`" + ` |
| Custom | ` + "`" + `custom` + "`" + ` | — |

---

## Email

**Interface:** ` + "`" + `internal/providers/email.EmailProvider` + "`" + `

| Provider | EMAIL_PROVIDER value | Required env vars |
|---|---|---|
| Resend | ` + "`" + `resend` + "`" + ` | ` + "`" + `RESEND_API_KEY` + "`" + ` |
| SMTP | ` + "`" + `smtp` + "`" + ` | ` + "`" + `SMTP_HOST` + "`" + `, ` + "`" + `SMTP_PORT` + "`" + `, ` + "`" + `SMTP_USERNAME` + "`" + `, ` + "`" + `SMTP_PASSWORD` + "`" + ` |
| Custom | ` + "`" + `custom` + "`" + ` | — |

---

## Push Notifications

**Interface:** ` + "`" + `internal/providers/push.PushProvider` + "`" + `

| Provider | PUSH_PROVIDER value | Required env vars |
|---|---|---|
| FCM (Firebase) | ` + "`" + `fcm` + "`" + ` | ` + "`" + `FCM_PROJECT_ID` + "`" + `, ` + "`" + `FCM_ACCESS_TOKEN` + "`" + ` or ` + "`" + `FCM_SERVER_KEY` + "`" + ` |
| OneSignal | ` + "`" + `onesignal` + "`" + ` | ` + "`" + `ONESIGNAL_APP_ID` + "`" + `, ` + "`" + `ONESIGNAL_API_KEY` + "`" + ` |
| Custom | ` + "`" + `custom` + "`" + ` | — |

---

## USSD

**Interface:** ` + "`" + `internal/providers/ussd.Provider` + "`" + `

| Provider | USSD_PROVIDER value | Notes |
|---|---|---|
| Africa's Talking | ` + "`" + `africastalking` + "`" + ` | Most widely used in East Africa |
| Yo! Uganda | ` + "`" + `yo` + "`" + ` | Uganda-specific |
| Custom | ` + "`" + `custom` + "`" + ` | — |

Use ` + "`" + `ussd.NewMenu()` + "`" + ` to build menu-driven USSD flows.
Use ` + "`" + `ussd.NewInMemorySessionStore()` + "`" + ` for dev, Redis for production.

---

## Adding a new provider

1. Create ` + "`" + `internal/providers/<category>/<name>.go` + "`" + `
2. Implement the ` + "`" + `<Category>Provider` + "`" + ` interface
3. Add a case to ` + "`" + `internal/providers/<category>/registry.go` + "`" + `
4. Add env vars to ` + "`" + `.env.example` + "`" + `
`
}
