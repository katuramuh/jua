package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"
)

func writeJuaNotificationsFiles(root string, opts Options) error {
	apiRoot := opts.APIRoot(root)
	module := opts.Module()

	files := map[string]string{
		// Phase 7: Types + Models + Preferences
		filepath.Join(apiRoot, "jua", "notifications", "types.go"):       notifTypesGo(),
		filepath.Join(apiRoot, "jua", "notifications", "model.go"):       notifModelGo(),
		filepath.Join(apiRoot, "jua", "notifications", "migration.go"):   notifMigrationGo(),
		filepath.Join(apiRoot, "jua", "notifications", "preferences.go"): notifPreferencesGo(),

		// Phase 8: Engine + Worker
		filepath.Join(apiRoot, "jua", "notifications", "engine.go"): notifEngineGo(),
		filepath.Join(apiRoot, "jua", "notifications", "worker.go"): notifWorkerGo(),

		// Phase 9: Templates
		filepath.Join(apiRoot, "jua", "notifications", "templates", "base.go"):    notifTemplatesBaseGo(),
		filepath.Join(apiRoot, "jua", "notifications", "templates", "builtin.go"): notifTemplatesBuiltinGo(),
		filepath.Join(apiRoot, "jua", "notifications", "template_registry.go"):    notifTemplateRegistryGo(),

		// Phase 10: Handlers + Routes
		filepath.Join(apiRoot, "jua", "notifications", "handlers.go"): notifHandlersGo(),
		filepath.Join(apiRoot, "jua", "notifications", "routes.go"):   notifRoutesGo(),

		// Phase 11: In-app delivery
		filepath.Join(apiRoot, "jua", "notifications", "inapp.go"): notifInAppGo(),
	}

	for path, content := range files {
		content = strings.ReplaceAll(content, "{{MODULE}}", module)
		if err := writeFile(path, content); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}

	// Frontend SDK (skip for API-only)
	if opts.ShouldIncludeWeb() {
		webRoot := filepath.Join(root, "apps", "web")
		webFiles := map[string]string{
			filepath.Join(webRoot, "jua", "notifications", "use-notifications.ts"):          notifUseNotificationsTS(),
			filepath.Join(webRoot, "jua", "notifications", "notification-bell.tsx"):         notifNotificationBellTSX(),
			filepath.Join(webRoot, "jua", "notifications", "notification-preferences.tsx"):  notifPreferencesTSX(),
		}
		for path, content := range webFiles {
			if err := writeFile(path, content); err != nil {
				return fmt.Errorf("writing %s: %w", path, err)
			}
		}
	}

	return nil
}

// ─── Phase 7: Types ───────────────────────────────────────────────────────

func notifTypesGo() string {
	return `package notifications

import (
	"time"

	"github.com/google/uuid"
)

// NotificationType determines delivery priority and channel rules.
type NotificationType string

const (
	// Transactional — payment, OTP, receipts. Cannot be opted out. Bypasses quiet hours.
	Transactional NotificationType = "transactional"
	// Operational — system alerts, status updates. Respects user preferences.
	Operational NotificationType = "operational"
	// Marketing — promotions, newsletters. Only sent to opted-in channels.
	Marketing NotificationType = "marketing"
	// Security — login alerts, suspicious activity. All channels, bypasses quiet hours.
	Security NotificationType = "security"
)

// NotificationChannel is a delivery channel.
type NotificationChannel string

const (
	ChannelSMS      NotificationChannel = "sms"
	ChannelEmail    NotificationChannel = "email"
	ChannelPush     NotificationChannel = "push"
	ChannelWhatsApp NotificationChannel = "whatsapp"
	ChannelInApp    NotificationChannel = "inapp"
)

// Notification is the request passed to Engine.Send().
type Notification struct {
	TenantID uuid.UUID
	UserID   uuid.UUID
	Type     NotificationType
	Title    string
	Body     string
	Data     map[string]interface{} // arbitrary metadata (e.g. reference, URL)
	Channels []NotificationChannel  // override channels; nil = use engine rules
}

// NotificationLog records a single delivery attempt.
type NotificationLog struct {
	ID             uuid.UUID
	NotificationID uuid.UUID
	Channel        NotificationChannel
	Status         string // sent, failed, delivered
	Error          string
	ProviderRef    string // provider's message ID
	SentAt         time.Time
	DeliveredAt    *time.Time
}
`
}

// ─── Phase 7: Model ───────────────────────────────────────────────────────

func notifModelGo() string {
	return `package notifications

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// NotificationRecord is the persisted notification in the database.
type NotificationRecord struct {
	ID        uuid.UUID      ` + "`" + `gorm:"type:uuid;primarykey"` + "`" + `
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt ` + "`" + `gorm:"index"` + "`" + `

	TenantID uuid.UUID ` + "`" + `gorm:"type:uuid;index"` + "`" + `
	UserID   uuid.UUID ` + "`" + `gorm:"type:uuid;index"` + "`" + `

	Type  string ` + "`" + `gorm:"not null"` + "`" + `
	Title string ` + "`" + `gorm:"not null"` + "`" + `
	Body  string ` + "`" + `gorm:"not null"` + "`" + `
	Data  datatypes.JSON

	Status string     ` + "`" + `gorm:"default:pending"` + "`" + ` // pending, sent, failed, read
	ReadAt *time.Time
	SentAt *time.Time

	Logs []NotificationLogRecord ` + "`" + `gorm:"foreignKey:NotificationID"` + "`" + `
}

func (n *NotificationRecord) BeforeCreate(_ *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

func (NotificationRecord) TableName() string { return "jua_notifications" }

// NotificationLogRecord records a single channel delivery attempt.
type NotificationLogRecord struct {
	ID             uuid.UUID  ` + "`" + `gorm:"type:uuid;primarykey"` + "`" + `
	CreatedAt      time.Time
	NotificationID uuid.UUID  ` + "`" + `gorm:"type:uuid;index"` + "`" + `
	Channel        string     ` + "`" + `gorm:"not null"` + "`" + `
	Status         string     ` + "`" + `gorm:"not null"` + "`" + ` // sent, failed, delivered
	Error          string
	ProviderRef    string
	SentAt         time.Time
	DeliveredAt    *time.Time
}

func (l *NotificationLogRecord) BeforeCreate(_ *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}

func (NotificationLogRecord) TableName() string { return "jua_notification_logs" }

// PushToken stores a device push token for a user.
type PushToken struct {
	ID        uuid.UUID      ` + "`" + `gorm:"type:uuid;primarykey"` + "`" + `
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt ` + "`" + `gorm:"index"` + "`" + `

	UserID   uuid.UUID ` + "`" + `gorm:"type:uuid;index"` + "`" + `
	TenantID uuid.UUID ` + "`" + `gorm:"type:uuid;index"` + "`" + `
	Token    string    ` + "`" + `gorm:"not null;uniqueIndex"` + "`" + `
	Platform string    ` + "`" + `gorm:"not null"` + "`" + ` // android, ios, web
	Active   bool      ` + "`" + `gorm:"default:true"` + "`" + `
}

func (t *PushToken) BeforeCreate(_ *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

func (PushToken) TableName() string { return "jua_push_tokens" }
`
}

