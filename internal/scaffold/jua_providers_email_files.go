package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"
)

func writeJuaProvidersEmailFiles(root string, opts Options) error {
	apiRoot := opts.APIRoot(root)
	module := opts.Module()
	_ = module

	files := map[string]string{
		filepath.Join(apiRoot, "internal", "providers", "email", "interface.go"): emailInterfaceGo(),
		filepath.Join(apiRoot, "internal", "providers", "email", "resend.go"):    emailResendGo(),
		filepath.Join(apiRoot, "internal", "providers", "email", "smtp.go"):      emailSMTPGo(),
		filepath.Join(apiRoot, "internal", "providers", "email", "custom.go"):    emailCustomGo(),
		filepath.Join(apiRoot, "internal", "providers", "email", "registry.go"):  emailRegistryGo(),
	}

	for path, content := range files {
		content = strings.ReplaceAll(content, "{{MODULE}}", module)
		if err := writeFile(path, content); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

func emailInterfaceGo() string {
	return `package email

import "context"

// Attachment holds a file attachment for an email.
type Attachment struct {
	Filename    string
	ContentType string
	Content     []byte
}

// EmailMessage holds the parameters for sending an email.
type EmailMessage struct {
	From        string // "Name <email@example.com>" or just "email@example.com"
	To          []string
	CC          []string
	BCC         []string
	ReplyTo     string
	Subject     string
	HTML        string
	Text        string
	Attachments []Attachment
	Tags        map[string]string
}

// EmailResponse is returned after sending an email.
type EmailResponse struct {
	MessageID string
	Status    string
	Raw       map[string]interface{}
}

// EmailProvider defines the contract for all email providers.
type EmailProvider interface {
	// Send sends an email message.
	Send(ctx context.Context, msg EmailMessage) (*EmailResponse, error)
	// SendBatch sends multiple emails in one API call (if supported).
	// Falls back to individual Send calls if batch is not natively supported.
	SendBatch(ctx context.Context, msgs []EmailMessage) ([]*EmailResponse, error)
}
`
}

func emailResendGo() string {
	return `package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Resend implements EmailProvider using the Resend API.
// TODO: Docs — https://resend.com/docs/api-reference/emails
type Resend struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewResend creates a Resend email provider from environment variables.
// Required: RESEND_API_KEY
func NewResend() *Resend {
	return &Resend{
		baseURL: "https://api.resend.com",
		apiKey:  os.Getenv("RESEND_API_KEY"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (r *Resend) Send(ctx context.Context, msg EmailMessage) (*EmailResponse, error) {
	payload := map[string]interface{}{
		"from":    msg.From,
		"to":      msg.To,
		"subject": msg.Subject,
	}
	if msg.HTML != "" {
		payload["html"] = msg.HTML
	}
	if msg.Text != "" {
		payload["text"] = msg.Text
	}
	if len(msg.CC) > 0 {
		payload["cc"] = msg.CC
	}
	if len(msg.BCC) > 0 {
		payload["bcc"] = msg.BCC
	}
	if msg.ReplyTo != "" {
		payload["reply_to"] = msg.ReplyTo
	}
	if len(msg.Tags) > 0 {
		tags := make([]map[string]string, 0, len(msg.Tags))
		for k, v := range msg.Tags {
			tags = append(tags, map[string]string{"name": k, "value": v})
		}
		payload["tags"] = tags
	}
	if len(msg.Attachments) > 0 {
		attachments := make([]map[string]interface{}, len(msg.Attachments))
		for i, a := range msg.Attachments {
			attachments[i] = map[string]interface{}{
				"filename":     a.Filename,
				"content":      a.Content,
				"content_type": a.ContentType,
			}
		}
		payload["attachments"] = attachments
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("resend http: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("resend: %v — %v", result["name"], result["message"])
	}

	id, _ := result["id"].(string)
	return &EmailResponse{MessageID: id, Status: "sent", Raw: result}, nil
}

func (r *Resend) SendBatch(ctx context.Context, msgs []EmailMessage) ([]*EmailResponse, error) {
	responses := make([]*EmailResponse, 0, len(msgs))
	for _, msg := range msgs {
		resp, err := r.Send(ctx, msg)
		if err != nil {
			return responses, err
		}
		responses = append(responses, resp)
	}
	return responses, nil
}
`
}

func emailSMTPGo() string {
	return `package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"os"
	"strconv"
	"strings"
)

// SMTP implements EmailProvider using standard SMTP.
// Works with Mailhog (dev), Gmail, SendGrid, Mailgun, Postmark, and any SMTP server.
type SMTP struct {
	host     string
	port     int
	username string
	password string
	useTLS   bool
}

// NewSMTP creates an SMTP email provider from environment variables.
// Required: SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD
// Optional: SMTP_TLS (default: "true")
func NewSMTP() *SMTP {
	port, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if port == 0 {
		port = 587
	}
	useTLS := os.Getenv("SMTP_TLS") != "false"
	return &SMTP{
		host:     os.Getenv("SMTP_HOST"),
		port:     port,
		username: os.Getenv("SMTP_USERNAME"),
		password: os.Getenv("SMTP_PASSWORD"),
		useTLS:   useTLS,
	}
}

func (s *SMTP) Send(ctx context.Context, msg EmailMessage) (*EmailResponse, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Build MIME message
	headers := make(textproto.MIMEHeader)
	headers.Set("From", msg.From)
	headers.Set("To", strings.Join(msg.To, ", "))
	headers.Set("Subject", msg.Subject)
	headers.Set("MIME-Version", "1.0")
	headers.Set("Content-Type", "multipart/mixed; boundary="+writer.Boundary())

	if len(msg.CC) > 0 {
		headers.Set("Cc", strings.Join(msg.CC, ", "))
	}
	if msg.ReplyTo != "" {
		headers.Set("Reply-To", msg.ReplyTo)
	}

	var headerBuf bytes.Buffer
	for k, vs := range headers {
		for _, v := range vs {
			headerBuf.WriteString(k + ": " + v + "\r\n")
		}
	}
	headerBuf.WriteString("\r\n")

	// Add HTML part
	if msg.HTML != "" {
		part, _ := writer.CreatePart(textproto.MIMEHeader{
			"Content-Type": {"text/html; charset=utf-8"},
		})
		part.Write([]byte(msg.HTML))
	}

	// Add text part
	if msg.Text != "" {
		part, _ := writer.CreatePart(textproto.MIMEHeader{
			"Content-Type": {"text/plain; charset=utf-8"},
		})
		part.Write([]byte(msg.Text))
	}

	// Add attachments
	for _, att := range msg.Attachments {
		part, _ := writer.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {att.ContentType},
			"Content-Disposition":       {"attachment; filename=\"" + att.Filename + "\""},
			"Content-Transfer-Encoding": {"base64"},
		})
		part.Write(att.Content)
	}

	writer.Close()

	addr := net.JoinHostPort(s.host, fmt.Sprintf("%d", s.port))
	auth := smtp.PlainAuth("", s.username, s.password, s.host)

	var sendErr error
	if s.useTLS {
		tlsConfig := &tls.Config{ServerName: s.host}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("smtp tls dial: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, s.host)
		if err != nil {
			return nil, fmt.Errorf("smtp new client: %w", err)
		}
		defer client.Quit()

		if err := client.Auth(auth); err != nil {
			return nil, fmt.Errorf("smtp auth: %w", err)
		}
		if err := client.Mail(msg.From); err != nil {
			return nil, fmt.Errorf("smtp mail from: %w", err)
		}
		for _, to := range msg.To {
			if err := client.Rcpt(to); err != nil {
				return nil, fmt.Errorf("smtp rcpt to %s: %w", to, err)
			}
		}
		w, err := client.Data()
		if err != nil {
			return nil, fmt.Errorf("smtp data: %w", err)
		}
		defer w.Close()

		w.Write(headerBuf.Bytes())
		w.Write(buf.Bytes())
	} else {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("smtp dial: %w", err)
		}
		defer conn.Close()

		message := headerBuf.String() + buf.String()
		sendErr = smtp.SendMail(addr, auth, msg.From, msg.To, []byte(message))
	}

	if sendErr != nil {
		return nil, fmt.Errorf("smtp send: %w", sendErr)
	}

	return &EmailResponse{Status: "sent"}, nil
}

func (s *SMTP) SendBatch(ctx context.Context, msgs []EmailMessage) ([]*EmailResponse, error) {
	responses := make([]*EmailResponse, 0, len(msgs))
	for _, msg := range msgs {
		resp, err := s.Send(ctx, msg)
		if err != nil {
			return responses, err
		}
		responses = append(responses, resp)
	}
	return responses, nil
}
`
}

func emailCustomGo() string {
	return `package email

import (
	"context"
	"fmt"
)

// CustomEmail is a stub EmailProvider for custom implementations.
type CustomEmail struct{ name string }

// NewCustomEmail creates a custom email provider stub.
func NewCustomEmail(name string) *CustomEmail { return &CustomEmail{name: name} }

func (c *CustomEmail) Send(ctx context.Context, msg EmailMessage) (*EmailResponse, error) {
	return nil, fmt.Errorf("custom email [%s]: Send not implemented", c.name)
}

func (c *CustomEmail) SendBatch(ctx context.Context, msgs []EmailMessage) ([]*EmailResponse, error) {
	return nil, fmt.Errorf("custom email [%s]: SendBatch not implemented", c.name)
}
`
}

func emailRegistryGo() string {
	return `package email

import (
	"fmt"
	"os"
	"strings"
)

// Registry returns the configured EmailProvider based on EMAIL_PROVIDER env var.
// Supported values: "resend", "smtp", "custom"
// Defaults to "resend" if unset.
func Registry() (EmailProvider, error) {
	provider := strings.ToLower(os.Getenv("EMAIL_PROVIDER"))
	if provider == "" {
		provider = "resend"
	}

	switch provider {
	case "resend":
		if os.Getenv("RESEND_API_KEY") == "" {
			return nil, fmt.Errorf("email registry: EMAIL_PROVIDER=resend but RESEND_API_KEY is not set")
		}
		return NewResend(), nil

	case "smtp":
		if os.Getenv("SMTP_HOST") == "" {
			return nil, fmt.Errorf("email registry: EMAIL_PROVIDER=smtp but SMTP_HOST is not set")
		}
		return NewSMTP(), nil

	case "custom":
		name := os.Getenv("CUSTOM_EMAIL_NAME")
		if name == "" {
			name = "custom"
		}
		return NewCustomEmail(name), nil

	default:
		return nil, fmt.Errorf("email registry: unknown EMAIL_PROVIDER %q (supported: resend, smtp, custom)", provider)
	}
}
`
}
