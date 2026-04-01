package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"
)

func writeJuaProvidersWhatsAppFiles(root string, opts Options) error {
	apiRoot := opts.APIRoot(root)
	module := opts.Module()
	_ = module

	files := map[string]string{
		filepath.Join(apiRoot, "internal", "providers", "whatsapp", "interface.go"):       whatsappInterfaceGo(),
		filepath.Join(apiRoot, "internal", "providers", "whatsapp", "africastalking.go"):  whatsappAfricasTalkingGo(),
		filepath.Join(apiRoot, "internal", "providers", "whatsapp", "twilio.go"):          whatsappTwilioGo(),
		filepath.Join(apiRoot, "internal", "providers", "whatsapp", "meta.go"):            whatsappMetaGo(),
		filepath.Join(apiRoot, "internal", "providers", "whatsapp", "custom.go"):          whatsappCustomGo(),
		filepath.Join(apiRoot, "internal", "providers", "whatsapp", "registry.go"):        whatsappRegistryGo(),
	}

	for path, content := range files {
		content = strings.ReplaceAll(content, "{{MODULE}}", module)
		if err := writeFile(path, content); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

func whatsappInterfaceGo() string {
	return `package whatsapp

import "context"

// TextMessage holds content for a plain-text WhatsApp message.
type TextMessage struct {
	To   string // E.164 phone number
	Body string
}

// TemplateMessage holds content for a WhatsApp template (approved HSM) message.
type TemplateMessage struct {
	To           string
	TemplateName string
	LanguageCode string // e.g. "en_US"
	Parameters   []string
}

// MediaMessage holds content for a WhatsApp media message.
type MediaMessage struct {
	To       string
	Type     string // "image", "document", "audio", "video"
	URL      string
	Caption  string
	Filename string // for documents
}

// MessageResponse is returned after sending a WhatsApp message.
type MessageResponse struct {
	MessageID string
	Status    string // "queued", "sent", "delivered", "failed"
	Raw       map[string]interface{}
}

// WhatsAppProvider defines the contract for all WhatsApp messaging providers.
type WhatsAppProvider interface {
	// SendText sends a plain-text message.
	SendText(ctx context.Context, msg TextMessage) (*MessageResponse, error)
	// SendTemplate sends an approved template (HSM) message.
	SendTemplate(ctx context.Context, msg TemplateMessage) (*MessageResponse, error)
	// SendMedia sends a media message (image, document, audio, video).
	SendMedia(ctx context.Context, msg MediaMessage) (*MessageResponse, error)
	// GetStatus retrieves the delivery status of a sent message.
	GetStatus(ctx context.Context, messageID string) (*MessageResponse, error)
}
`
}

func whatsappAfricasTalkingGo() string {
	return `package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// AfricasTalkingWhatsApp implements WhatsAppProvider via Africa's Talking.
// TODO: Docs — https://developers.africastalking.com/docs/whatsapp
type AfricasTalkingWhatsApp struct {
	baseURL  string
	apiKey   string
	username string
	from     string
	client   *http.Client
}

// NewAfricasTalkingWhatsApp creates an Africa's Talking WhatsApp provider.
// Required env vars: AT_API_KEY, AT_USERNAME, AT_WHATSAPP_FROM
func NewAfricasTalkingWhatsApp() *AfricasTalkingWhatsApp {
	return &AfricasTalkingWhatsApp{
		baseURL:  "https://api.africastalking.com/whatsapp",
		apiKey:   os.Getenv("AT_API_KEY"),
		username: os.Getenv("AT_USERNAME"),
		from:     os.Getenv("AT_WHATSAPP_FROM"),
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *AfricasTalkingWhatsApp) post(ctx context.Context, path string, payload interface{}) (map[string]interface{}, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("apiKey", a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("at whatsapp http: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

func (a *AfricasTalkingWhatsApp) SendText(ctx context.Context, msg TextMessage) (*MessageResponse, error) {
	result, err := a.post(ctx, "/message", map[string]interface{}{
		"username": a.username,
		"from":     a.from,
		"to":       msg.To,
		"message":  msg.Body,
	})
	if err != nil {
		return nil, err
	}
	return &MessageResponse{Status: "queued", Raw: result}, nil
}

func (a *AfricasTalkingWhatsApp) SendTemplate(ctx context.Context, msg TemplateMessage) (*MessageResponse, error) {
	components := make([]map[string]interface{}, 0)
	if len(msg.Parameters) > 0 {
		params := make([]map[string]string, len(msg.Parameters))
		for i, p := range msg.Parameters {
			params[i] = map[string]string{"type": "text", "text": p}
		}
		components = append(components, map[string]interface{}{
			"type":       "body",
			"parameters": params,
		})
	}
	result, err := a.post(ctx, "/template", map[string]interface{}{
		"username": a.username,
		"from":     a.from,
		"to":       msg.To,
		"template": map[string]interface{}{
			"name":       msg.TemplateName,
			"language":   map[string]string{"code": msg.LanguageCode},
			"components": components,
		},
	})
	if err != nil {
		return nil, err
	}
	return &MessageResponse{Status: "queued", Raw: result}, nil
}

func (a *AfricasTalkingWhatsApp) SendMedia(ctx context.Context, msg MediaMessage) (*MessageResponse, error) {
	result, err := a.post(ctx, "/media", map[string]interface{}{
		"username": a.username,
		"from":     a.from,
		"to":       msg.To,
		"type":     msg.Type,
		"url":      msg.URL,
		"caption":  msg.Caption,
	})
	if err != nil {
		return nil, err
	}
	return &MessageResponse{Status: "queued", Raw: result}, nil
}

func (a *AfricasTalkingWhatsApp) GetStatus(ctx context.Context, messageID string) (*MessageResponse, error) {
	return &MessageResponse{MessageID: messageID, Status: "unknown"}, nil
}
`
}

func whatsappTwilioGo() string {
	return `package whatsapp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"encoding/json"
)

// TwilioWhatsApp implements WhatsAppProvider via Twilio WhatsApp Business API.
// TODO: Docs — https://www.twilio.com/docs/whatsapp/api
type TwilioWhatsApp struct {
	accountSID string
	authToken  string
	from       string // whatsapp:+14155238886
	client     *http.Client
}

// NewTwilioWhatsApp creates a Twilio WhatsApp provider.
// Required env vars: TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN, TWILIO_WHATSAPP_FROM
func NewTwilioWhatsApp() *TwilioWhatsApp {
	return &TwilioWhatsApp{
		accountSID: os.Getenv("TWILIO_ACCOUNT_SID"),
		authToken:  os.Getenv("TWILIO_AUTH_TOKEN"),
		from:       os.Getenv("TWILIO_WHATSAPP_FROM"),
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *TwilioWhatsApp) send(ctx context.Context, to, body, mediaURL string) (*MessageResponse, error) {
	params := url.Values{
		"From": {t.from},
		"To":   {"whatsapp:" + to},
		"Body": {body},
	}
	if mediaURL != "" {
		params.Set("MediaUrl", mediaURL)
	}

	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", t.accountSID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(t.accountSID, t.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twilio whatsapp http: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("twilio whatsapp: %v", result["message"])
	}

	sid, _ := result["sid"].(string)
	status, _ := result["status"].(string)
	return &MessageResponse{MessageID: sid, Status: status, Raw: result}, nil
}

func (t *TwilioWhatsApp) SendText(ctx context.Context, msg TextMessage) (*MessageResponse, error) {
	return t.send(ctx, msg.To, msg.Body, "")
}

func (t *TwilioWhatsApp) SendTemplate(ctx context.Context, msg TemplateMessage) (*MessageResponse, error) {
	// Twilio uses Content Templates for approved messages
	body := msg.TemplateName
	if len(msg.Parameters) > 0 {
		body = fmt.Sprintf("%s: %s", msg.TemplateName, strings.Join(msg.Parameters, ", "))
	}
	return t.send(ctx, msg.To, body, "")
}

func (t *TwilioWhatsApp) SendMedia(ctx context.Context, msg MediaMessage) (*MessageResponse, error) {
	return t.send(ctx, msg.To, msg.Caption, msg.URL)
}

func (t *TwilioWhatsApp) GetStatus(ctx context.Context, messageID string) (*MessageResponse, error) {
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages/%s.json",
		t.accountSID, messageID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(t.accountSID, t.authToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twilio whatsapp status: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	status, _ := result["status"].(string)
	return &MessageResponse{MessageID: messageID, Status: status, Raw: result}, nil
}
`
}

func whatsappMetaGo() string {
	return `package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// MetaWhatsApp implements WhatsAppProvider via Meta (Facebook) WhatsApp Cloud API.
// TODO: Docs — https://developers.facebook.com/docs/whatsapp/cloud-api
type MetaWhatsApp struct {
	baseURL     string
	accessToken string
	phoneID     string // WhatsApp Business Phone Number ID
	client      *http.Client
}

// NewMetaWhatsApp creates a Meta WhatsApp Cloud API provider.
// Required env vars: META_WHATSAPP_ACCESS_TOKEN, META_WHATSAPP_PHONE_ID
func NewMetaWhatsApp() *MetaWhatsApp {
	return &MetaWhatsApp{
		baseURL:     "https://graph.facebook.com/v18.0",
		accessToken: os.Getenv("META_WHATSAPP_ACCESS_TOKEN"),
		phoneID:     os.Getenv("META_WHATSAPP_PHONE_ID"),
		client:      &http.Client{Timeout: 30 * time.Second},
	}
}

func (m *MetaWhatsApp) post(ctx context.Context, payload interface{}) (map[string]interface{}, error) {
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/%s/messages", m.baseURL, m.phoneID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+m.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("meta whatsapp http: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if errObj, ok := result["error"].(map[string]interface{}); ok {
		return nil, fmt.Errorf("meta whatsapp: %v", errObj["message"])
	}
	return result, nil
}

func (m *MetaWhatsApp) SendText(ctx context.Context, msg TextMessage) (*MessageResponse, error) {
	result, err := m.post(ctx, map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                msg.To,
		"type":              "text",
		"text":              map[string]string{"body": msg.Body},
	})
	if err != nil {
		return nil, err
	}

	msgID := ""
	if messages, ok := result["messages"].([]interface{}); ok && len(messages) > 0 {
		if first, ok := messages[0].(map[string]interface{}); ok {
			msgID, _ = first["id"].(string)
		}
	}
	return &MessageResponse{MessageID: msgID, Status: "queued", Raw: result}, nil
}

func (m *MetaWhatsApp) SendTemplate(ctx context.Context, msg TemplateMessage) (*MessageResponse, error) {
	components := []map[string]interface{}{}
	if len(msg.Parameters) > 0 {
		params := make([]map[string]string, len(msg.Parameters))
		for i, p := range msg.Parameters {
			params[i] = map[string]string{"type": "text", "text": p}
		}
		components = append(components, map[string]interface{}{
			"type":       "body",
			"parameters": params,
		})
	}

	result, err := m.post(ctx, map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                msg.To,
		"type":              "template",
		"template": map[string]interface{}{
			"name":       msg.TemplateName,
			"language":   map[string]string{"code": msg.LanguageCode},
			"components": components,
		},
	})
	if err != nil {
		return nil, err
	}

	msgID := ""
	if messages, ok := result["messages"].([]interface{}); ok && len(messages) > 0 {
		if first, ok := messages[0].(map[string]interface{}); ok {
			msgID, _ = first["id"].(string)
		}
	}
	return &MessageResponse{MessageID: msgID, Status: "queued", Raw: result}, nil
}

func (m *MetaWhatsApp) SendMedia(ctx context.Context, msg MediaMessage) (*MessageResponse, error) {
	mediaObj := map[string]string{"link": msg.URL}
	if msg.Caption != "" {
		mediaObj["caption"] = msg.Caption
	}
	if msg.Filename != "" {
		mediaObj["filename"] = msg.Filename
	}

	result, err := m.post(ctx, map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                msg.To,
		"type":              msg.Type,
		msg.Type:            mediaObj,
	})
	if err != nil {
		return nil, err
	}

	msgID := ""
	if messages, ok := result["messages"].([]interface{}); ok && len(messages) > 0 {
		if first, ok := messages[0].(map[string]interface{}); ok {
			msgID, _ = first["id"].(string)
		}
	}
	return &MessageResponse{MessageID: msgID, Status: "queued", Raw: result}, nil
}

func (m *MetaWhatsApp) GetStatus(ctx context.Context, messageID string) (*MessageResponse, error) {
	// Meta doesn't have a direct status check endpoint — status comes via webhooks.
	return &MessageResponse{MessageID: messageID, Status: "unknown"}, nil
}
`
}

func whatsappCustomGo() string {
	return `package whatsapp

import (
	"context"
	"fmt"
)

// CustomWhatsApp is a stub WhatsAppProvider for custom implementations.
type CustomWhatsApp struct{ name string }

// NewCustomWhatsApp creates a custom WhatsApp provider stub.
func NewCustomWhatsApp(name string) *CustomWhatsApp { return &CustomWhatsApp{name: name} }

func (c *CustomWhatsApp) SendText(ctx context.Context, msg TextMessage) (*MessageResponse, error) {
	return nil, fmt.Errorf("custom whatsapp [%s]: SendText not implemented", c.name)
}

func (c *CustomWhatsApp) SendTemplate(ctx context.Context, msg TemplateMessage) (*MessageResponse, error) {
	return nil, fmt.Errorf("custom whatsapp [%s]: SendTemplate not implemented", c.name)
}

func (c *CustomWhatsApp) SendMedia(ctx context.Context, msg MediaMessage) (*MessageResponse, error) {
	return nil, fmt.Errorf("custom whatsapp [%s]: SendMedia not implemented", c.name)
}

func (c *CustomWhatsApp) GetStatus(ctx context.Context, messageID string) (*MessageResponse, error) {
	return nil, fmt.Errorf("custom whatsapp [%s]: GetStatus not implemented", c.name)
}
`
}

func whatsappRegistryGo() string {
	return `package whatsapp

import (
	"fmt"
	"os"
	"strings"
)

// Registry returns the configured WhatsAppProvider based on WHATSAPP_PROVIDER env var.
// Supported values: "africastalking", "at", "twilio", "meta", "custom"
// Defaults to "meta" if unset.
func Registry() (WhatsAppProvider, error) {
	provider := strings.ToLower(os.Getenv("WHATSAPP_PROVIDER"))
	if provider == "" {
		provider = "meta"
	}

	switch provider {
	case "africastalking", "at":
		if os.Getenv("AT_API_KEY") == "" {
			return nil, fmt.Errorf("whatsapp registry: WHATSAPP_PROVIDER=africastalking but AT_API_KEY is not set")
		}
		return NewAfricasTalkingWhatsApp(), nil

	case "twilio":
		if os.Getenv("TWILIO_ACCOUNT_SID") == "" {
			return nil, fmt.Errorf("whatsapp registry: WHATSAPP_PROVIDER=twilio but TWILIO_ACCOUNT_SID is not set")
		}
		return NewTwilioWhatsApp(), nil

	case "meta":
		if os.Getenv("META_WHATSAPP_ACCESS_TOKEN") == "" {
			return nil, fmt.Errorf("whatsapp registry: WHATSAPP_PROVIDER=meta but META_WHATSAPP_ACCESS_TOKEN is not set")
		}
		return NewMetaWhatsApp(), nil

	case "custom":
		name := os.Getenv("CUSTOM_WHATSAPP_NAME")
		if name == "" {
			name = "custom"
		}
		return NewCustomWhatsApp(name), nil

	default:
		return nil, fmt.Errorf("whatsapp registry: unknown WHATSAPP_PROVIDER %q (supported: africastalking, twilio, meta, custom)", provider)
	}
}
`
}