func notifMigrationGo() string {
	return `package notifications

import "gorm.io/gorm"

// Migrate creates all notification tables.
// Call this from your database migration setup.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&NotificationRecord{},
		&NotificationLogRecord{},
		&PushToken{},
		&NotificationPreferences{},
	)
}
`
}

// ─── Phase 7: Preferences ─────────────────────────────────────────────────

func notifPreferencesGo() string {
	return `package notifications

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// NotificationPreferences stores per-user channel and quiet-hours settings.
type NotificationPreferences struct {
	ID        uuid.UUID      ` + "`" + `gorm:"type:uuid;primarykey"` + "`" + `
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt ` + "`" + `gorm:"index"` + "`" + `

	UserID   uuid.UUID ` + "`" + `gorm:"type:uuid;uniqueIndex:idx_notif_prefs_user_tenant"` + "`" + `
	TenantID uuid.UUID ` + "`" + `gorm:"type:uuid;uniqueIndex:idx_notif_prefs_user_tenant"` + "`" + `

	// Channel toggles
	SMSEnabled      bool ` + "`" + `gorm:"default:true"` + "`" + `
	EmailEnabled    bool ` + "`" + `gorm:"default:true"` + "`" + `
	PushEnabled     bool ` + "`" + `gorm:"default:true"` + "`" + `
	WhatsAppEnabled bool ` + "`" + `gorm:"default:false"` + "`" + `
	InAppEnabled    bool ` + "`" + `gorm:"default:true"` + "`" + `

	// Quiet hours — no non-urgent notifications during this window
	QuietHoursEnabled bool   ` + "`" + `gorm:"default:false"` + "`" + `
	QuietHoursStart   string ` + "`" + `gorm:"default:'22:00'"` + "`" + `
	QuietHoursEnd     string ` + "`" + `gorm:"default:'07:00'"` + "`" + `
	Timezone          string ` + "`" + `gorm:"default:'Africa/Kampala'"` + "`" + `

	// Per-type channel overrides (JSON arrays of NotificationChannel)
	TransactionalChannels datatypes.JSON
	OperationalChannels   datatypes.JSON
	MarketingChannels     datatypes.JSON
	SecurityChannels      datatypes.JSON
}

func (p *NotificationPreferences) BeforeCreate(_ *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

func (NotificationPreferences) TableName() string { return "jua_notification_preferences" }

// defaultPreferences returns sensible defaults for a new user.
func defaultPreferences(userID, tenantID uuid.UUID) *NotificationPreferences {
	return &NotificationPreferences{
		UserID:            userID,
		TenantID:          tenantID,
		SMSEnabled:        true,
		EmailEnabled:      true,
		PushEnabled:       true,
		WhatsAppEnabled:   false,
		InAppEnabled:      true,
		QuietHoursEnabled: false,
		QuietHoursStart:   "22:00",
		QuietHoursEnd:     "07:00",
		Timezone:          "Africa/Kampala",
	}
}
`
}

// ─── Phase 8: Engine ──────────────────────────────────────────────────────

