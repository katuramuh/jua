package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"
)

func writeJuaBillingFiles(root string, opts Options) error {
	apiRoot := opts.APIRoot(root)
	module := opts.Module()

	files := map[string]string{
		filepath.Join(apiRoot, "internal", "billing", "model.go"):    billingModelGo(),
		filepath.Join(apiRoot, "internal", "billing", "service.go"):  billingServiceGo(),
		filepath.Join(apiRoot, "internal", "billing", "handler.go"):  billingHandlerGo(),
		filepath.Join(apiRoot, "internal", "billing", "migrate.go"):  billingMigrateGo(),
	}

	for path, content := range files {
		content = strings.ReplaceAll(content, "{{MODULE}}", module)
		if err := writeFile(path, content); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

func billingModelGo() string {
	return `package billing

import (
	"time"

	"gorm.io/gorm"
)

// BillingInterval represents how often a subscription is charged.
type BillingInterval string

const (
	IntervalMonthly BillingInterval = "monthly"
	IntervalYearly  BillingInterval = "yearly"
	IntervalOneTime BillingInterval = "one_time"
)

// SubscriptionStatus represents the current state of a subscription.
type SubscriptionStatus string

const (
	SubActive    SubscriptionStatus = "active"
	SubTrialing  SubscriptionStatus = "trialing"
	SubPastDue   SubscriptionStatus = "past_due"
	SubCancelled SubscriptionStatus = "cancelled"
	SubExpired   SubscriptionStatus = "expired"
)

// Plan represents a billing plan that customers can subscribe to.
type Plan struct {
	ID        uint           ` + "`" + `gorm:"primarykey" json:"id"` + "`" + `
	CreatedAt time.Time      ` + "`" + `json:"created_at"` + "`" + `
	UpdatedAt time.Time      ` + "`" + `json:"updated_at"` + "`" + `
	DeletedAt gorm.DeletedAt ` + "`" + `gorm:"index" json:"-"` + "`" + `

	// Name is the human-readable plan name, e.g. "Pro Monthly"
	Name string ` + "`" + `gorm:"not null" json:"name"` + "`" + `

	// Slug is the URL-safe identifier, e.g. "pro-monthly"
	Slug string ` + "`" + `gorm:"uniqueIndex;not null" json:"slug"` + "`" + `

	// Description is a short marketing description.
	Description string ` + "`" + `json:"description"` + "`" + `

	// Amount is the price in the smallest currency unit (e.g. UGX, cents).
	Amount float64 ` + "`" + `gorm:"not null" json:"amount"` + "`" + `

	// Currency is the ISO 4217 currency code, e.g. "UGX", "USD"
	Currency string ` + "`" + `gorm:"not null;default:'UGX'" json:"currency"` + "`" + `

	// Interval is the billing cycle.
	Interval BillingInterval ` + "`" + `gorm:"not null;default:'monthly'" json:"interval"` + "`" + `

	// TrialDays is the number of free trial days (0 = no trial).
	TrialDays int ` + "`" + `gorm:"default:0" json:"trial_days"` + "`" + `

	// Features is a JSON-serialised list of feature keys included in this plan.
	Features []string ` + "`" + `gorm:"serializer:json" json:"features"` + "`" + `

	// Active controls whether this plan is available for new subscriptions.
	Active bool ` + "`" + `gorm:"default:true" json:"active"` + "`" + `

	// MaxUsers is the seat limit for this plan (0 = unlimited).
	MaxUsers int ` + "`" + `gorm:"default:0" json:"max_users"` + "`" + `

	// ExternalID is the plan/price ID from a payment provider (e.g. Stripe price ID).
	ExternalID string ` + "`" + `json:"external_id,omitempty"` + "`" + `
}

// TableName returns the GORM table name.
func (Plan) TableName() string { return "billing_plans" }

// Subscription represents a customer's active subscription to a plan.
type Subscription struct {
	ID        uint           ` + "`" + `gorm:"primarykey" json:"id"` + "`" + `
	CreatedAt time.Time      ` + "`" + `json:"created_at"` + "`" + `
	UpdatedAt time.Time      ` + "`" + `json:"updated_at"` + "`" + `
	DeletedAt gorm.DeletedAt ` + "`" + `gorm:"index" json:"-"` + "`" + `

	// OwnerID is the user or tenant this subscription belongs to.
	OwnerID uint ` + "`" + `gorm:"not null;index" json:"owner_id"` + "`" + `

	// PlanID references the billing plan.
	PlanID uint ` + "`" + `gorm:"not null" json:"plan_id"` + "`" + `
	Plan   Plan ` + "`" + `gorm:"foreignKey:PlanID" json:"plan"` + "`" + `

	// Status is the current subscription state.
	Status SubscriptionStatus ` + "`" + `gorm:"not null;default:'trialing'" json:"status"` + "`" + `

	// CurrentPeriodStart is when the current billing period started.
	CurrentPeriodStart time.Time ` + "`" + `json:"current_period_start"` + "`" + `

	// CurrentPeriodEnd is when the current billing period ends.
	CurrentPeriodEnd time.Time ` + "`" + `json:"current_period_end"` + "`" + `

	// TrialEnd is when the free trial expires (nil = not in trial).
	TrialEnd *time.Time ` + "`" + `json:"trial_end,omitempty"` + "`" + `

	// CancelAtPeriodEnd defers cancellation to end of current period.
	CancelAtPeriodEnd bool ` + "`" + `gorm:"default:false" json:"cancel_at_period_end"` + "`" + `

	// CancelledAt is when the subscription was cancelled.
	CancelledAt *time.Time ` + "`" + `json:"cancelled_at,omitempty"` + "`" + `

	// ExternalID is the subscription ID from a payment provider.
	ExternalID string ` + "`" + `json:"external_id,omitempty"` + "`" + `
}

// TableName returns the GORM table name.
func (Subscription) TableName() string { return "subscriptions" }

// IsActive returns true if the subscription is usable.
func (s *Subscription) IsActive() bool {
	return s.Status == SubActive || s.Status == SubTrialing
}

// Invoice represents a billing invoice / payment record.
type Invoice struct {
	ID        uint      ` + "`" + `gorm:"primarykey" json:"id"` + "`" + `
	CreatedAt time.Time ` + "`" + `json:"created_at"` + "`" + `
	UpdatedAt time.Time ` + "`" + `json:"updated_at"` + "`" + `

	// OwnerID is the user or tenant being billed.
	OwnerID uint ` + "`" + `gorm:"not null;index" json:"owner_id"` + "`" + `

	// SubscriptionID links to the subscription (optional for one-time charges).
	SubscriptionID *uint ` + "`" + `json:"subscription_id,omitempty"` + "`" + `

	// Amount is the total charge in the smallest currency unit.
	Amount float64 ` + "`" + `gorm:"not null" json:"amount"` + "`" + `

	// Currency is the ISO 4217 currency code.
	Currency string ` + "`" + `gorm:"not null;default:'UGX'" json:"currency"` + "`" + `

	// Status: "draft", "open", "paid", "void", "uncollectible"
	Status string ` + "`" + `gorm:"not null;default:'open'" json:"status"` + "`" + `

	// PaidAt is when the invoice was paid.
	PaidAt *time.Time ` + "`" + `json:"paid_at,omitempty"` + "`" + `

	// TransactionID is the payment provider's transaction reference.
	TransactionID string ` + "`" + `json:"transaction_id,omitempty"` + "`" + `

	// Description is a human-readable description for the invoice line item.
	Description string ` + "`" + `json:"description"` + "`" + `

	// ExternalID is the invoice ID from the payment provider.
	ExternalID string ` + "`" + `json:"external_id,omitempty"` + "`" + `
}

// TableName returns the GORM table name.
func (Invoice) TableName() string { return "invoices" }
`
}

func billingServiceGo() string {
	return `package billing

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ErrNoPlan is returned when the requested plan does not exist.
var ErrNoPlan = errors.New("billing: plan not found")

// ErrAlreadySubscribed is returned when an owner already has an active subscription.
var ErrAlreadySubscribed = errors.New("billing: owner already has an active subscription")

// Service handles billing plan management, subscriptions, and invoices.
type Service struct {
	db *gorm.DB
}

// NewService creates a new billing service.
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// --- Plans ---

// CreatePlan creates a new billing plan.
func (s *Service) CreatePlan(plan *Plan) error {
	if err := s.db.Create(plan).Error; err != nil {
		return fmt.Errorf("billing: create plan: %w", err)
	}
	return nil
}

// ListPlans returns all active billing plans.
func (s *Service) ListPlans(includeInactive bool) ([]Plan, error) {
	var plans []Plan
	q := s.db.Model(&Plan{})
	if !includeInactive {
		q = q.Where("active = true")
	}
	if err := q.Order("amount ASC").Find(&plans).Error; err != nil {
		return nil, fmt.Errorf("billing: list plans: %w", err)
	}
	return plans, nil
}

// GetPlanBySlug fetches a plan by its slug.
func (s *Service) GetPlanBySlug(slug string) (*Plan, error) {
	var plan Plan
	if err := s.db.Where("slug = ?", slug).First(&plan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNoPlan
		}
		return nil, fmt.Errorf("billing: get plan: %w", err)
	}
	return &plan, nil
}

// --- Subscriptions ---

// Subscribe creates a new subscription for an owner.
// If the plan has trial days, the subscription starts in trialing status.
func (s *Service) Subscribe(ownerID, planID uint) (*Subscription, error) {
	// Check for existing active subscription
	var existing Subscription
	err := s.db.Where("owner_id = ? AND status IN ?", ownerID,
		[]SubscriptionStatus{SubActive, SubTrialing, SubPastDue}).
		First(&existing).Error
	if err == nil {
		return nil, ErrAlreadySubscribed
	}

	var plan Plan
	if err := s.db.First(&plan, planID).Error; err != nil {
		return nil, ErrNoPlan
	}

	now := time.Now()
	status := SubActive
	var trialEnd *time.Time
	periodEnd := nextPeriodEnd(now, plan.Interval)

	if plan.TrialDays > 0 {
		status = SubTrialing
		te := now.Add(time.Duration(plan.TrialDays) * 24 * time.Hour)
		trialEnd = &te
		periodEnd = te
	}

	sub := &Subscription{
		OwnerID:            ownerID,
		PlanID:             planID,
		Status:             status,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   periodEnd,
		TrialEnd:           trialEnd,
	}
	if err := s.db.Create(sub).Error; err != nil {
		return nil, fmt.Errorf("billing: subscribe: %w", err)
	}
	return sub, nil
}

// GetActiveSubscription returns the active subscription for an owner.
func (s *Service) GetActiveSubscription(ownerID uint) (*Subscription, error) {
	var sub Subscription
	err := s.db.Where("owner_id = ? AND status IN ?", ownerID,
		[]SubscriptionStatus{SubActive, SubTrialing}).
		Preload("Plan").
		First(&sub).Error
	if err != nil {
		return nil, fmt.Errorf("billing: get subscription: %w", err)
	}
	return &sub, nil
}

// Cancel marks a subscription for cancellation at period end.
func (s *Service) Cancel(ownerID uint) error {
	result := s.db.Model(&Subscription{}).
		Where("owner_id = ? AND status IN ?", ownerID,
			[]SubscriptionStatus{SubActive, SubTrialing}).
		Updates(map[string]interface{}{
			"cancel_at_period_end": true,
		})
	if result.RowsAffected == 0 {
		return fmt.Errorf("billing: no active subscription to cancel")
	}
	return result.Error
}

// ExpireSubscriptions cancels subscriptions whose period has ended.
// Call from a cron job daily.
func (s *Service) ExpireSubscriptions() (int64, error) {
	now := time.Now()
	result := s.db.Model(&Subscription{}).
		Where("current_period_end < ? AND cancel_at_period_end = true", now).
		Updates(map[string]interface{}{
			"status":       SubCancelled,
			"cancelled_at": now,
		})
	return result.RowsAffected, result.Error
}

// --- Invoices ---

// CreateInvoice records a billing event (charge, payment, etc).
func (s *Service) CreateInvoice(invoice *Invoice) error {
	if err := s.db.Create(invoice).Error; err != nil {
		return fmt.Errorf("billing: create invoice: %w", err)
	}
	return nil
}

// MarkPaid marks an invoice as paid.
func (s *Service) MarkPaid(invoiceID uint, transactionID string) error {
	now := time.Now()
	return s.db.Model(&Invoice{}).Where("id = ?", invoiceID).
		Updates(map[string]interface{}{
			"status":         "paid",
			"paid_at":        now,
			"transaction_id": transactionID,
		}).Error
}

// ListInvoices returns paginated invoices for an owner.
func (s *Service) ListInvoices(ownerID uint, page, pageSize int) ([]Invoice, int64, error) {
	var invoices []Invoice
	var total int64

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	q := s.db.Model(&Invoice{}).Where("owner_id = ?", ownerID)
	q.Count(&total)
	err := q.Offset((page-1)*pageSize).Limit(pageSize).
		Order("created_at DESC").Find(&invoices).Error
	return invoices, total, err
}

// nextPeriodEnd calculates the end of the next billing period.
func nextPeriodEnd(from time.Time, interval BillingInterval) time.Time {
	switch interval {
	case IntervalYearly:
		return from.AddDate(1, 0, 0)
	case IntervalOneTime:
		return from.AddDate(100, 0, 0) // effectively never expires
	default: // monthly
		return from.AddDate(0, 1, 0)
	}
}
`
}

func billingHandlerGo() string {
	return `package billing

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	// Jua events — fire domain events for real-time + notifications
	"{{MODULE}}/jua/events"
)

// Handler provides HTTP endpoints for billing management.
type Handler struct {
	svc *Service
}

// NewHandler creates a billing HTTP handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterPublicRoutes mounts public billing routes (plan listing).
func (h *Handler) RegisterPublicRoutes(rg *gin.RouterGroup) {
	rg.GET("/billing/plans", h.ListPlans)
}

// RegisterAuthRoutes mounts authenticated billing routes.
func (h *Handler) RegisterAuthRoutes(rg *gin.RouterGroup) {
	rg.GET("/billing/subscription", h.GetSubscription)
	rg.POST("/billing/subscribe", h.Subscribe)
	rg.DELETE("/billing/subscription", h.CancelSubscription)
	rg.GET("/billing/invoices", h.ListInvoices)
}

// RegisterAdminRoutes mounts admin billing routes.
func (h *Handler) RegisterAdminRoutes(rg *gin.RouterGroup) {
	rg.GET("/billing/plans", h.ListPlans)
	rg.POST("/billing/plans", h.CreatePlan)
}

func ownerID(c *gin.Context) uint {
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(uint); ok {
			return id
		}
	}
	return 0
}

// ListPlans returns all active billing plans.
func (h *Handler) ListPlans(c *gin.Context) {
	plans, err := h.svc.ListPlans(false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": plans})
}

// CreatePlan creates a new billing plan (admin only).
func (h *Handler) CreatePlan(c *gin.Context) {
	var plan Plan
	if err := c.ShouldBindJSON(&plan); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}
	if err := h.svc.CreatePlan(&plan); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": plan, "message": "Plan created"})
}

// GetSubscription returns the active subscription for the authenticated user.
func (h *Handler) GetSubscription(c *gin.Context) {
	sub, err := h.svc.GetActiveSubscription(ownerID(c))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "No active subscription"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": sub})
}

// Subscribe creates a new subscription for the authenticated user.
func (h *Handler) Subscribe(c *gin.Context) {
	var body struct {
		PlanID uint ` + "`" + `json:"plan_id" binding:"required"` + "`" + `
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	sub, err := h.svc.Subscribe(ownerID(c), body.PlanID)
	if err != nil {
		code := http.StatusInternalServerError
		errCode := "INTERNAL_ERROR"
		if err == ErrAlreadySubscribed {
			code = http.StatusConflict
			errCode = "ALREADY_SUBSCRIBED"
		} else if err == ErrNoPlan {
			code = http.StatusNotFound
			errCode = "PLAN_NOT_FOUND"
		}
		c.JSON(code, gin.H{"error": gin.H{"code": errCode, "message": err.Error()}})
		return
	}
	// Jua events: notify real-time + notification engine on subscription activation
	events.Publish(events.SubscriptionActivated, "", map[string]interface{}{
		"plan_id": body.PlanID,
		"user_id": ownerID(c),
	})
	c.JSON(http.StatusCreated, gin.H{"data": sub, "message": "Subscription created"})
}

// CancelSubscription cancels the authenticated user's subscription at period end.
func (h *Handler) CancelSubscription(c *gin.Context) {
	if err := h.svc.Cancel(ownerID(c)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "CANCEL_FAILED", "message": err.Error()}})
		return
	}
	// Jua events: fire cancellation event
	events.Publish(events.SubscriptionCancelled, "", map[string]interface{}{"user_id": ownerID(c)})
	c.JSON(http.StatusOK, gin.H{"message": "Subscription will be cancelled at the end of the current period"})
}

// ListInvoices returns paginated invoices for the authenticated user.
func (h *Handler) ListInvoices(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	invoices, total, err := h.svc.ListInvoices(ownerID(c), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}})
		return
	}

	pages := total / int64(pageSize)
	if total%int64(pageSize) != 0 {
		pages++
	}

	c.JSON(http.StatusOK, gin.H{
		"data": invoices,
		"meta": gin.H{"total": total, "page": page, "page_size": pageSize, "pages": pages},
	})
}
`
}

func billingMigrateGo() string {
	return `package billing

import (
	"fmt"

	"gorm.io/gorm"
)

// Migrate runs AutoMigrate for all billing models.
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&Plan{}, &Subscription{}, &Invoice{}); err != nil {
		return fmt.Errorf("billing: auto migrate: %w", err)
	}
	return nil
}
`
}
