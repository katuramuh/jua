package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"
)

func writeJuaProvidersSMSFiles(root string, opts Options) error {
	apiRoot := opts.APIRoot(root)
	module := opts.Module()
	_ = module

	files := map[string]string{
		filepath.Join(apiRoot, "internal", "providers", "sms", "interface.go"):       smsInterfaceGo(),
		filepath.Join(apiRoot, "internal", "providers", "sms", "africastalking.go"):  smsAfricasTalkingGo(),
		filepath.Join(apiRoot, "internal", "providers", "sms", "relworx.go"):         smsRelworxGo(),
		filepath.Join(apiRoot, "internal", "providers", "sms", "esmsafrica.go"):      smsESMSAfricaGo(),
		filepath.Join(apiRoot, "internal", "providers", "sms", "twilio.go"):          smsTwilioGo(),
		filepath.Join(apiRoot, "internal", "providers", "sms", "vonage.go"):          smsVonageGo(),
		filepath.Join(apiRoot, "internal", "providers", "sms", "custom.go"):          smsCustomGo(),
		filepath.Join(apiRoot, "internal", "providers", "sms", "registry.go"):        smsRegistryGo(),
	}

	for path, content := range files {
		content = strings.ReplaceAll(content, "{{MODULE}}", module)
		if err := writeFile(path, content); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

func smsInterfaceGo() string {
	return `package sms

// SMSProvider defines the contract for all SMS providers.
// Add a new provider by implementing this interface and registering it in registry.go.
type SMSProvider interface {
	// Send sends a single SMS message to a phone number in E.164 format (e.g. +256771234567).
	Send(to string, message string) error

	// SendBulk sends the same message to multiple recipients.
	SendBulk(recipients []string, message string) error

	// SendOTP sends a one-time password SMS. The provider may use a pre-approved template.
	SendOTP(to string, otp string) error

	// GetDeliveryStatus returns the delivery status for a previously sent message ID.
	GetDeliveryStatus(messageID string) (string, error)
}
`
}

func smsAfricasTalkingGo() string {
	return `package sms

// TODO: Africa's Talking API docs — https://developers.africastalking.com/docs/sms/sending

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// AfricasTalkingProvider sends SMS via the Africa's Talking API.
type AfricasTalkingProvider struct {
	apiKey   string
	username string
	senderID string
	client   *http.Client
}

// NewAfricasTalkingProvider creates a provider using AT_API_KEY, AT_USERNAME, AT_SENDER_ID env vars.
func NewAfricasTalkingProvider() *AfricasTalkingProvider {
	return &AfricasTalkingProvider{
		apiKey:   os.Getenv("AFRICASTALKING_API_KEY"),
		username: os.Getenv("AFRICASTALKING_USERNAME"),
		senderID: os.Getenv("AFRICASTALKING_SENDER_ID"),
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *AfricasTalkingProvider) Send(to, message string) error {
	data := url.Values{}
	data.Set("username", p.username)
	data.Set("to", to)
	data.Set("message", message)
	if p.senderID != "" {
		data.Set("from", p.senderID)
	}

	req, err := http.NewRequest(http.MethodPost,
		"https://api.africastalking.com/version1/messaging",
		strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("africastalking: send: %w", err)
	}
	req.Header.Set("apiKey", p.apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("africastalking: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("africastalking: send: status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (p *AfricasTalkingProvider) SendBulk(recipients []string, message string) error {
	return p.Send(strings.Join(recipients, ","), message)
}

func (p *AfricasTalkingProvider) SendOTP(to, otp string) error {
	return p.Send(to, fmt.Sprintf("Your verification code is: %s. Valid for 5 minutes. Do not share.", otp))
}

func (p *AfricasTalkingProvider) GetDeliveryStatus(messageID string) (string, error) {
	// TODO: Africa's Talking does not expose a direct status endpoint — use delivery reports webhook instead.
	// See: https://developers.africastalking.com/docs/sms/notifications
	_ = messageID
	var result struct {
		SMSMessageData struct {
			Recipients []struct {
				Status string ` + "`" + `json:"status"` + "`" + `
			} ` + "`" + `json:"Recipients"` + "`" + `
		} ` + "`" + `json:"SMSMessageData"` + "`" + `
	}
	_ = json.Unmarshal(nil, &result)
	return "unknown", fmt.Errorf("africastalking: delivery status not available via polling — use webhook")
}
`
}

func smsRelworxGo() string {
	return `package sms

// TODO: Relworx API docs — https://api.relworx.com/docs

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// RelworxProvider sends SMS via the Relworx API.
type RelworxProvider struct {
	apiKey   string
	senderID string
	client   *http.Client
}

// NewRelworxProvider creates a provider using RELWORX_API_KEY and RELWORX_SENDER_ID env vars.
func NewRelworxProvider() *RelworxProvider {
	return &RelworxProvider{
		apiKey:   os.Getenv("RELWORX_API_KEY"),
		senderID: os.Getenv("RELWORX_SENDER_ID"),
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *RelworxProvider) Send(to, message string) error {
	payload := fmt.Sprintf(` + "`" + `{"recipient":"%s","message":"%s","sender_id":"%s"}` + "`" + `,
		to, message, p.senderID)

	req, err := http.NewRequest(http.MethodPost,
		"https://api.relworx.com/sms/send",
		strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("relworx: send: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("relworx: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("relworx: send: status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (p *RelworxProvider) SendBulk(recipients []string, message string) error {
	for _, r := range recipients {
		if err := p.Send(r, message); err != nil {
			return err
		}
	}
	return nil
}

func (p *RelworxProvider) SendOTP(to, otp string) error {
	return p.Send(to, fmt.Sprintf("Your verification code is: %s. Valid for 5 minutes.", otp))
}

func (p *RelworxProvider) GetDeliveryStatus(messageID string) (string, error) {
	// TODO: See Relworx delivery report docs
	_ = messageID
	return "unknown", nil
}
`
}

func smsESMSAfricaGo() string {
	return `package sms

// TODO: eSMS Africa API docs — https://esmsafrica.io/api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ESMSAfricaProvider sends SMS via the eSMS Africa API.
type ESMSAfricaProvider struct {
	apiKey   string
	senderID string
	client   *http.Client
}

// NewESMSAfricaProvider creates a provider using ESMS_API_KEY and ESMS_SENDER_ID env vars.
func NewESMSAfricaProvider() *ESMSAfricaProvider {
	return &ESMSAfricaProvider{
		apiKey:   os.Getenv("ESMS_API_KEY"),
		senderID: os.Getenv("ESMS_SENDER_ID"),
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *ESMSAfricaProvider) Send(to, message string) error {
	payload := fmt.Sprintf(` + "`" + `{"to":"%s","message":"%s","sender":"%s"}` + "`" + `, to, message, p.senderID)

	req, err := http.NewRequest(http.MethodPost,
		"https://api.esmsafrica.io/v1/sms/send",
		strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("esmsafrica: send: %w", err)
	}
	req.Header.Set("X-API-Key", p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("esmsafrica: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("esmsafrica: send: status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (p *ESMSAfricaProvider) SendBulk(recipients []string, message string) error {
	for _, r := range recipients {
		if err := p.Send(r, message); err != nil {
			return err
		}
	}
	return nil
}

func (p *ESMSAfricaProvider) SendOTP(to, otp string) error {
	return p.Send(to, fmt.Sprintf("Your OTP is: %s. Valid for 5 minutes.", otp))
}

func (p *ESMSAfricaProvider) GetDeliveryStatus(messageID string) (string, error) {
	_ = messageID
	return "unknown", nil
}
`
}

func smsTwilioGo() string {
	return `package sms

// TODO: Twilio SMS API docs — https://www.twilio.com/docs/sms/api

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// TwilioProvider sends SMS via the Twilio Messaging API.
type TwilioProvider struct {
	accountSID string
	authToken  string
	from       string
	client     *http.Client
}

// NewTwilioProvider creates a provider using TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN, TWILIO_FROM_NUMBER env vars.
func NewTwilioProvider() *TwilioProvider {
	return &TwilioProvider{
		accountSID: os.Getenv("TWILIO_ACCOUNT_SID"),
		authToken:  os.Getenv("TWILIO_AUTH_TOKEN"),
		from:       os.Getenv("TWILIO_FROM_NUMBER"),
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *TwilioProvider) Send(to, message string) error {
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", p.accountSID)

	data := url.Values{}
	data.Set("To", to)
	data.Set("From", p.from)
	data.Set("Body", message)

	req, err := http.NewRequest(http.MethodPost, apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("twilio: send: %w", err)
	}
	req.SetBasicAuth(p.accountSID, p.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("twilio: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("twilio: send: status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (p *TwilioProvider) SendBulk(recipients []string, message string) error {
	for _, r := range recipients {
		if err := p.Send(r, message); err != nil {
			return err
		}
	}
	return nil
}

func (p *TwilioProvider) SendOTP(to, otp string) error {
	return p.Send(to, fmt.Sprintf("Your verification code is: %s. Valid for 5 minutes.", otp))
}

func (p *TwilioProvider) GetDeliveryStatus(messageID string) (string, error) {
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages/%s.json",
		p.accountSID, messageID)

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("twilio: delivery_status: %w", err)
	}
	req.SetBasicAuth(p.accountSID, p.authToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("twilio: delivery_status: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Status string ` + "`" + `json:"status"` + "`" + `
	}
	if err := decodeJSON(resp.Body, &result); err != nil {
		return "", fmt.Errorf("twilio: delivery_status: %w", err)
	}
	return result.Status, nil
}
`
}

func smsVonageGo() string {
	return `package sms

// TODO: Vonage SMS API docs — https://developer.vonage.com/api/sms

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// VonageProvider sends SMS via the Vonage (formerly Nexmo) SMS API.
type VonageProvider struct {
	apiKey    string
	apiSecret string
	from      string
	client    *http.Client
}

// NewVonageProvider creates a provider using VONAGE_API_KEY, VONAGE_API_SECRET, VONAGE_FROM env vars.
func NewVonageProvider() *VonageProvider {
	return &VonageProvider{
		apiKey:    os.Getenv("VONAGE_API_KEY"),
		apiSecret: os.Getenv("VONAGE_API_SECRET"),
		from:      os.Getenv("VONAGE_FROM"),
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *VonageProvider) Send(to, message string) error {
	payload := fmt.Sprintf(
		` + "`" + `{"api_key":"%s","api_secret":"%s","to":"%s","from":"%s","text":"%s"}` + "`" + `,
		p.apiKey, p.apiSecret, to, p.from, message)

	req, err := http.NewRequest(http.MethodPost,
		"https://rest.nexmo.com/sms/json",
		strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("vonage: send: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("vonage: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("vonage: send: status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (p *VonageProvider) SendBulk(recipients []string, message string) error {
	for _, r := range recipients {
		if err := p.Send(r, message); err != nil {
			return err
		}
	}
	return nil
}

func (p *VonageProvider) SendOTP(to, otp string) error {
	return p.Send(to, fmt.Sprintf("Your verification code is: %s. Valid for 5 minutes.", otp))
}

func (p *VonageProvider) GetDeliveryStatus(messageID string) (string, error) {
	_ = messageID
	// TODO: Vonage DLR via webhook — https://developer.vonage.com/messaging/sms/guides/delivery-receipts
	return "unknown", nil
}
`
}

func smsCustomGo() string {
	return `package sms

// CustomProvider sends SMS via any provider that accepts a generic HTTP POST webhook.
// Configure the URL and API key via CUSTOM_SMS_WEBHOOK_URL and CUSTOM_SMS_API_KEY.

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// CustomProvider sends SMS via a configurable HTTP webhook endpoint.
type CustomProvider struct {
	webhookURL string
	apiKey     string
	client     *http.Client
}

// NewCustomProvider creates a provider using CUSTOM_SMS_WEBHOOK_URL and CUSTOM_SMS_API_KEY env vars.
func NewCustomProvider() *CustomProvider {
	return &CustomProvider{
		webhookURL: os.Getenv("CUSTOM_SMS_WEBHOOK_URL"),
		apiKey:     os.Getenv("CUSTOM_SMS_API_KEY"),
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *CustomProvider) Send(to, message string) error {
	payload := fmt.Sprintf(` + "`" + `{"to":"%s","message":"%s"}` + "`" + `, to, message)

	req, err := http.NewRequest(http.MethodPost, p.webhookURL, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("custom_sms: send: %w", err)
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("custom_sms: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("custom_sms: send: status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (p *CustomProvider) SendBulk(recipients []string, message string) error {
	for _, r := range recipients {
		if err := p.Send(r, message); err != nil {
			return err
		}
	}
	return nil
}

func (p *CustomProvider) SendOTP(to, otp string) error {
	return p.Send(to, fmt.Sprintf("Your OTP is: %s. Valid for 5 minutes.", otp))
}

func (p *CustomProvider) GetDeliveryStatus(messageID string) (string, error) {
	_ = messageID
	return "unknown", nil
}
`
}

func smsRegistryGo() string {
	return `package sms

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// New returns the SMS provider configured via the SMS_PROVIDER environment variable.
// Valid values: africastalking, relworx, esmsafrica, twilio, vonage, custom.
func New() (SMSProvider, error) {
	provider := os.Getenv("SMS_PROVIDER")
	switch provider {
	case "africastalking":
		return NewAfricasTalkingProvider(), nil
	case "relworx":
		return NewRelworxProvider(), nil
	case "esmsafrica":
		return NewESMSAfricaProvider(), nil
	case "twilio":
		return NewTwilioProvider(), nil
	case "vonage":
		return NewVonageProvider(), nil
	case "custom":
		return NewCustomProvider(), nil
	case "":
		return nil, fmt.Errorf("sms: SMS_PROVIDER is not set")
	default:
		return nil, fmt.Errorf("sms: unknown provider %q (valid: africastalking, relworx, esmsafrica, twilio, vonage, custom)", provider)
	}
}

// decodeJSON is a shared helper for decoding HTTP response bodies.
func decodeJSON(r io.Reader, v interface{}) error {
	return json.NewDecoder(r).Decode(v)
}
`
}