func notifEngineGo() string {
	return `package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"` + "{{MODULE}}" + `/jua/events"
	"` + "{{MODULE}}" + `/internal/providers/email"
	"` + "{{MODULE}}" + `/internal/providers/push"
	"` + "{{MODULE}}" + `/internal/providers/sms"
)

// Engine is the unified notification delivery engine.
// Wire it in main.go after all providers are initialised.
type Engine struct {
	db       *gorm.DB
	bus      events.Bus
	sms      sms.SMSProvider
	email    email.EmailProvider
	push     push.PushProvider
}

// GlobalEngine is the application-wide notification engine.
var GlobalEngine *Engine

// NewEngine creates a notification engine.
// Providers may be nil — their channels are skipped if unavailable.
func NewEngine(db *gorm.DB, bus events.Bus, smsP sms.SMSProvider, emailP email.EmailProvider, pushP push.PushProvider) *Engine {
	e := &Engine{db: db, bus: bus, sms: smsP, email: emailP, push: pushP}
	GlobalEngine = e
	return e
}

// Send delivers a notification through all applicable channels.
// In-app delivery is always immediate; other channels are async goroutines.
func (e *Engine) Send(ctx context.Context, n *Notification) error {
	// 1. Persist the notification
	record, err := e.save(n)
	if err != nil {
		return fmt.Errorf("saving notification: %w", err)
	}

	// 2. Load user preferences (or defaults)
	prefs := e.loadPreferences(n.UserID, n.TenantID)

	// 3. Resolve delivery channels
	channels := e.resolveChannels(n, prefs)

	// 4. Check quiet hours (transactional and security bypass)
	if e.isQuietHours(prefs) && !bypassesQuietHours(n.Type) {
		e.scheduleAfterQuietHours(record, channels)
		return nil
	}

	// 5. Deliver in-app immediately (real-time via event bus)
	for _, ch := range channels {
		if ch == ChannelInApp {
			e.deliverInApp(ctx, n, record)
		}
	}

	// 6. Deliver other channels asynchronously
	for _, ch := range channels {
		if ch == ChannelInApp {
			continue
		}
		ch := ch
		go func() {
			if err := e.deliverChannel(context.Background(), record, ch); err != nil {
				log.Printf("[jua/notifications] delivery error (%s): %v", ch, err)
			}
		}()
	}

	return nil
}

// resolveChannels determines which channels to use based on type and preferences.
func (e *Engine) resolveChannels(n *Notification, prefs *NotificationPreferences) []NotificationChannel {
	// Explicit override from caller
	if len(n.Channels) > 0 {
		return n.Channels
	}

	switch n.Type {
	case Transactional:
		// Always SMS + InApp; Push if user has a token
		ch := []NotificationChannel{ChannelSMS, ChannelInApp}
		if prefs.PushEnabled && e.hasPushToken(n.UserID) {
			ch = append(ch, ChannelPush)
		}
		return ch

	case Security:
		// Maximum urgency — all channels simultaneously
		return []NotificationChannel{ChannelSMS, ChannelEmail, ChannelPush, ChannelInApp}

	case Operational:
		return e.channelsFromPrefs(prefs, prefs.OperationalChannels)

	case Marketing:
		return e.optedInChannels(prefs)
	}

	return []NotificationChannel{ChannelInApp}
}

func (e *Engine) channelsFromPrefs(prefs *NotificationPreferences, override []byte) []NotificationChannel {
	// Try per-type override first
	if len(override) > 0 {
		var channels []NotificationChannel
		if err := json.Unmarshal(override, &channels); err == nil && len(channels) > 0 {
			return channels
		}
	}
	return e.optedInChannels(prefs)
}

func (e *Engine) optedInChannels(prefs *NotificationPreferences) []NotificationChannel {
	var ch []NotificationChannel
	if prefs.InAppEnabled {
		ch = append(ch, ChannelInApp)
	}
	if prefs.SMSEnabled {
		ch = append(ch, ChannelSMS)
	}
	if prefs.EmailEnabled {
		ch = append(ch, ChannelEmail)
	}
	if prefs.PushEnabled && e.hasPushToken(uuid.Nil) {
		ch = append(ch, ChannelPush)
	}
	if prefs.WhatsAppEnabled {
		ch = append(ch, ChannelWhatsApp)
	}
	if len(ch) == 0 {
		ch = []NotificationChannel{ChannelInApp}
	}
	return ch
}

func (e *Engine) isQuietHours(prefs *NotificationPreferences) bool {
	if !prefs.QuietHoursEnabled {
		return false
	}
	loc, err := time.LoadLocation(prefs.Timezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	current := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	start := prefs.QuietHoursStart
	end := prefs.QuietHoursEnd

	if start <= end {
		return current >= start && current < end
	}
	// Crosses midnight (e.g. 22:00 – 07:00)
	return current >= start || current < end
}

func bypassesQuietHours(t NotificationType) bool {
	return t == Transactional || t == Security
}

func (e *Engine) scheduleAfterQuietHours(record *NotificationRecord, channels []NotificationChannel) {
	// Store as pending — a cron job should retry pending notifications after quiet hours end
	data, _ := json.Marshal(channels)
	e.db.Model(record).Updates(map[string]interface{}{
		"status": "quiet_hours",
		"data":   string(data),
	})
}

func (e *Engine) save(n *Notification) (*NotificationRecord, error) {
	data, _ := json.Marshal(n.Data)
	record := &NotificationRecord{
		TenantID: n.TenantID,
		UserID:   n.UserID,
		Type:     string(n.Type),
		Title:    n.Title,
		Body:     n.Body,
		Data:     data,
		Status:   "pending",
	}
	if err := e.db.Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

func (e *Engine) loadPreferences(userID, tenantID uuid.UUID) *NotificationPreferences {
	var prefs NotificationPreferences
	err := e.db.Where("user_id = ? AND tenant_id = ?", userID, tenantID).First(&prefs).Error
	if err != nil {
		return defaultPreferences(userID, tenantID)
	}
	return &prefs
}

func (e *Engine) hasPushToken(userID uuid.UUID) bool {
	var count int64
	e.db.Model(&PushToken{}).Where("user_id = ? AND active = true", userID).Count(&count)
	return count > 0
}

func (e *Engine) deliverChannel(ctx context.Context, record *NotificationRecord, ch NotificationChannel) error {
	var deliveryErr error

	switch ch {
	case ChannelSMS:
		if e.sms != nil {
			// Load user phone from DB (assumes users table has a phone column)
			var phone string
			e.db.Raw("SELECT phone FROM users WHERE id = ?", record.UserID).Scan(&phone)
			if phone != "" {
				deliveryErr = e.sms.Send(phone, record.Body)
			}
		}
	case ChannelEmail:
		if e.email != nil {
			var emailAddr string
			e.db.Raw("SELECT email FROM users WHERE id = ?", record.UserID).Scan(&emailAddr)
			if emailAddr != "" {
				_, deliveryErr = e.email.Send(ctx, email.EmailMessage{
					To:      []string{emailAddr},
					Subject: record.Title,
					HTML:    "<p>" + record.Body + "</p>",
					Text:    record.Body,
				})
			}
		}
	case ChannelPush:
		if e.push != nil {
			var tokens []PushToken
			e.db.Where("user_id = ? AND active = true", record.UserID).Find(&tokens)
			for _, t := range tokens {
				_, _ = e.push.Send(ctx, push.PushNotification{
					Token: t.Token,
					Title: record.Title,
					Body:  record.Body,
				})
			}
		}
	}

	// Log the delivery result
	status := "sent"
	errMsg := ""
	if deliveryErr != nil {
		status = "failed"
		errMsg = deliveryErr.Error()
	}

	now := time.Now()
	logRecord := &NotificationLogRecord{
		NotificationID: record.ID,
		Channel:        string(ch),
		Status:         status,
		Error:          errMsg,
		SentAt:         now,
	}
	e.db.Create(logRecord)

	if deliveryErr == nil {
		e.db.Model(record).Updates(map[string]interface{}{"status": "sent", "sent_at": &now})
	}

	return deliveryErr
}
`
}

// ─── Phase 8: Worker ──────────────────────────────────────────────────────

func notifWorkerGo() string {
	return `package notifications

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"
)

// RetryPending re-attempts failed notifications that are due for retry.
// Schedule this to run every 5 minutes via your cron scheduler.
//
//	// In your cron setup:
//	cron.Every(5*time.Minute, func() { notifications.RetryPending(db) })
func RetryPending(db *gorm.DB) {
	engine := GlobalEngine
	if engine == nil {
		return
	}

	var records []NotificationRecord
	db.Where("status IN (?) AND updated_at < ?",
		[]string{"failed", "pending"},
		time.Now().Add(-5*time.Minute),
	).Preload("Logs").Limit(100).Find(&records)

	for _, record := range records {
		if len(record.Logs) >= 3 {
			// Max 3 attempts — mark as permanently failed
			db.Model(&record).Update("status", "failed_permanent")
			continue
		}

		record := record
		go func() {
			ctx := context.Background()
			// Re-determine channels from logs
			attempted := make(map[string]bool)
			for _, l := range record.Logs {
				if l.Status == "sent" {
					attempted[l.Channel] = true
				}
			}
			// Retry channels that haven't succeeded
			for _, ch := range []NotificationChannel{ChannelSMS, ChannelEmail, ChannelPush} {
				if !attempted[string(ch)] {
					if err := engine.deliverChannel(ctx, &record, ch); err != nil {
						log.Printf("[jua/notifications] retry failed (%s %s): %v", record.ID, ch, err)
					}
				}
			}
		}()
	}
}

// ProcessQuietHours sends notifications that were held for quiet hours.
// Schedule to run once per hour.
func ProcessQuietHours(db *gorm.DB) {
	engine := GlobalEngine
	if engine == nil {
		return
	}

	var records []NotificationRecord
	db.Where("status = ?", "quiet_hours").Find(&records)

	for _, record := range records {
		prefs := engine.loadPreferences(record.UserID, record.TenantID)
		if !engine.isQuietHours(prefs) {
			record := record
			go func() {
				ctx := context.Background()
				_ = engine.deliverChannel(ctx, &record, ChannelSMS)
				_ = engine.deliverChannel(ctx, &record, ChannelPush)
			}()
		}
	}
}
`
}

// ─── Phase 9: Templates ───────────────────────────────────────────────────

