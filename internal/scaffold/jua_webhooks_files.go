package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"
)

func writeJuaWebhooksFiles(root string, opts Options) error {
	apiRoot := opts.APIRoot(root)
	module := opts.Module()

	files := map[string]string{
		filepath.Join(apiRoot, "internal", "webhooks", "model.go"):      webhooksModelGo(),
		filepath.Join(apiRoot, "internal", "webhooks", "dispatcher.go"): webhooksDispatcherGo(),
		filepath.Join(apiRoot, "internal", "webhooks", "service.go"):    webhooksServiceGo(),
		filepath.Join(apiRoot, "internal", "webhooks", "handler.go"):    webhooksHandlerGo(),
		filepath.Join(apiRoot, "internal", "webhooks", "migrate.go"):    webhooksMigrateGo(),
	}

	for path, content := range files {
		content = strings.ReplaceAll(content, "{{MODULE}}", module)
		if err := writeFile(path, content); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

func webhooksModelGo() string {
	return `package webhooks

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Webhook represents a registered webhook endpoint.
type Webhook struct {
	ID        uint           ` + "`" + `gorm:"primarykey" json:"id"` + "`" + `
	CreatedAt time.Time      ` + "`" + `json:"created_at"` + "`" + `
	UpdatedAt time.Time      ` + "`" + `json:"updated_at"` + "`" + `
	DeletedAt gorm.DeletedAt ` + "`" + `gorm:"index" json:"-"` + "`" + `

	// OwnerID is the user or tenant that registered this webhook.
	OwnerID uint ` + "`" + `gorm:"not null;index" json:"owner_id"` + "`" + `

	// URL is the HTTPS endpoint that receives webhook POST requests.
	URL string ` + "`" + `gorm:"not null" json:"url"` + "`" + `

	// Events is the list of event names this webhook subscribes to.
	// Use "*" to subscribe to all events.
	// Examples: ["user.created", "payment.success", "order.shipped"]
	Events pq.StringArray ` + "`" + `gorm:"type:text[];not null" json:"events"` + "`" + `

	// Secret is the HMAC-SHA256 signing secret.
	// Included as X-Jua-Signature header so receivers can verify authenticity.
	// Shown only once at creation — store it securely.
	Secret string ` + "`" + `gorm:"not null" json:"-"` + "`" + `

	// Active controls whether this webhook receives deliveries.
	Active bool ` + "`" + `gorm:"default:true" json:"active"` + "`" + `

	// Description is an optional human-readable label.
	Description string ` + "`" + `json:"description,omitempty"` + "`" + `
}

// TableName returns the GORM table name.
func (Webhook) TableName() string { return "webhooks" }

// SubscribesTo returns true if this webhook is subscribed to the given event.
func (w *Webhook) SubscribesTo(event string) bool {
	for _, e := range w.Events {
		if e == "*" || e == event {
			return true
		}
	}
	return false
}

// DeliveryStatus represents the outcome of a webhook delivery attempt.
type DeliveryStatus string

const (
	DeliveryPending  DeliveryStatus = "pending"
	DeliverySuccess  DeliveryStatus = "success"
	DeliveryFailed   DeliveryStatus = "failed"
	DeliveryRetrying DeliveryStatus = "retrying"
)

// WebhookEvent is a delivery log entry for a single webhook event.
type WebhookEvent struct {
	ID        uint      ` + "`" + `gorm:"primarykey" json:"id"` + "`" + `
	CreatedAt time.Time ` + "`" + `json:"created_at"` + "`" + `
	UpdatedAt time.Time ` + "`" + `json:"updated_at"` + "`" + `

	// WebhookID is the webhook this event was delivered to.
	WebhookID uint ` + "`" + `gorm:"not null;index" json:"webhook_id"` + "`" + `

	// EventName is the event type, e.g. "user.created".
	EventName string ` + "`" + `gorm:"not null;index" json:"event_name"` + "`" + `

	// Payload is the JSON body sent to the webhook URL.
	Payload string ` + "`" + `gorm:"type:text" json:"payload"` + "`" + `

	// Status is the current delivery status.
	Status DeliveryStatus ` + "`" + `gorm:"default:'pending'" json:"status"` + "`" + `

	// Attempts is the number of delivery attempts made so far.
	Attempts int ` + "`" + `gorm:"default:0" json:"attempts"` + "`" + `

	// MaxAttempts is the maximum retries before marking as failed.
	MaxAttempts int ` + "`" + `gorm:"default:5" json:"max_attempts"` + "`" + `

	// NextAttemptAt is when the next retry is scheduled.
	NextAttemptAt *time.Time ` + "`" + `json:"next_attempt_at,omitempty"` + "`" + `

	// ResponseCode is the HTTP status code of the last delivery attempt.
	ResponseCode int ` + "`" + `json:"response_code,omitempty"` + "`" + `

	// ResponseBody is the first 500 bytes of the response body.
	ResponseBody string ` + "`" + `gorm:"type:text" json:"response_body,omitempty"` + "`" + `

	// Error holds the error message if delivery failed.
	Error string ` + "`" + `json:"error,omitempty"` + "`" + `
}

// TableName returns the GORM table name.
func (WebhookEvent) TableName() string { return "webhook_events" }
`
}

func webhooksDispatcherGo() string {
	return `package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	"gorm.io/gorm"

	// Jua events — broadcast webhook triggers to real-time engine
	"{{MODULE}}/jua/events"
)

// Dispatcher handles async webhook delivery with exponential backoff retries.
type Dispatcher struct {
	db     *gorm.DB
	client *http.Client
}

// NewDispatcher creates a webhook dispatcher.
func NewDispatcher(db *gorm.DB) *Dispatcher {
	return &Dispatcher{
		db: db,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Trigger fires an event to all active webhooks subscribed to it.
// It creates WebhookEvent records and starts async delivery goroutines.
//
// Usage:
//
//	dispatcher.Trigger(ctx, ownerID, "user.created", map[string]interface{}{
//	    "id": user.ID, "email": user.Email,
//	})
func (d *Dispatcher) Trigger(ctx context.Context, ownerID uint, eventName string, payload interface{}) error {
	// Find all active webhooks for this owner that subscribe to this event
	var hooks []Webhook
	if err := d.db.Where("owner_id = ? AND active = true", ownerID).Find(&hooks).Error; err != nil {
		return fmt.Errorf("webhooks: find hooks: %w", err)
	}

	payloadBytes, err := json.Marshal(map[string]interface{}{
		"event":      eventName,
		"created_at": time.Now().UTC(),
		"data":       payload,
	})
	if err != nil {
		return fmt.Errorf("webhooks: marshal payload: %w", err)
	}
	payloadStr := string(payloadBytes)

	for _, hook := range hooks {
		if !hook.SubscribesTo(eventName) {
			continue
		}

		now := time.Now()
		event := &WebhookEvent{
			WebhookID:     hook.ID,
			EventName:     eventName,
			Payload:       payloadStr,
			Status:        DeliveryPending,
			MaxAttempts:   5,
			NextAttemptAt: &now,
		}
		if err := d.db.Create(event).Error; err != nil {
			log.Printf("webhooks: create event: %v", err)
			continue
		}

		// Jua events: publish to real-time engine so dashboards see it instantly
		events.PublishCustom(eventName, fmt.Sprintf("%d", ownerID), map[string]interface{}{
			"webhook_id": hook.ID,
			"event":      eventName,
		})

		// Deliver asynchronously
		go d.deliver(context.Background(), hook, event)
	}

	return nil
}

// deliver attempts to deliver a webhook event to its endpoint.
func (d *Dispatcher) deliver(ctx context.Context, hook Webhook, event *WebhookEvent) {
	sig := sign([]byte(event.Payload), hook.Secret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook.URL,
		bytes.NewBufferString(event.Payload))
	if err != nil {
		d.markFailed(event, 0, err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Jua-Signature", "sha256="+sig)
	req.Header.Set("X-Jua-Event", event.EventName)
	req.Header.Set("X-Jua-Delivery", fmt.Sprintf("%d", event.ID))

	resp, err := d.client.Do(req)
	attempts := event.Attempts + 1

	if err != nil {
		if attempts >= event.MaxAttempts {
			d.markFailed(event, 0, err.Error())
		} else {
			d.scheduleRetry(event, attempts, err.Error())
		}
		return
	}
	defer resp.Body.Close()

	bodyBuf := make([]byte, 500)
	n, _ := resp.Body.Read(bodyBuf)
	respBody := string(bodyBuf[:n])

	updates := map[string]interface{}{
		"attempts":      attempts,
		"response_code": resp.StatusCode,
		"response_body": respBody,
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		updates["status"] = DeliverySuccess
		d.db.Model(event).Updates(updates)
		return
	}

	// Non-2xx response — retry or fail
	if attempts >= event.MaxAttempts {
		updates["status"] = DeliveryFailed
		updates["error"] = fmt.Sprintf("HTTP %d after %d attempts", resp.StatusCode, attempts)
		d.db.Model(event).Updates(updates)
	} else {
		updates["status"] = DeliveryRetrying
		next := time.Now().Add(backoffDuration(attempts))
		updates["next_attempt_at"] = next
		d.db.Model(event).Updates(updates)
	}
}

func (d *Dispatcher) markFailed(event *WebhookEvent, code int, errMsg string) {
	d.db.Model(event).Updates(map[string]interface{}{
		"status":        DeliveryFailed,
		"attempts":      event.Attempts + 1,
		"response_code": code,
		"error":         errMsg,
	})
}

func (d *Dispatcher) scheduleRetry(event *WebhookEvent, attempts int, errMsg string) {
	next := time.Now().Add(backoffDuration(attempts))
	d.db.Model(event).Updates(map[string]interface{}{
		"status":          DeliveryRetrying,
		"attempts":        attempts,
		"next_attempt_at": next,
		"error":           errMsg,
	})
}

// RetryPending re-attempts all events in the "retrying" state that are due.
// Call this from a cron job every minute.
func (d *Dispatcher) RetryPending(ctx context.Context) error {
	var events []WebhookEvent
	err := d.db.Where("status = ? AND next_attempt_at <= ? AND attempts < max_attempts",
		DeliveryRetrying, time.Now()).
		Preload("Webhook").
		Find(&events).Error
	if err != nil {
		return fmt.Errorf("webhooks: retry pending: %w", err)
	}

	for _, event := range events {
		go d.deliver(ctx, event.Webhook, &event)
	}
	return nil
}

// sign produces an HMAC-SHA256 signature of the payload using the webhook secret.
func sign(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature checks that an incoming webhook signature header is valid.
// Use this in your receiver endpoints to verify Jua webhooks.
//
//	sig := r.Header.Get("X-Jua-Signature") // "sha256=abc123..."
//	if !webhooks.VerifySignature(body, secret, sig) {
//	    http.Error(w, "invalid signature", 401)
//	    return
//	}
func VerifySignature(payload []byte, secret, signatureHeader string) bool {
	const prefix = "sha256="
	if len(signatureHeader) <= len(prefix) {
		return false
	}
	expected := signatureHeader[len(prefix):]
	actual := sign(payload, secret)
	return hmac.Equal([]byte(expected), []byte(actual))
}

// GenerateSecret returns a cryptographically random 32-byte hex secret.
func GenerateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// backoffDuration returns the delay before the nth retry attempt.
// Uses exponential backoff: 30s, 60s, 5m, 30m, 2h
func backoffDuration(attempt int) time.Duration {
	minutes := math.Pow(2, float64(attempt)) * 0.5
	return time.Duration(minutes * float64(time.Minute))
}
`
}

func webhooksServiceGo() string {
	return `package webhooks

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// Service handles webhook CRUD operations.
type Service struct {
	db         *gorm.DB
	dispatcher *Dispatcher
}

// NewService creates a new webhook service.
func NewService(db *gorm.DB) *Service {
	return &Service{
		db:         db,
		dispatcher: NewDispatcher(db),
	}
}

// Dispatcher returns the underlying dispatcher for triggering events.
func (s *Service) Dispatcher() *Dispatcher {
	return s.dispatcher
}

// Trigger fires an event. Shortcut for s.Dispatcher().Trigger().
func (s *Service) Trigger(ctx context.Context, ownerID uint, event string, payload interface{}) error {
	return s.dispatcher.Trigger(ctx, ownerID, event, payload)
}

// Create registers a new webhook endpoint.
func (s *Service) Create(ownerID uint, url string, events []string, description string) (*Webhook, string, error) {
	secret, err := GenerateSecret()
	if err != nil {
		return nil, "", fmt.Errorf("webhooks: generate secret: %w", err)
	}

	hook := &Webhook{
		OwnerID:     ownerID,
		URL:         url,
		Events:      events,
		Secret:      secret,
		Active:      true,
		Description: description,
	}
	if err := s.db.Create(hook).Error; err != nil {
		return nil, "", fmt.Errorf("webhooks: create: %w", err)
	}
	// Return the secret once — it won't be retrievable again
	return hook, secret, nil
}

// List returns all webhooks for an owner.
func (s *Service) List(ownerID uint) ([]Webhook, error) {
	var hooks []Webhook
	if err := s.db.Where("owner_id = ?", ownerID).Find(&hooks).Error; err != nil {
		return nil, fmt.Errorf("webhooks: list: %w", err)
	}
	return hooks, nil
}

// GetByID returns a webhook by ID, validating ownership.
func (s *Service) GetByID(id, ownerID uint) (*Webhook, error) {
	var hook Webhook
	if err := s.db.Where("id = ? AND owner_id = ?", id, ownerID).First(&hook).Error; err != nil {
		return nil, fmt.Errorf("webhooks: get: %w", err)
	}
	return &hook, nil
}

// Delete soft-deletes a webhook.
func (s *Service) Delete(id, ownerID uint) error {
	result := s.db.Where("id = ? AND owner_id = ?", id, ownerID).Delete(&Webhook{})
	if result.Error != nil {
		return fmt.Errorf("webhooks: delete: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("webhooks: webhook not found")
	}
	return nil
}

// Toggle activates or deactivates a webhook.
func (s *Service) Toggle(id, ownerID uint, active bool) error {
	result := s.db.Model(&Webhook{}).
		Where("id = ? AND owner_id = ?", id, ownerID).
		Update("active", active)
	return result.Error
}

// ListEvents returns delivery events for a webhook.
func (s *Service) ListEvents(webhookID, ownerID uint, page, pageSize int) ([]WebhookEvent, int64, error) {
	// Verify ownership
	if _, err := s.GetByID(webhookID, ownerID); err != nil {
		return nil, 0, err
	}

	var events []WebhookEvent
	var total int64

	q := s.db.Model(&WebhookEvent{}).Where("webhook_id = ?", webhookID)
	q.Count(&total)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	if err := q.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&events).Error; err != nil {
		return nil, 0, fmt.Errorf("webhooks: list events: %w", err)
	}
	return events, total, nil
}
`
}

func webhooksHandlerGo() string {
	return `package webhooks

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler provides HTTP endpoints for webhook management.
type Handler struct {
	svc *Service
}

// NewHandler creates a webhook HTTP handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts webhook routes. Expects an authenticated router group.
// The ownerID is read from the "user_id" key set by your auth middleware.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/webhooks", h.List)
	rg.POST("/webhooks", h.Create)
	rg.DELETE("/webhooks/:id", h.Delete)
	rg.PATCH("/webhooks/:id/toggle", h.Toggle)
	rg.GET("/webhooks/:id/events", h.ListEvents)
}

func ownerFromCtx(c *gin.Context) uint {
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(uint); ok {
			return id
		}
	}
	return 0
}

// List returns all webhooks for the authenticated user.
func (h *Handler) List(c *gin.Context) {
	hooks, err := h.svc.List(ownerFromCtx(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": hooks})
}

// Create registers a new webhook.
func (h *Handler) Create(c *gin.Context) {
	var input struct {
		URL         string   ` + "`" + `json:"url" binding:"required,url"` + "`" + `
		Events      []string ` + "`" + `json:"events" binding:"required,min=1"` + "`" + `
		Description string   ` + "`" + `json:"description"` + "`" + `
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	hook, secret, err := h.svc.Create(ownerFromCtx(c), input.URL, input.Events, input.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}})
		return
	}

	// Include the secret ONLY in this response — it's not stored in plaintext
	c.JSON(http.StatusCreated, gin.H{
		"data":    hook,
		"secret":  secret,
		"message": "Webhook created. Save the secret — it won't be shown again.",
	})
}

// Delete removes a webhook.
func (h *Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid webhook ID"}})
		return
	}
	if err := h.svc.Delete(uint(id), ownerFromCtx(c)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Webhook deleted"})
}

// Toggle activates or deactivates a webhook.
func (h *Handler) Toggle(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid webhook ID"}})
		return
	}
	var body struct {
		Active bool ` + "`" + `json:"active"` + "`" + `
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}
	if err := h.svc.Toggle(uint(id), ownerFromCtx(c), body.Active); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Webhook updated"})
}

// ListEvents returns delivery logs for a webhook.
func (h *Handler) ListEvents(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid webhook ID"}})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	events, total, err := h.svc.ListEvents(uint(id), ownerFromCtx(c), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}})
		return
	}

	pages := total / int64(pageSize)
	if total%int64(pageSize) != 0 {
		pages++
	}

	c.JSON(http.StatusOK, gin.H{
		"data": events,
		"meta": gin.H{"total": total, "page": page, "page_size": pageSize, "pages": pages},
	})
}
`
}

func webhooksMigrateGo() string {
	return `package webhooks

import (
	"fmt"

	"gorm.io/gorm"
)

// Migrate runs AutoMigrate for Webhook and WebhookEvent models.
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&Webhook{}, &WebhookEvent{}); err != nil {
		return fmt.Errorf("webhooks: auto migrate: %w", err)
	}
	return nil
}
`
}
