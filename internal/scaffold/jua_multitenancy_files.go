package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"
)

func writeJuaMultitenancyFiles(root string, opts Options) error {
	apiRoot := opts.APIRoot(root)
	module := opts.Module()

	files := map[string]string{
		filepath.Join(apiRoot, "internal", "tenancy", "model.go"):      tenancyModelGo(),
		filepath.Join(apiRoot, "internal", "tenancy", "context.go"):    tenancyContextGo(),
		filepath.Join(apiRoot, "internal", "tenancy", "resolver.go"):   tenancyResolverGo(),
		filepath.Join(apiRoot, "internal", "tenancy", "middleware.go"): tenancyMiddlewareGo(),
		filepath.Join(apiRoot, "internal", "tenancy", "scope.go"):      tenancyScopeGo(),
		filepath.Join(apiRoot, "internal", "tenancy", "service.go"):    tenancyServiceGo(),
		filepath.Join(apiRoot, "internal", "tenancy", "handler.go"):    tenancyHandlerGo(),
		filepath.Join(apiRoot, "internal", "tenancy", "migrate.go"):    tenancyMigrateGo(),
	}

	for path, content := range files {
		content = strings.ReplaceAll(content, "{{MODULE}}", module)
		if err := writeFile(path, content); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

func tenancyModelGo() string {
	return `package tenancy

import (
	"time"

	"gorm.io/gorm"
)

// Plan represents a billing/feature tier for a tenant.
type Plan string

const (
	PlanFree       Plan = "free"
	PlanStarter    Plan = "starter"
	PlanPro        Plan = "pro"
	PlanEnterprise Plan = "enterprise"
)

// Status represents the operational state of a tenant.
type Status string

const (
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
	StatusTrial     Status = "trial"
	StatusCancelled Status = "cancelled"
)

// Tenant represents a tenant (organisation) in the system.
// Each tenant gets an isolated data namespace — all tenant-scoped models
// must include a tenant_id column and use the TenantScope GORM scope.
type Tenant struct {
	ID        uint           ` + "`" + `gorm:"primarykey" json:"id"` + "`" + `
	CreatedAt time.Time      ` + "`" + `json:"created_at"` + "`" + `
	UpdatedAt time.Time      ` + "`" + `json:"updated_at"` + "`" + `
	DeletedAt gorm.DeletedAt ` + "`" + `gorm:"index" json:"-"` + "`" + `

	// Name is the human-readable organisation name.
	Name string ` + "`" + `gorm:"not null" json:"name"` + "`" + `

	// Slug is the URL-safe identifier used in subdomains and API paths.
	// Example: "acme" → acme.yourdomain.com
	Slug string ` + "`" + `gorm:"uniqueIndex;not null" json:"slug"` + "`" + `

	// Domain is an optional custom domain for this tenant.
	// Example: "app.acmecorp.com"
	Domain string ` + "`" + `gorm:"uniqueIndex" json:"domain,omitempty"` + "`" + `

	// Plan is the current billing/feature plan.
	Plan Plan ` + "`" + `gorm:"default:'free'" json:"plan"` + "`" + `

	// Status is the operational state of the tenant.
	Status Status ` + "`" + `gorm:"default:'trial'" json:"status"` + "`" + `

	// TrialEndsAt is when the trial period expires (nil = no trial).
	TrialEndsAt *time.Time ` + "`" + `json:"trial_ends_at,omitempty"` + "`" + `

	// MaxUsers is the maximum number of users allowed (0 = unlimited).
	MaxUsers int ` + "`" + `gorm:"default:0" json:"max_users"` + "`" + `

	// Metadata holds arbitrary tenant-specific configuration as JSON.
	Metadata map[string]interface{} ` + "`" + `gorm:"serializer:json" json:"metadata,omitempty"` + "`" + `

	// OwnerID is the user ID of the tenant's primary administrator.
	OwnerID uint ` + "`" + `json:"owner_id"` + "`" + `
}

// TableName returns the GORM table name.
func (Tenant) TableName() string { return "tenants" }

// IsActive returns true if the tenant can be used.
func (t *Tenant) IsActive() bool {
	return t.Status == StatusActive || t.Status == StatusTrial
}

// HasFeature returns true if the tenant's plan includes the given feature key.
// Extend this to match your plan/feature matrix.
func (t *Tenant) HasFeature(feature string) bool {
	switch t.Plan {
	case PlanEnterprise:
		return true
	case PlanPro:
		return feature != "white_label" && feature != "sso"
	case PlanStarter:
		return feature == "api_access" || feature == "webhooks"
	default:
		return false
	}
}
`
}

func tenancyContextGo() string {
	return `package tenancy

import (
	"context"
	"errors"
)

type contextKey string

const tenantContextKey contextKey = "tenant"

// ErrNoTenant is returned when a tenant is not found in the context.
var ErrNoTenant = errors.New("tenancy: no tenant in context")

// WithTenant stores a tenant in the context.
func WithTenant(ctx context.Context, tenant *Tenant) context.Context {
	return context.WithValue(ctx, tenantContextKey, tenant)
}

// FromContext retrieves the tenant from the context.
// Returns ErrNoTenant if not set.
func FromContext(ctx context.Context) (*Tenant, error) {
	t, ok := ctx.Value(tenantContextKey).(*Tenant)
	if !ok || t == nil {
		return nil, ErrNoTenant
	}
	return t, nil
}

// MustFromContext retrieves the tenant from the context and panics if not set.
// Use only in contexts where the tenant middleware is guaranteed to have run.
func MustFromContext(ctx context.Context) *Tenant {
	t, err := FromContext(ctx)
	if err != nil {
		panic(err)
	}
	return t
}
`
}

func tenancyResolverGo() string {
	return `package tenancy

import (
	"net/http"
	"strings"

	"gorm.io/gorm"
)

// Resolver locates the current tenant from an HTTP request.
type Resolver struct {
	db            *gorm.DB
	rootDomain    string // e.g. "yourdomain.com" — stripped to extract subdomain slug
	headerName    string // optional header override, e.g. "X-Tenant-ID"
}

// NewResolver creates a tenant resolver.
//
//	rootDomain: your app's root domain, e.g. "yourdomain.com"
//	headerName: optional custom header for API clients, e.g. "X-Tenant-Slug"
//	            Pass "" to disable header-based resolution.
func NewResolver(db *gorm.DB, rootDomain, headerName string) *Resolver {
	return &Resolver{db: db, rootDomain: rootDomain, headerName: headerName}
}

// Resolve attempts to identify the tenant from the request using:
//  1. Custom header (if configured), e.g. X-Tenant-Slug: acme
//  2. Subdomain, e.g. acme.yourdomain.com → slug "acme"
//  3. Custom domain match (full Host header)
//
// Returns nil, nil if no tenant identifier is found (useful for public routes).
func (r *Resolver) Resolve(req *http.Request) (*Tenant, error) {
	// 1. Header-based resolution
	if r.headerName != "" {
		if slug := req.Header.Get(r.headerName); slug != "" {
			return r.findBySlug(slug)
		}
	}

	host := req.Host
	// Strip port
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// 2. Custom domain match
	if r.rootDomain != "" && !strings.HasSuffix(host, "."+r.rootDomain) && host != r.rootDomain {
		tenant, err := r.findByDomain(host)
		if err == nil && tenant != nil {
			return tenant, nil
		}
	}

	// 3. Subdomain extraction
	if r.rootDomain != "" && strings.HasSuffix(host, "."+r.rootDomain) {
		slug := strings.TrimSuffix(host, "."+r.rootDomain)
		// Skip www and api as reserved subdomains
		if slug != "" && slug != "www" && slug != "api" && slug != "admin" {
			return r.findBySlug(slug)
		}
	}

	return nil, nil
}

func (r *Resolver) findBySlug(slug string) (*Tenant, error) {
	var tenant Tenant
	if err := r.db.Where("slug = ? AND status != ?", slug, StatusCancelled).First(&tenant).Error; err != nil {
		return nil, err
	}
	return &tenant, nil
}

func (r *Resolver) findByDomain(domain string) (*Tenant, error) {
	var tenant Tenant
	if err := r.db.Where("domain = ? AND status != ?", domain, StatusCancelled).First(&tenant).Error; err != nil {
		return nil, err
	}
	return &tenant, nil
}
`
}

func tenancyMiddlewareGo() string {
	return `package tenancy

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Middleware returns a Gin middleware that resolves the current tenant and stores
// it in the request context. If no tenant is found and requireTenant is true,
// the request is rejected with 404.
//
// Usage:
//
//	resolver := tenancy.NewResolver(db, "yourdomain.com", "X-Tenant-Slug")
//	router.Use(tenancy.Middleware(resolver, true))
func Middleware(resolver *Resolver, requireTenant bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenant, err := resolver.Resolve(c.Request)
		if err != nil {
			if requireTenant {
				c.JSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"code":    "TENANT_NOT_FOUND",
						"message": "Tenant not found",
					},
				})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		if tenant == nil {
			if requireTenant {
				c.JSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"code":    "TENANT_NOT_FOUND",
						"message": "No tenant could be identified for this request",
					},
				})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		if !tenant.IsActive() {
			c.JSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"code":    "TENANT_SUSPENDED",
					"message": "This account is suspended. Please contact support.",
				},
			})
			c.Abort()
			return
		}

		// Store tenant in both Gin context and request context
		c.Set("tenant", tenant)
		ctx := WithTenant(c.Request.Context(), tenant)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// GetTenant retrieves the resolved tenant from a Gin context.
// Returns nil if no tenant was set (public routes).
func GetTenant(c *gin.Context) *Tenant {
	if v, exists := c.Get("tenant"); exists {
		if t, ok := v.(*Tenant); ok {
			return t
		}
	}
	return nil
}

// RequireTenant is a Gin middleware that returns 401 if no tenant is in context.
// Use after Middleware(resolver, false) to selectively require a tenant.
func RequireTenant() gin.HandlerFunc {
	return func(c *gin.Context) {
		if GetTenant(c) == nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "TENANT_REQUIRED",
					"message": "A tenant context is required for this endpoint",
				},
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireFeature returns a middleware that checks the tenant has the given feature.
func RequireFeature(feature string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenant := GetTenant(c)
		if tenant == nil || !tenant.HasFeature(feature) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"code":    "FEATURE_NOT_AVAILABLE",
					"message": "Your current plan does not include this feature",
				},
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
`
}

func tenancyScopeGo() string {
	return `package tenancy

import (
	"context"

	"gorm.io/gorm"
)

// TenantScope returns a GORM scope that filters queries by the current tenant.
// Use it in all queries on tenant-scoped models.
//
// Example:
//
//	db.WithContext(ctx).Scopes(tenancy.TenantScope(ctx)).Find(&posts)
func TenantScope(ctx context.Context) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		tenant, err := FromContext(ctx)
		if err != nil {
			// No tenant in context — return a scope that matches nothing
			// to prevent accidental cross-tenant data leaks.
			return db.Where("1 = 0")
		}
		return db.Where("tenant_id = ?", tenant.ID)
	}
}

// SetTenantID is a GORM BeforeCreate hook helper.
// Call it in your model's BeforeCreate hook to auto-assign the tenant ID.
//
// Example in a model:
//
//	func (p *Post) BeforeCreate(tx *gorm.DB) error {
//	    return tenancy.SetTenantID(tx.Statement.Context, p)
//	}
func SetTenantID(ctx context.Context, model interface{ SetTenant(id uint) }) error {
	tenant, err := FromContext(ctx)
	if err != nil {
		return err
	}
	model.SetTenant(tenant.ID)
	return nil
}

// TenantModel is an embeddable struct that adds tenant_id to any GORM model.
// Models that embed TenantModel are automatically scoped to a tenant.
//
// Example:
//
//	type Post struct {
//	    tenancy.TenantModel
//	    Title   string
//	    Content string
//	}
type TenantModel struct {
	TenantID uint ` + "`" + `gorm:"not null;index" json:"tenant_id"` + "`" + `
}

// SetTenant satisfies the SetTenantID interface.
func (t *TenantModel) SetTenant(id uint) { t.TenantID = id }
`
}

func tenancyServiceGo() string {
	return `package tenancy

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ErrSlugTaken is returned when a slug is already in use.
var ErrSlugTaken = errors.New("tenancy: slug is already taken")

// ErrDomainTaken is returned when a custom domain is already registered.
var ErrDomainTaken = errors.New("tenancy: domain is already registered")

var slugRe = regexp.MustCompile(`+"`"+`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`+"`"+`)

// Service handles tenant CRUD and lifecycle operations.
type Service struct {
	db *gorm.DB
}

// NewService creates a new tenant service.
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// CreateInput holds the fields required to create a new tenant.
type CreateInput struct {
	Name    string
	Slug    string
	OwnerID uint
	Plan    Plan
}

// Create creates a new tenant with a 14-day trial.
func (s *Service) Create(input CreateInput) (*Tenant, error) {
	slug := strings.ToLower(strings.TrimSpace(input.Slug))
	if !slugRe.MatchString(slug) {
		return nil, fmt.Errorf("tenancy: slug %q is invalid — use 2-63 lowercase letters, numbers, or hyphens", slug)
	}

	// Check uniqueness
	var count int64
	s.db.Model(&Tenant{}).Where("slug = ?", slug).Count(&count)
	if count > 0 {
		return nil, ErrSlugTaken
	}

	plan := input.Plan
	if plan == "" {
		plan = PlanFree
	}

	trialEnd := time.Now().Add(14 * 24 * time.Hour)
	tenant := &Tenant{
		Name:        strings.TrimSpace(input.Name),
		Slug:        slug,
		OwnerID:     input.OwnerID,
		Plan:        plan,
		Status:      StatusTrial,
		TrialEndsAt: &trialEnd,
	}

	if err := s.db.Create(tenant).Error; err != nil {
		return nil, fmt.Errorf("tenancy: create: %w", err)
	}
	return tenant, nil
}

// GetByID fetches a tenant by primary key.
func (s *Service) GetByID(id uint) (*Tenant, error) {
	var tenant Tenant
	if err := s.db.First(&tenant, id).Error; err != nil {
		return nil, fmt.Errorf("tenancy: get by id: %w", err)
	}
	return &tenant, nil
}

// GetBySlug fetches a tenant by slug.
func (s *Service) GetBySlug(slug string) (*Tenant, error) {
	var tenant Tenant
	if err := s.db.Where("slug = ?", slug).First(&tenant).Error; err != nil {
		return nil, fmt.Errorf("tenancy: get by slug: %w", err)
	}
	return &tenant, nil
}

// List returns all tenants with optional status filter.
func (s *Service) List(status Status, page, pageSize int) ([]Tenant, int64, error) {
	var tenants []Tenant
	var total int64

	q := s.db.Model(&Tenant{})
	if status != "" {
		q = q.Where("status = ?", status)
	}

	q.Count(&total)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	if err := q.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&tenants).Error; err != nil {
		return nil, 0, fmt.Errorf("tenancy: list: %w", err)
	}
	return tenants, total, nil
}

// SetCustomDomain assigns a verified custom domain to a tenant.
func (s *Service) SetCustomDomain(tenantID uint, domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))

	var count int64
	s.db.Model(&Tenant{}).Where("domain = ? AND id != ?", domain, tenantID).Count(&count)
	if count > 0 {
		return ErrDomainTaken
	}

	return s.db.Model(&Tenant{}).Where("id = ?", tenantID).Update("domain", domain).Error
}

// Suspend suspends a tenant (blocks all API access).
func (s *Service) Suspend(tenantID uint) error {
	return s.db.Model(&Tenant{}).Where("id = ?", tenantID).Update("status", StatusSuspended).Error
}

// Activate activates or re-activates a tenant.
func (s *Service) Activate(tenantID uint) error {
	return s.db.Model(&Tenant{}).Where("id = ?", tenantID).Update("status", StatusActive).Error
}

// Upgrade changes a tenant's plan.
func (s *Service) Upgrade(tenantID uint, plan Plan) error {
	updates := map[string]interface{}{"plan": plan, "status": StatusActive}
	return s.db.Model(&Tenant{}).Where("id = ?", tenantID).Updates(updates).Error
}

// ExpireTrials marks tenants whose trial has ended as suspended.
// Call this from a cron job.
func (s *Service) ExpireTrials() (int64, error) {
	result := s.db.Model(&Tenant{}).
		Where("status = ? AND trial_ends_at < NOW()", StatusTrial).
		Update("status", StatusSuspended)
	return result.RowsAffected, result.Error
}
`
}

func tenancyHandlerGo() string {
	return `package tenancy

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler provides admin HTTP handlers for tenant management.
// Mount with: adminGroup.GET("/tenants", h.List)
type Handler struct {
	svc *Service
}

// NewHandler creates a new tenant admin handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts all tenant admin routes onto a Gin router group.
// Expects the group to already be protected by admin auth middleware.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/tenants", h.List)
	rg.POST("/tenants", h.Create)
	rg.GET("/tenants/:id", h.Get)
	rg.PATCH("/tenants/:id/suspend", h.Suspend)
	rg.PATCH("/tenants/:id/activate", h.Activate)
	rg.PATCH("/tenants/:id/upgrade", h.Upgrade)
	rg.PATCH("/tenants/:id/domain", h.SetDomain)
}

// List godoc
// @Summary List all tenants
// @Tags Admin/Tenants
// @Produce json
// @Param status query string false "Filter by status"
// @Param page query int false "Page number"
// @Param page_size query int false "Items per page"
// @Success 200 {object} map[string]interface{}
// @Router /admin/tenants [get]
func (h *Handler) List(c *gin.Context) {
	status := Status(c.Query("status"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	tenants, total, err := h.svc.List(status, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()},
		})
		return
	}

	pages := total / int64(pageSize)
	if total%int64(pageSize) != 0 {
		pages++
	}

	c.JSON(http.StatusOK, gin.H{
		"data": tenants,
		"meta": gin.H{
			"total":     total,
			"page":      page,
			"page_size": pageSize,
			"pages":     pages,
		},
	})
}

// Create godoc
// @Summary Create a new tenant
// @Tags Admin/Tenants
// @Accept json
// @Produce json
// @Param body body CreateInput true "Tenant data"
// @Success 201 {object} map[string]interface{}
// @Router /admin/tenants [post]
func (h *Handler) Create(c *gin.Context) {
	var input CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()},
		})
		return
	}

	tenant, err := h.svc.Create(input)
	if err != nil {
		code := http.StatusInternalServerError
		errCode := "INTERNAL_ERROR"
		if err == ErrSlugTaken {
			code = http.StatusConflict
			errCode = "SLUG_TAKEN"
		}
		c.JSON(code, gin.H{"error": gin.H{"code": errCode, "message": err.Error()}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": tenant, "message": "Tenant created successfully"})
}

// Get godoc
// @Summary Get a tenant by ID
// @Tags Admin/Tenants
// @Produce json
// @Param id path int true "Tenant ID"
// @Success 200 {object} map[string]interface{}
// @Router /admin/tenants/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid tenant ID"}})
		return
	}

	tenant, err := h.svc.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Tenant not found"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tenant})
}

// Suspend godoc
// @Summary Suspend a tenant
// @Tags Admin/Tenants
// @Produce json
// @Param id path int true "Tenant ID"
// @Success 200 {object} map[string]interface{}
// @Router /admin/tenants/{id}/suspend [patch]
func (h *Handler) Suspend(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid tenant ID"}})
		return
	}
	if err := h.svc.Suspend(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Tenant suspended"})
}

// Activate godoc
// @Summary Activate a tenant
// @Tags Admin/Tenants
// @Produce json
// @Param id path int true "Tenant ID"
// @Success 200 {object} map[string]interface{}
// @Router /admin/tenants/{id}/activate [patch]
func (h *Handler) Activate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid tenant ID"}})
		return
	}
	if err := h.svc.Activate(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Tenant activated"})
}

// Upgrade godoc
// @Summary Upgrade a tenant's plan
// @Tags Admin/Tenants
// @Accept json
// @Produce json
// @Param id path int true "Tenant ID"
// @Param body body map[string]string true "Plan"
// @Success 200 {object} map[string]interface{}
// @Router /admin/tenants/{id}/upgrade [patch]
func (h *Handler) Upgrade(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid tenant ID"}})
		return
	}

	var body struct {
		Plan Plan ` + "`" + `json:"plan" binding:"required"` + "`" + `
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	if err := h.svc.Upgrade(uint(id), body.Plan); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Tenant plan updated"})
}

// SetDomain godoc
// @Summary Set a custom domain for a tenant
// @Tags Admin/Tenants
// @Accept json
// @Produce json
// @Param id path int true "Tenant ID"
// @Param body body map[string]string true "Domain"
// @Success 200 {object} map[string]interface{}
// @Router /admin/tenants/{id}/domain [patch]
func (h *Handler) SetDomain(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid tenant ID"}})
		return
	}

	var body struct {
		Domain string ` + "`" + `json:"domain" binding:"required"` + "`" + `
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	if err := h.svc.SetCustomDomain(uint(id), body.Domain); err != nil {
		code := http.StatusInternalServerError
		errCode := "INTERNAL_ERROR"
		if err == ErrDomainTaken {
			code = http.StatusConflict
			errCode = "DOMAIN_TAKEN"
		}
		c.JSON(code, gin.H{"error": gin.H{"code": errCode, "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Custom domain updated"})
}
`
}

func tenancyMigrateGo() string {
	return `package tenancy

import (
	"fmt"

	"gorm.io/gorm"
)

// Migrate runs AutoMigrate for the Tenant model.
// Call this from your database migration entrypoint.
//
// Example in cmd/migrate/main.go:
//
//	if err := tenancy.Migrate(db); err != nil {
//	    log.Fatalf("tenancy migrate: %v", err)
//	}
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&Tenant{}); err != nil {
		return fmt.Errorf("tenancy: auto migrate: %w", err)
	}
	return nil
}
`
}