func notifTemplatesBaseGo() string {
	return `package templates

import (
	"bytes"
	"text/template"
)

// Template defines how a notification looks across all delivery channels.
type Template struct {
	Name string
	Type string // transactional, operational, marketing, security

	SMS      string       // Go template string
	Email    EmailTpl
	Push     PushTpl
	WhatsApp WhatsAppTpl
	InApp    InAppTpl
}

// EmailTpl defines the email representation.
type EmailTpl struct {
	Subject  string // Go template string
	HTMLFile string // path to HTML template (optional)
	HTML     string // inline HTML template string (used if HTMLFile is empty)
}

// PushTpl defines the push notification.
type PushTpl struct {
	Title string
	Body  string
	Icon  string
	URL   string
}

// WhatsAppTpl references a pre-approved Meta template.
type WhatsAppTpl struct {
	TemplateName string
	Params       []string // template variable names
}

// InAppTpl defines the in-app notification appearance.
type InAppTpl struct {
	Title string
	Body  string
	Icon  string  // emoji or icon name
	URL   string  // click destination
	Color string  // accent hex colour
}

// Rendered holds the rendered output for each channel.
type Rendered struct {
	SMS      string
	EmailSubject string
	EmailHTML    string
	PushTitle    string
	PushBody     string
	InAppTitle   string
	InAppBody    string
	InAppIcon    string
	InAppURL     string
	InAppColor   string
}

// Render executes all template strings with the provided data.
func (t *Template) Render(data map[string]interface{}) (*Rendered, error) {
	r := &Rendered{
		InAppIcon:  t.InApp.Icon,
		InAppURL:   t.InApp.URL,
		InAppColor: t.InApp.Color,
	}

	r.SMS = renderStr(t.SMS, data)
	r.EmailSubject = renderStr(t.Email.Subject, data)
	if t.Email.HTML != "" {
		r.EmailHTML = renderStr(t.Email.HTML, data)
	}
	r.PushTitle = renderStr(t.Push.Title, data)
	r.PushBody = renderStr(t.Push.Body, data)
	r.InAppTitle = renderStr(t.InApp.Title, data)
	r.InAppBody = renderStr(t.InApp.Body, data)

	return r, nil
}

func renderStr(tmplStr string, data map[string]interface{}) string {
	if tmplStr == "" {
		return ""
	}
	tmpl, err := template.New("").Parse(tmplStr)
	if err != nil {
		return tmplStr
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return tmplStr
	}
	return buf.String()
}
`
}

func notifTemplatesBuiltinGo() string {
	return `package templates

// Built-in notification templates.
// Register these in init() or call notifications.RegisterBuiltins().

var PaymentReceived = &Template{
	Name: "payment.received",
	Type: "transactional",
	SMS:  "{{.Title}}: UGX {{.Amount}} received from {{.Sender}}. Ref: {{.Reference}}",
	Email: EmailTpl{
		Subject: "Payment of UGX {{.Amount}} Received",
		HTML:    "<h2>Payment Received</h2><p>UGX <strong>{{.Amount}}</strong> from {{.Sender}}.</p><p>Reference: {{.Reference}}</p>",
	},
	Push: PushTpl{
		Title: "Payment Received \U0001f4b0",
		Body:  "UGX {{.Amount}} from {{.Sender}}",
		URL:   "/payments/{{.Reference}}",
	},
	InApp: InAppTpl{
		Title: "Payment Received",
		Body:  "UGX {{.Amount}} from {{.Sender}}",
		Icon:  "\U0001f4b0",
		Color: "#10B981",
		URL:   "/payments/{{.Reference}}",
	},
}

var PaymentFailed = &Template{
	Name: "payment.failed",
	Type: "transactional",
	SMS:  "Payment of UGX {{.Amount}} failed. Reason: {{.Reason}}. Please try again.",
	Email: EmailTpl{
		Subject: "Payment Failed — UGX {{.Amount}}",
		HTML:    "<h2>Payment Failed</h2><p>Your payment of UGX <strong>{{.Amount}}</strong> could not be processed.</p><p>Reason: {{.Reason}}</p>",
	},
	Push: PushTpl{Title: "Payment Failed", Body: "UGX {{.Amount}} — {{.Reason}}"},
	InApp: InAppTpl{
		Title: "Payment Failed",
		Body:  "UGX {{.Amount}} — {{.Reason}}",
		Icon:  "\u274c",
		Color: "#EF4444",
		URL:   "/payments",
	},
}

var OTPVerification = &Template{
	Name: "otp.verification",
	Type: "transactional",
	SMS:  "Your {{.AppName}} verification code is {{.OTP}}. Valid for {{.ExpiryMinutes}} minutes. Do not share.",
	Email: EmailTpl{
		Subject: "Your verification code — {{.OTP}}",
		HTML:    "<h2>Verification Code</h2><p>Your code is: <strong style='font-size:2em'>{{.OTP}}</strong></p><p>Valid for {{.ExpiryMinutes}} minutes.</p>",
	},
	Push:  PushTpl{Title: "Verification Code", Body: "Your code: {{.OTP}}"},
	InApp: InAppTpl{Title: "Verification Code", Body: "{{.OTP}} (expires in {{.ExpiryMinutes}}m)", Icon: "\U0001f511"},
}

var SubscriptionActivated = &Template{
	Name: "subscription.activated",
	Type: "transactional",
	SMS:  "Your {{.PlanName}} subscription is now active. Enjoy!",
	Email: EmailTpl{
		Subject: "Subscription Activated — {{.PlanName}}",
		HTML:    "<h2>Welcome to {{.PlanName}}!</h2><p>Your subscription is now active.</p>",
	},
	Push:  PushTpl{Title: "Subscription Active \U0001f389", Body: "{{.PlanName}} plan is now active"},
	InApp: InAppTpl{Title: "Subscription Activated", Body: "{{.PlanName}} is now active", Icon: "\U0001f389", Color: "#10B981"},
}

var SubscriptionExpiring = &Template{
	Name: "subscription.expiring",
	Type: "operational",
	SMS:  "Your {{.PlanName}} subscription expires in {{.DaysLeft}} days. Renew to avoid interruption.",
	Email: EmailTpl{
		Subject: "Your subscription expires in {{.DaysLeft}} days",
		HTML:    "<h2>Subscription Expiring Soon</h2><p>Your <strong>{{.PlanName}}</strong> plan expires in {{.DaysLeft}} days.</p>",
	},
	Push:  PushTpl{Title: "Subscription Expiring", Body: "{{.PlanName}} expires in {{.DaysLeft}} days"},
	InApp: InAppTpl{Title: "Subscription Expiring", Body: "{{.PlanName}} — {{.DaysLeft}} days left", Icon: "\u23f3", Color: "#F59E0B"},
}

var SubscriptionExpired = &Template{
	Name: "subscription.expired",
	Type: "transactional",
	SMS:  "Your {{.PlanName}} subscription has expired. Renew now to restore access.",
	Email: EmailTpl{
		Subject: "Your subscription has expired",
		HTML:    "<h2>Subscription Expired</h2><p>Your <strong>{{.PlanName}}</strong> plan has expired. Renew to restore access.</p>",
	},
	Push:  PushTpl{Title: "Subscription Expired", Body: "{{.PlanName}} — renew now"},
	InApp: InAppTpl{Title: "Subscription Expired", Body: "Renew {{.PlanName}} to restore access", Icon: "\u26a0\ufe0f", Color: "#EF4444"},
}

var LoginNewDevice = &Template{
	Name: "login.new_device",
	Type: "security",
	SMS:  "New login to your account from {{.Device}} in {{.Location}}. Not you? Secure your account immediately.",
	Email: EmailTpl{
		Subject: "New login detected — {{.Device}}",
		HTML:    "<h2>New Login Detected</h2><p>We noticed a new login from <strong>{{.Device}}</strong> in {{.Location}}.</p><p>If this wasn't you, please secure your account immediately.</p>",
	},
	Push:  PushTpl{Title: "New Login Detected \U0001f6a8", Body: "From {{.Device}} in {{.Location}}"},
	InApp: InAppTpl{Title: "New Login Detected", Body: "{{.Device}} — {{.Location}}", Icon: "\U0001f6a8", Color: "#EF4444", URL: "/settings/security"},
}

var Welcome = &Template{
	Name: "user.welcome",
	Type: "transactional",
	SMS:  "Welcome to {{.AppName}}! Your account is ready. Start at: {{.AppURL}}",
	Email: EmailTpl{
		Subject: "Welcome to {{.AppName}} \U0001f44b",
		HTML:    "<h2>Welcome to {{.AppName}}!</h2><p>Your account has been created. <a href='{{.AppURL}}'>Get started here</a>.</p>",
	},
	Push:  PushTpl{Title: "Welcome to {{.AppName}}! \U0001f44b", Body: "Your account is ready"},
	InApp: InAppTpl{Title: "Welcome!", Body: "Your account is all set", Icon: "\U0001f44b", Color: "#6C5CE7"},
}

var PasswordReset = &Template{
	Name: "password.reset",
	Type: "transactional",
	SMS:  "Your {{.AppName}} password reset code is {{.Code}}. Expires in {{.ExpiryMinutes}} minutes.",
	Email: EmailTpl{
		Subject: "Reset your password",
		HTML:    "<h2>Password Reset</h2><p>Use this code to reset your password: <strong style='font-size:1.5em'>{{.Code}}</strong></p><p>Expires in {{.ExpiryMinutes}} minutes.</p>",
	},
	Push:  PushTpl{Title: "Password Reset Requested", Body: "Code: {{.Code}}"},
	InApp: InAppTpl{Title: "Password Reset", Body: "Check your email/SMS for the reset code", Icon: "\U0001f512"},
}
`
}

