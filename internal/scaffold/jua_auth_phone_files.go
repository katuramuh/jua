package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"
)

func writeJuaAuthPhoneFiles(root string, opts Options) error {
	apiRoot := opts.APIRoot(root)
	module := opts.Module()

	files := map[string]string{
		filepath.Join(apiRoot, "internal", "auth", "phone", "model.go"):   phoneOTPModelGo(),
		filepath.Join(apiRoot, "internal", "auth", "phone", "service.go"): phoneOTPServiceGo(),
		filepath.Join(apiRoot, "internal", "auth", "phone", "handler.go"): phoneOTPHandlerGo(),
		filepath.Join(apiRoot, "internal", "auth", "phone", "migrate.go"): phoneOTPMigrateGo(),
	}

	for path, content := range files {
		content = strings.ReplaceAll(content, "{{MODULE}}", module)
		if err := writeFile(path, content); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

func phoneOTPModelGo() string {
	return `package phone

import (
	"time"

	"gorm.io/gorm"
)

// PhoneOTP stores a one-time password for phone number verification.
// Records expire after ExpiresAt and are deleted after successful verification.
type PhoneOTP struct {
	ID        uint           ` + "`" + `gorm:"primarykey" json:"id"` + "`" + `
	CreatedAt time.Time      ` + "`" + `json:"created_at"` + "`" + `
	UpdatedAt time.Time      ` + "`" + `json:"updated_at"` + "`" + `
	DeletedAt gorm.DeletedAt ` + "`" + `gorm:"index" json:"-"` + "`" + `

	// Phone is the E.164 phone number this OTP was sent to.
	Phone string ` + "`" + `gorm:"not null;index" json:"phone"` + "`" + `

	// CodeHash is the bcrypt hash of the 6-digit OTP.
	// Never store the raw OTP.
	CodeHash string ` + "`" + `gorm:"not null" json:"-"` + "`" + `

	// Attempts is the number of failed verification attempts.
	Attempts int ` + "`" + `gorm:"default:0" json:"-"` + "`" + `

	// ExpiresAt is when this OTP becomes invalid.
	ExpiresAt time.Time ` + "`" + `gorm:"not null;index" json:"expires_at"` + "`" + `

	// Purpose distinguishes between different OTP flows.
	// Values: "login", "register", "verify_phone", "reset_password"
	Purpose string ` + "`" + `gorm:"not null;default:'login'" json:"purpose"` + "`" + `

	// Verified marks that this OTP has been successfully used.
	Verified bool ` + "`" + `gorm:"default:false" json:"-"` + "`" + `
}

// TableName returns the GORM table name.
func (PhoneOTP) TableName() string { return "phone_otps" }

// IsExpired returns true if the OTP has passed its expiry time.
func (p *PhoneOTP) IsExpired() bool {
	return time.Now().After(p.ExpiresAt)
}

// IsExhausted returns true if too many failed attempts have been made.
func (p *PhoneOTP) IsExhausted() bool {
	return p.Attempts >= 5
}
`
}

func phoneOTPServiceGo() string {
	return `package phone

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"{{MODULE}}/internal/providers/sms"
)

// Sentinel errors returned by the OTP service.
var (
	ErrOTPExpired    = errors.New("phone otp: code has expired")
	ErrOTPInvalid    = errors.New("phone otp: code is incorrect")
	ErrOTPExhausted  = errors.New("phone otp: too many failed attempts")
	ErrRateLimited   = errors.New("phone otp: too many OTPs requested — try again later")
	ErrInvalidPhone  = errors.New("phone otp: invalid phone number format")
)

var e164Re = regexp.MustCompile(` + "`" + `^\+[1-9]\d{6,14}$` + "`" + `)

// OTPTTLMinutes is how long an OTP is valid for.
const OTPTTLMinutes = 10

// MaxOTPsPerWindow is the maximum OTPs that can be requested per phone per window.
const MaxOTPsPerWindow = 3

// Service handles phone OTP generation, delivery, and verification.
type Service struct {
	db          *gorm.DB
	smsProvider sms.SMSProvider
	appName     string // Used in SMS message text
}

// NewService creates a new phone OTP service.
//
//	appName: shown in the SMS, e.g. "MyApp" → "Your MyApp code is 123456"
func NewService(db *gorm.DB, smsProvider sms.SMSProvider, appName string) *Service {
	return &Service{db: db, smsProvider: smsProvider, appName: appName}
}

// RequestOTP generates a new OTP and sends it via SMS.
// Rate-limits to MaxOTPsPerWindow requests per phone per OTPTTLMinutes window.
func (s *Service) RequestOTP(ctx context.Context, phone, purpose string) error {
	if !e164Re.MatchString(phone) {
		return ErrInvalidPhone
	}

	// Rate-limit check: count recent OTPs for this phone
	var count int64
	window := time.Now().Add(-OTPTTLMinutes * time.Minute)
	s.db.Model(&PhoneOTP{}).
		Where("phone = ? AND purpose = ? AND created_at > ?", phone, purpose, window).
		Count(&count)
	if count >= MaxOTPsPerWindow {
		return ErrRateLimited
	}

	// Invalidate any existing unverified OTPs for this phone+purpose
	s.db.Where("phone = ? AND purpose = ? AND verified = false", phone, purpose).
		Delete(&PhoneOTP{})

	// Generate a 6-digit OTP
	code, err := generateOTP(6)
	if err != nil {
		return fmt.Errorf("phone otp: generate: %w", err)
	}

	// Hash the OTP before storing
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("phone otp: hash: %w", err)
	}

	otp := &PhoneOTP{
		Phone:     phone,
		CodeHash:  string(hash),
		Purpose:   purpose,
		ExpiresAt: time.Now().Add(OTPTTLMinutes * time.Minute),
	}
	if err := s.db.Create(otp).Error; err != nil {
		return fmt.Errorf("phone otp: save: %w", err)
	}

	// Send OTP via SMS
	msg := fmt.Sprintf("Your %s verification code is %s. Valid for %d minutes. Do not share this code.",
		s.appName, code, OTPTTLMinutes)
	if _, err := s.smsProvider.Send(ctx, sms.Message{
		To:   phone,
		Body: msg,
	}); err != nil {
		// Clean up the stored OTP if SMS fails
		s.db.Delete(otp)
		return fmt.Errorf("phone otp: send sms: %w", err)
	}

	return nil
}

// VerifyOTP checks the provided code against the stored OTP for the phone+purpose.
// Returns nil on success. On success, the OTP record is marked verified and deleted.
func (s *Service) VerifyOTP(ctx context.Context, phone, code, purpose string) error {
	if !e164Re.MatchString(phone) {
		return ErrInvalidPhone
	}

	var otp PhoneOTP
	err := s.db.Where("phone = ? AND purpose = ? AND verified = false", phone, purpose).
		Order("created_at DESC").
		First(&otp).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrOTPInvalid
	}
	if err != nil {
		return fmt.Errorf("phone otp: lookup: %w", err)
	}

	if otp.IsExpired() {
		s.db.Delete(&otp)
		return ErrOTPExpired
	}

	if otp.IsExhausted() {
		s.db.Delete(&otp)
		return ErrOTPExhausted
	}

	if err := bcrypt.CompareHashAndPassword([]byte(otp.CodeHash), []byte(code)); err != nil {
		// Increment attempts
		s.db.Model(&otp).Update("attempts", otp.Attempts+1)
		if otp.Attempts+1 >= 5 {
			s.db.Delete(&otp)
			return ErrOTPExhausted
		}
		return ErrOTPInvalid
	}

	// Success — mark as verified and delete
	s.db.Model(&otp).Update("verified", true)
	s.db.Delete(&otp)
	return nil
}

// CleanupExpired deletes all expired OTP records. Call from a cron job.
func (s *Service) CleanupExpired() (int64, error) {
	result := s.db.Where("expires_at < ? OR verified = true", time.Now()).Delete(&PhoneOTP{})
	return result.RowsAffected, result.Error
}

// generateOTP returns a cryptographically random numeric string of the given length.
func generateOTP(length int) (string, error) {
	digits := "0123456789"
	code := make([]byte, length)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		code[i] = digits[n.Int64()]
	}
	return string(code), nil
}
`
}

func phoneOTPHandlerGo() string {
	return `package phone

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	// Jua events — fire domain events for real-time + notifications
	"{{MODULE}}/jua/events"
)

// Handler provides HTTP endpoints for phone OTP authentication.
type Handler struct {
	svc      *Service
	jwtIssue func(userID uint, phone string) (string, error) // injected from your auth service
}

// NewHandler creates a phone OTP handler.
//
//	jwtIssue: a function that issues a JWT token given a user ID and phone number.
//	          Pass your existing auth service's IssueToken method.
func NewHandler(svc *Service, jwtIssue func(userID uint, phone string) (string, error)) *Handler {
	return &Handler{svc: svc, jwtIssue: jwtIssue}
}

// RegisterRoutes mounts phone OTP routes onto a Gin router group.
//
//	POST /auth/phone/request   — request an OTP
//	POST /auth/phone/verify    — verify OTP + get JWT
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/phone/request", h.RequestOTP)
	rg.POST("/phone/verify", h.VerifyOTP)
}

// RequestOTP godoc
// @Summary Request a phone OTP
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body requestOTPInput true "Phone number"
// @Success 200 {object} map[string]interface{}
// @Router /auth/phone/request [post]
func (h *Handler) RequestOTP(c *gin.Context) {
	var input struct {
		Phone   string ` + "`" + `json:"phone" binding:"required"` + "`" + `
		Purpose string ` + "`" + `json:"purpose"` + "`" + `
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()},
		})
		return
	}

	purpose := input.Purpose
	if purpose == "" {
		purpose = "login"
	}

	if err := h.svc.RequestOTP(c.Request.Context(), input.Phone, purpose); err != nil {
		code, errCode, msg := httpError(err)
		c.JSON(code, gin.H{"error": gin.H{"code": errCode, "message": msg}})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "OTP sent to " + maskPhone(input.Phone),
	})
}

// VerifyOTP godoc
// @Summary Verify a phone OTP and get a JWT
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body verifyOTPInput true "Phone + OTP code"
// @Success 200 {object} map[string]interface{}
// @Router /auth/phone/verify [post]
func (h *Handler) VerifyOTP(c *gin.Context) {
	var input struct {
		Phone   string ` + "`" + `json:"phone" binding:"required"` + "`" + `
		Code    string ` + "`" + `json:"code" binding:"required"` + "`" + `
		Purpose string ` + "`" + `json:"purpose"` + "`" + `
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()},
		})
		return
	}

	purpose := input.Purpose
	if purpose == "" {
		purpose = "login"
	}

	if err := h.svc.VerifyOTP(c.Request.Context(), input.Phone, input.Code, purpose); err != nil {
		code, errCode, msg := httpError(err)
		c.JSON(code, gin.H{"error": gin.H{"code": errCode, "message": msg}})
		return
	}

	// Jua events: fire login event for real-time + security notifications
	events.Publish(events.UserLoggedIn, "", map[string]interface{}{"phone": input.Phone})

	// OTP verified — issue JWT
	// TODO: look up or create the user by phone number in your users table,
	// then pass their ID to jwtIssue. For now we pass 0 as a placeholder.
	//
	// Example:
	//   user, _ := userSvc.FindOrCreateByPhone(input.Phone)
	//   token, _ := h.jwtIssue(user.ID, input.Phone)
	token := ""
	if h.jwtIssue != nil {
		var err error
		token, err = h.jwtIssue(0, input.Phone)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{"code": "TOKEN_ERROR", "message": "Failed to issue token"},
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":    gin.H{"token": token, "phone": input.Phone},
		"message": "Phone verified successfully",
	})
}

func httpError(err error) (int, string, string) {
	switch {
	case errors.Is(err, ErrOTPExpired):
		return http.StatusUnprocessableEntity, "OTP_EXPIRED", "This code has expired. Please request a new one."
	case errors.Is(err, ErrOTPInvalid):
		return http.StatusUnprocessableEntity, "OTP_INVALID", "Incorrect code. Please try again."
	case errors.Is(err, ErrOTPExhausted):
		return http.StatusTooManyRequests, "OTP_EXHAUSTED", "Too many failed attempts. Please request a new code."
	case errors.Is(err, ErrRateLimited):
		return http.StatusTooManyRequests, "RATE_LIMITED", "Too many OTP requests. Please wait before trying again."
	case errors.Is(err, ErrInvalidPhone):
		return http.StatusBadRequest, "INVALID_PHONE", "Phone number must be in E.164 format (e.g. +256771234567)."
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR", err.Error()
	}
}

// maskPhone hides the middle digits of a phone number for the response.
// +256771234567 → +256***4567
func maskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	return phone[:4] + "***" + phone[len(phone)-4:]
}
`
}

func phoneOTPMigrateGo() string {
	return `package phone

import (
	"fmt"

	"gorm.io/gorm"
)

// Migrate runs AutoMigrate for the PhoneOTP model.
// Call this from your database migration entrypoint.
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&PhoneOTP{}); err != nil {
		return fmt.Errorf("phone otp: auto migrate: %w", err)
	}
	return nil
}
`
}