func notifTemplateRegistryGo() string {
	return `package notifications

import (
	"fmt"
	"sync"

	"` + "{{MODULE}}" + `/jua/notifications/templates"
)

var (
	registryMu sync.RWMutex
	registry   = map[string]*templates.Template{}
)

// RegisterTemplate adds a notification template to the global registry.
func RegisterTemplate(t *templates.Template) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[t.Name] = t
}

// GetTemplate retrieves a template by name.
func GetTemplate(name string) (*templates.Template, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	t, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("notification template %q not found", name)
	}
	return t, nil
}

// RenderTemplate renders a template by name with the given data.
func RenderTemplate(name string, data map[string]interface{}) (*templates.Rendered, error) {
	t, err := GetTemplate(name)
	if err != nil {
		return nil, err
	}
	return t.Render(data)
}

// RegisterBuiltins registers all built-in Jua notification templates.
// Call this once on startup.
func RegisterBuiltins() {
	for _, t := range []*templates.Template{
		templates.PaymentReceived,
		templates.PaymentFailed,
		templates.OTPVerification,
		templates.SubscriptionActivated,
		templates.SubscriptionExpiring,
		templates.SubscriptionExpired,
		templates.LoginNewDevice,
		templates.Welcome,
		templates.PasswordReset,
	} {
		RegisterTemplate(t)
	}
}
`
}

// ─── Phase 10: Handlers ───────────────────────────────────────────────────

func notifHandlersGo() string {
	return `package notifications

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Handler handles all notification HTTP endpoints.
type Handler struct {
	db     *gorm.DB
	engine *Engine
}

// NewHandler creates a notification handler.
func NewHandler(db *gorm.DB, engine *Engine) *Handler {
	return &Handler{db: db, engine: engine}
}

// ── User endpoints ────────────────────────────────────────────────────────

// List returns paginated notifications for the current user.
// GET /api/notifications
func (h *Handler) List(c *gin.Context) {
	userID := mustParseUUID(c.GetString("userID"))
	tenantID := mustParseUUID(c.GetString("tenantID"))

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 { page = 1 }
	if limit > 100 { limit = 100 }
	offset := (page - 1) * limit

	query := h.db.Where("user_id = ? AND tenant_id = ?", userID, tenantID)
	if status := c.Query("status"); status == "unread" {
		query = query.Where("read_at IS NULL")
	} else if status == "read" {
		query = query.Where("read_at IS NOT NULL")
	}
	if typ := c.Query("type"); typ != "" {
		query = query.Where("type = ?", typ)
	}

	var total int64
	query.Model(&NotificationRecord{}).Count(&total)

	var records []NotificationRecord
	query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&records)

	c.JSON(http.StatusOK, gin.H{
		"data": records,
		"meta": gin.H{
			"total":     total,
			"page":      page,
			"page_size": limit,
			"pages":     (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// UnreadCount returns the unread notification count.
// GET /api/notifications/unread-count
func (h *Handler) UnreadCount(c *gin.Context) {
	userID := mustParseUUID(c.GetString("userID"))
	tenantID := mustParseUUID(c.GetString("tenantID"))

	var count int64
	h.db.Model(&NotificationRecord{}).
		Where("user_id = ? AND tenant_id = ? AND read_at IS NULL", userID, tenantID).
		Count(&count)

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"count": count}})
}

// MarkRead marks a single notification as read.
// PUT /api/notifications/:id/read
func (h *Handler) MarkRead(c *gin.Context) {
	userID := mustParseUUID(c.GetString("userID"))
	id := mustParseUUID(c.Param("id"))

	result := h.db.Model(&NotificationRecord{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("read_at", gorm.Expr("NOW()"))

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "notification not found"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}, "message": "marked as read"})
}

// MarkAllRead marks all notifications as read for the current user.
// PUT /api/notifications/read-all
func (h *Handler) MarkAllRead(c *gin.Context) {
	userID := mustParseUUID(c.GetString("userID"))
	tenantID := mustParseUUID(c.GetString("tenantID"))

	h.db.Model(&NotificationRecord{}).
		Where("user_id = ? AND tenant_id = ? AND read_at IS NULL", userID, tenantID).
		Update("read_at", gorm.Expr("NOW()"))

	c.JSON(http.StatusOK, gin.H{"message": "all notifications marked as read"})
}

// Delete soft-deletes a notification.
// DELETE /api/notifications/:id
func (h *Handler) Delete(c *gin.Context) {
	userID := mustParseUUID(c.GetString("userID"))
	id := mustParseUUID(c.Param("id"))

	result := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&NotificationRecord{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "notification not found"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "notification deleted"})
}

// GetPreferences returns the user's notification preferences.
// GET /api/notifications/preferences
func (h *Handler) GetPreferences(c *gin.Context) {
	userID := mustParseUUID(c.GetString("userID"))
	tenantID := mustParseUUID(c.GetString("tenantID"))

	prefs := h.engine.loadPreferences(userID, tenantID)
	c.JSON(http.StatusOK, gin.H{"data": prefs})
}

// UpdatePreferences saves notification preferences.
// PUT /api/notifications/preferences
func (h *Handler) UpdatePreferences(c *gin.Context) {
	userID := mustParseUUID(c.GetString("userID"))
	tenantID := mustParseUUID(c.GetString("tenantID"))

	var input NotificationPreferences
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	var existing NotificationPreferences
	err := h.db.Where("user_id = ? AND tenant_id = ?", userID, tenantID).First(&existing).Error
	if err != nil {
		// Create new preferences
		input.UserID = userID
		input.TenantID = tenantID
		h.db.Create(&input)
	} else {
		input.ID = existing.ID
		h.db.Save(&input)
	}
	c.JSON(http.StatusOK, gin.H{"data": input, "message": "preferences saved"})
}

// RegisterPushToken saves a device push token.
// POST /api/notifications/push-token
func (h *Handler) RegisterPushToken(c *gin.Context) {
	userID := mustParseUUID(c.GetString("userID"))
	tenantID := mustParseUUID(c.GetString("tenantID"))

	var input struct {
		Token    string ` + "`" + `json:"token" binding:"required"` + "`" + `
		Platform string ` + "`" + `json:"platform" binding:"required"` + "`" + ` // android, ios, web
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	token := &PushToken{
		UserID:   userID,
		TenantID: tenantID,
		Token:    input.Token,
		Platform: input.Platform,
		Active:   true,
	}
	h.db.Where(PushToken{Token: input.Token}).Assign(token).FirstOrCreate(token)
	c.JSON(http.StatusOK, gin.H{"message": "push token registered"})
}

// ── Admin endpoints ───────────────────────────────────────────────────────

// AdminStats returns delivery statistics.
// GET /api/admin/notifications/stats
func (h *Handler) AdminStats(c *gin.Context) {
	type channelStat struct {
		Channel string ` + "`" + `json:"channel"` + "`" + `
		Sent    int64  ` + "`" + `json:"sent"` + "`" + `
		Failed  int64  ` + "`" + `json:"failed"` + "`" + `
	}
	var stats []channelStat
	h.db.Model(&NotificationLogRecord{}).
		Select("channel, COUNT(CASE WHEN status='sent' THEN 1 END) as sent, COUNT(CASE WHEN status='failed' THEN 1 END) as failed").
		Group("channel").
		Scan(&stats)

	var totalToday int64
	h.db.Model(&NotificationRecord{}).
		Where("created_at >= CURRENT_DATE").
		Count(&totalToday)

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"total_today":    totalToday,
		"by_channel":     stats,
	}})
}

// AdminLogs returns the full notification log.
// GET /api/admin/notifications/logs
func (h *Handler) AdminLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 { page = 1 }
	if limit > 200 { limit = 200 }

	var total int64
	var logs []NotificationLogRecord
	h.db.Model(&NotificationLogRecord{}).Count(&total)
	h.db.Order("created_at DESC").Limit(limit).Offset((page-1)*limit).Find(&logs)

	c.JSON(http.StatusOK, gin.H{
		"data": logs,
		"meta": gin.H{"total": total, "page": page, "page_size": limit},
	})
}

// AdminBroadcast sends a notification to all users of a tenant.
// POST /api/admin/notifications/broadcast
func (h *Handler) AdminBroadcast(c *gin.Context) {
	var input struct {
		TenantID string   ` + "`" + `json:"tenant_id" binding:"required"` + "`" + `
		Title    string   ` + "`" + `json:"title" binding:"required"` + "`" + `
		Body     string   ` + "`" + `json:"body" binding:"required"` + "`" + `
		Type     string   ` + "`" + `json:"type"` + "`" + `
		Channels []string ` + "`" + `json:"channels"` + "`" + `
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	tenantID := mustParseUUID(input.TenantID)
	notifType := NotificationType(input.Type)
	if notifType == "" {
		notifType = Operational
	}

	// Load all users for tenant and send async
	type userRow struct{ ID uuid.UUID }
	var users []userRow
	h.db.Raw("SELECT id FROM users WHERE tenant_id = ?", tenantID).Scan(&users)

	var channels []NotificationChannel
	for _, ch := range input.Channels {
		channels = append(channels, NotificationChannel(ch))
	}

	sent := 0
	for _, u := range users {
		go func(userID uuid.UUID) {
			h.engine.Send(c.Request.Context(), &Notification{ //nolint:errcheck
				TenantID: tenantID,
				UserID:   userID,
				Type:     notifType,
				Title:    input.Title,
				Body:     input.Body,
				Channels: channels,
			})
		}(u.ID)
		sent++
	}

	c.JSON(http.StatusOK, gin.H{"message": "broadcast queued", "data": gin.H{"recipients": sent}})
}

// TestNotification sends a test notification to the current user.
// POST /api/notifications/test
func (h *Handler) TestNotification(c *gin.Context) {
	userID := mustParseUUID(c.GetString("userID"))
	tenantID := mustParseUUID(c.GetString("tenantID"))

	err := h.engine.Send(c.Request.Context(), &Notification{
		TenantID: tenantID,
		UserID:   userID,
		Type:     Operational,
		Title:    "Test Notification",
		Body:     "This is a test notification from Jua.",
		Data:     map[string]interface{}{"test": true},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "test notification sent"})
}

// mustParseUUID parses a UUID string, returning uuid.Nil on failure.
func mustParseUUID(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}
`
}

// ─── Phase 10: Routes ─────────────────────────────────────────────────────

func notifRoutesGo() string {
	return `package notifications

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"` + "{{MODULE}}" + `/internal/middleware"
	"` + "{{MODULE}}" + `/internal/services"
)

// RegisterRoutes mounts all notification routes onto the Gin engine.
//
//	// Jua notifications routes
//	notifications.RegisterRoutes(router, db, authService)
func RegisterRoutes(router *gin.Engine, db *gorm.DB, authService *services.AuthService) {
	engine := GlobalEngine
	if engine == nil {
		engine = NewEngine(db, nil, nil, nil, nil)
	}
	h := NewHandler(db, engine)

	// Protected user routes
	protected := router.Group("/api")
	protected.Use(middleware.Auth(db, authService))
	{
		protected.GET("/notifications", h.List)
		protected.GET("/notifications/unread-count", h.UnreadCount)
		protected.PUT("/notifications/read-all", h.MarkAllRead)
		protected.PUT("/notifications/:id/read", h.MarkRead)
		protected.DELETE("/notifications/:id", h.Delete)
		protected.GET("/notifications/preferences", h.GetPreferences)
		protected.PUT("/notifications/preferences", h.UpdatePreferences)
		protected.POST("/notifications/push-token", h.RegisterPushToken)
		protected.POST("/notifications/test", h.TestNotification)
	}

	// Admin routes
	admin := router.Group("/api")
	admin.Use(middleware.Auth(db, authService))
	admin.Use(middleware.RequireRole("ADMIN"))
	{
		admin.GET("/admin/notifications/stats", h.AdminStats)
		admin.GET("/admin/notifications/logs", h.AdminLogs)
		admin.POST("/admin/notifications/broadcast", h.AdminBroadcast)
	}
}
`
}

// ─── Phase 11: In-app delivery ────────────────────────────────────────────

func notifInAppGo() string {
	return `package notifications

import (
	"context"

	"` + "{{MODULE}}" + `/jua/events"
)

// deliverInApp sends a notification instantly via the real-time event bus.
// The frontend useNotifications() hook receives it and updates the UI.
func (e *Engine) deliverInApp(_ context.Context, n *Notification, record *NotificationRecord) {
	payload := map[string]interface{}{
		"id":        record.ID,
		"title":     n.Title,
		"body":      n.Body,
		"type":      string(n.Type),
		"data":      n.Data,
		"createdAt": record.CreatedAt,
	}

	events.PublishForUser(
		"notification.new",
		n.TenantID.String(),
		n.UserID.String(),
		payload,
	)
}
`
}

// ─── Phase 11: Frontend hooks + components ────────────────────────────────

func notifUseNotificationsTS() string {
	return `import { useState, useEffect, useCallback } from 'react'
import { useRealtime } from '../realtime/realtime-provider'

export interface AppNotification {
  id: string
  title: string
  body: string
  type: 'transactional' | 'operational' | 'marketing' | 'security'
  data?: Record<string, unknown>
  readAt?: string
  createdAt: string
}

async function fetchNotifications(): Promise<AppNotification[]> {
  const token = localStorage.getItem('jua_token') || ''
  const res = await fetch('/api/notifications?limit=50', {
    headers: { Authorization: ` + "`" + `Bearer ${token}` + "`" + ` },
  })
  if (!res.ok) return []
  const json = await res.json()
  return json.data ?? []
}

async function fetchUnreadCount(): Promise<number> {
  const token = localStorage.getItem('jua_token') || ''
  const res = await fetch('/api/notifications/unread-count', {
    headers: { Authorization: ` + "`" + `Bearer ${token}` + "`" + ` },
  })
  if (!res.ok) return 0
  const json = await res.json()
  return json.data?.count ?? 0
}

async function markNotificationRead(id: string): Promise<void> {
  const token = localStorage.getItem('jua_token') || ''
  await fetch(` + "`" + `/api/notifications/${id}/read` + "`" + `, {
    method: 'PUT',
    headers: { Authorization: ` + "`" + `Bearer ${token}` + "`" + ` },
  })
}

async function markAllNotificationsRead(): Promise<void> {
  const token = localStorage.getItem('jua_token') || ''
  await fetch('/api/notifications/read-all', {
    method: 'PUT',
    headers: { Authorization: ` + "`" + `Bearer ${token}` + "`" + ` },
  })
}

/**
 * useNotifications manages the notification list and unread count.
 * New notifications arrive in real time via RealtimeProvider.
 */
export function useNotifications() {
  const [notifications, setNotifications] = useState<AppNotification[]>([])
  const [unreadCount, setUnreadCount] = useState(0)
  const [loading, setLoading] = useState(true)
  const realtime = useRealtime()

  // Load initial state
  useEffect(() => {
    Promise.all([fetchNotifications(), fetchUnreadCount()]).then(
      ([notifs, count]) => {
        setNotifications(notifs)
        setUnreadCount(count)
        setLoading(false)
      }
    )
  }, [])

  // Live updates via real-time
  useEffect(() => {
    return realtime.on('notification.new', (data: unknown) => {
      const notif = data as AppNotification
      setNotifications((prev) => [notif, ...prev])
      setUnreadCount((prev) => prev + 1)
    })
  }, [realtime])

  const markRead = useCallback(async (id: string) => {
    await markNotificationRead(id)
    setNotifications((prev) =>
      prev.map((n) => (n.id === id ? { ...n, readAt: new Date().toISOString() } : n))
    )
    setUnreadCount((prev) => Math.max(0, prev - 1))
  }, [])

  const markAllRead = useCallback(async () => {
    await markAllNotificationsRead()
    setNotifications((prev) => prev.map((n) => ({ ...n, readAt: new Date().toISOString() })))
    setUnreadCount(0)
  }, [])

  return { notifications, unreadCount, loading, markRead, markAllRead }
}
`
}

func notifNotificationBellTSX() string {
	return `'use client'

import { useState, useRef, useEffect } from 'react'
import { Bell } from 'lucide-react'
import { useNotifications, type AppNotification } from './use-notifications'

/**
 * NotificationBell — drop-in bell icon with live unread count and dropdown list.
 * Place in your top navigation bar.
 *
 * Requires RealtimeProvider to be an ancestor in the component tree.
 */
export function NotificationBell() {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const { notifications, unreadCount, markRead, markAllRead } = useNotifications()

  // Close on outside click
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  return (
    <div className="relative" ref={ref}>
      {/* Bell button */}
      <button
        onClick={() => setOpen((o) => !o)}
        className="relative p-2 rounded-lg hover:bg-[var(--bg-hover)] transition-colors"
        aria-label="Notifications"
      >
        <Bell className="h-5 w-5 text-[var(--text-secondary)]" />
        {unreadCount > 0 && (
          <span className="absolute -top-0.5 -right-0.5 h-4 min-w-4 px-1 rounded-full bg-[var(--accent)] text-white text-[10px] font-semibold flex items-center justify-center">
            {unreadCount > 99 ? '99+' : unreadCount}
          </span>
        )}
      </button>

      {/* Dropdown */}
      {open && (
        <div className="absolute right-0 mt-2 w-80 bg-[var(--bg-elevated)] border border-[var(--border)] rounded-xl shadow-xl z-50 overflow-hidden">
          {/* Header */}
          <div className="flex items-center justify-between px-4 py-3 border-b border-[var(--border)]">
            <span className="text-sm font-semibold text-[var(--text-primary)]">
              Notifications
            </span>
            {unreadCount > 0 && (
              <button
                onClick={markAllRead}
                className="text-xs text-[var(--accent)] hover:underline"
              >
                Mark all as read
              </button>
            )}
          </div>

          {/* List */}
          <div className="max-h-96 overflow-y-auto divide-y divide-[var(--border)]">
            {notifications.length === 0 ? (
              <div className="py-12 text-center text-sm text-[var(--text-muted)]">
                <span className="block text-2xl mb-2">🎉</span>
                You&apos;re all caught up
              </div>
            ) : (
              notifications.slice(0, 20).map((n) => (
                <NotificationItem key={n.id} notification={n} onRead={markRead} />
              ))
            )}
          </div>

          {/* Footer */}
          {notifications.length > 0 && (
            <div className="px-4 py-2.5 border-t border-[var(--border)]">
              <a
                href="/notifications"
                className="text-xs text-[var(--accent)] hover:underline"
                onClick={() => setOpen(false)}
              >
                View all notifications
              </a>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function NotificationItem({
  notification: n,
  onRead,
}: {
  notification: AppNotification
  onRead: (id: string) => void
}) {
  const unread = !n.readAt

  return (
    <button
      onClick={() => { if (unread) onRead(n.id) }}
      className={[
        'w-full text-left px-4 py-3 hover:bg-[var(--bg-hover)] transition-colors',
        unread ? 'bg-[var(--bg-tertiary)]' : '',
      ].join(' ')}
    >
      <div className="flex items-start gap-2.5">
        {unread && (
          <span className="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-[var(--accent)]" />
        )}
        <div className={unread ? '' : 'ml-4'}>
          <p className="text-sm font-medium text-[var(--text-primary)] leading-snug">
            {n.title}
          </p>
          <p className="text-xs text-[var(--text-secondary)] mt-0.5 leading-relaxed">
            {n.body}
          </p>
          <p className="text-[10px] text-[var(--text-muted)] mt-1">
            {new Date(n.createdAt).toLocaleString()}
          </p>
        </div>
      </div>
    </button>
  )
}
`
}

func notifPreferencesTSX() string {
	return `'use client'

import { useState, useEffect } from 'react'

interface Preferences {
  sms_enabled: boolean
  email_enabled: boolean
  push_enabled: boolean
  whats_app_enabled: boolean
  in_app_enabled: boolean
  quiet_hours_enabled: boolean
  quiet_hours_start: string
  quiet_hours_end: string
  timezone: string
}

const TIMEZONES = [
  'Africa/Kampala',
  'Africa/Nairobi',
  'Africa/Lagos',
  'Africa/Accra',
  'Africa/Johannesburg',
  'Africa/Cairo',
  'UTC',
  'Europe/London',
  'America/New_York',
]

async function loadPreferences(): Promise<Preferences | null> {
  const token = localStorage.getItem('jua_token') || ''
  const res = await fetch('/api/notifications/preferences', {
    headers: { Authorization: ` + "`" + `Bearer ${token}` + "`" + ` },
  })
  if (!res.ok) return null
  return (await res.json()).data
}

async function savePreferences(prefs: Partial<Preferences>): Promise<boolean> {
  const token = localStorage.getItem('jua_token') || ''
  const res = await fetch('/api/notifications/preferences', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', Authorization: ` + "`" + `Bearer ${token}` + "`" + ` },
    body: JSON.stringify(prefs),
  })
  return res.ok
}

/**
 * NotificationPreferences — settings page for notification channel toggles,
 * quiet hours, and timezone configuration.
 */
export function NotificationPreferences() {
  const [prefs, setPrefs] = useState<Preferences>({
    sms_enabled: true,
    email_enabled: true,
    push_enabled: true,
    whats_app_enabled: false,
    in_app_enabled: true,
    quiet_hours_enabled: false,
    quiet_hours_start: '22:00',
    quiet_hours_end: '07:00',
    timezone: 'Africa/Kampala',
  })
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    loadPreferences().then((p) => { if (p) setPrefs(p) })
  }, [])

  const toggle = (key: keyof Preferences) => {
    setPrefs((prev) => ({ ...prev, [key]: !prev[key] }))
    setSaved(false)
  }

  const handleSave = async () => {
    setSaving(true)
    const ok = await savePreferences(prefs)
    setSaving(false)
    if (ok) { setSaved(true); setTimeout(() => setSaved(false), 3000) }
  }

  return (
    <div className="max-w-lg space-y-6">
      <div>
        <h2 className="text-base font-semibold text-[var(--text-primary)]">Notification Channels</h2>
        <p className="text-sm text-[var(--text-secondary)] mt-1">
          Choose how you want to receive notifications.
        </p>
      </div>

      <div className="space-y-3">
        {([
          ['in_app_enabled', 'In-App', 'Notifications inside the dashboard'],
          ['push_enabled', 'Push', 'Browser and mobile push notifications'],
          ['email_enabled', 'Email', 'Email notifications to your address'],
          ['sms_enabled', 'SMS', 'Text messages to your phone'],
          ['whats_app_enabled', 'WhatsApp', 'Messages via WhatsApp'],
        ] as const).map(([key, label, desc]) => (
          <label key={key} className="flex items-center justify-between p-3 rounded-lg border border-[var(--border)] cursor-pointer hover:bg-[var(--bg-hover)]">
            <div>
              <span className="text-sm font-medium text-[var(--text-primary)]">{label}</span>
              <p className="text-xs text-[var(--text-muted)]">{desc}</p>
            </div>
            <Toggle checked={prefs[key] as boolean} onChange={() => toggle(key)} />
          </label>
        ))}
      </div>

      <div>
        <h2 className="text-base font-semibold text-[var(--text-primary)]">Quiet Hours</h2>
        <p className="text-sm text-[var(--text-secondary)] mt-1">
          Mute non-urgent notifications during these hours.
        </p>
      </div>

      <label className="flex items-center justify-between">
        <span className="text-sm text-[var(--text-primary)]">Enable quiet hours</span>
        <Toggle checked={prefs.quiet_hours_enabled} onChange={() => toggle('quiet_hours_enabled')} />
      </label>

      {prefs.quiet_hours_enabled && (
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="text-xs text-[var(--text-secondary)]">From</label>
            <input
              type="time"
              value={prefs.quiet_hours_start}
              onChange={(e) => { setPrefs((p) => ({ ...p, quiet_hours_start: e.target.value })); setSaved(false) }}
              className="mt-1 w-full px-3 py-2 rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border)] text-sm text-[var(--text-primary)]"
            />
          </div>
          <div>
            <label className="text-xs text-[var(--text-secondary)]">Until</label>
            <input
              type="time"
              value={prefs.quiet_hours_end}
              onChange={(e) => { setPrefs((p) => ({ ...p, quiet_hours_end: e.target.value })); setSaved(false) }}
              className="mt-1 w-full px-3 py-2 rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border)] text-sm text-[var(--text-primary)]"
            />
          </div>
        </div>
      )}

      <div>
        <label className="text-xs text-[var(--text-secondary)]">Timezone</label>
        <select
          value={prefs.timezone}
          onChange={(e) => { setPrefs((p) => ({ ...p, timezone: e.target.value })); setSaved(false) }}
          className="mt-1 w-full px-3 py-2 rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border)] text-sm text-[var(--text-primary)]"
        >
          {TIMEZONES.map((tz) => <option key={tz} value={tz}>{tz}</option>)}
        </select>
      </div>

      <button
        onClick={handleSave}
        disabled={saving}
        className="px-6 py-2.5 rounded-lg bg-[var(--accent)] hover:bg-[var(--accent-hover)] text-white text-sm font-medium transition-colors disabled:opacity-60"
      >
        {saving ? 'Saving…' : saved ? 'Saved ✓' : 'Save Preferences'}
      </button>
    </div>
  )
}

function Toggle({ checked, onChange }: { checked: boolean; onChange: () => void }) {
  return (
    <button
      onClick={onChange}
      className={[
        'relative inline-flex h-5 w-9 items-center rounded-full transition-colors',
        checked ? 'bg-[var(--accent)]' : 'bg-[var(--bg-hover)]',
      ].join(' ')}
    >
      <span
        className={[
          'inline-block h-3.5 w-3.5 rounded-full bg-white transition-transform',
          checked ? 'translate-x-[18px]' : 'translate-x-[3px]',
        ].join(' ')}
      />
    </button>
  )
}
`
}
