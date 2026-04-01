package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"
)

func writeJuaProvidersUSSDFiles(root string, opts Options) error {
	apiRoot := opts.APIRoot(root)
	module := opts.Module()
	_ = module

	files := map[string]string{
		filepath.Join(apiRoot, "internal", "providers", "ussd", "interface.go"):      ussdInterfaceGo(),
		filepath.Join(apiRoot, "internal", "providers", "ussd", "africastalking.go"): ussdAfricasTalkingGo(),
		filepath.Join(apiRoot, "internal", "providers", "ussd", "yo_uganda.go"):      ussdYoUgandaGo(),
		filepath.Join(apiRoot, "internal", "providers", "ussd", "custom.go"):         ussdCustomGo(),
		filepath.Join(apiRoot, "internal", "providers", "ussd", "registry.go"):       ussdRegistryGo(),
		filepath.Join(apiRoot, "internal", "providers", "ussd", "session.go"):        ussdSessionGo(),
		filepath.Join(apiRoot, "internal", "providers", "ussd", "menu.go"):           ussdMenuGo(),
	}

	for path, content := range files {
		content = strings.ReplaceAll(content, "{{MODULE}}", module)
		if err := writeFile(path, content); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

func ussdInterfaceGo() string {
	return `package ussd

import "context"

// Request represents an incoming USSD request from a provider.
type Request struct {
	// SessionID is the unique USSD session identifier from the provider.
	SessionID string
	// ServiceCode is the USSD code dialled (e.g. *123#).
	ServiceCode string
	// PhoneNumber is the caller's phone number in E.164 format.
	PhoneNumber string
	// Text is the accumulated user input (empty on first request).
	// For Africa's Talking: "1*2*3" means user entered 1, then 2, then 3.
	Text string
	// Input is the latest user input for this menu step.
	Input string
	// IsNewSession is true when this is the first request in a session.
	IsNewSession bool
}

// Response instructs the provider whether to continue or end the session.
type Response struct {
	// Message is the text to display to the user.
	Message string
	// Continue is true to prompt for more input; false to end the session.
	Continue bool
}

// Handler is a function that processes a USSD request and returns a response.
type Handler func(ctx context.Context, req Request) Response

// Provider defines the contract for USSD gateway providers.
type Provider interface {
	// ParseRequest extracts a USSD Request from provider-specific HTTP params.
	ParseRequest(params map[string]string) (Request, error)
	// FormatResponse formats a Response into the provider's expected HTTP response body.
	FormatResponse(resp Response) string
}
`
}

func ussdAfricasTalkingGo() string {
	return `package ussd

import (
	"fmt"
	"strings"
)

// AfricasTalking implements Provider for Africa's Talking USSD gateway.
// TODO: Docs — https://developers.africastalking.com/docs/ussd/handling_sessions
//
// Incoming POST params: sessionId, serviceCode, phoneNumber, text
// Response format: "CON <message>" (continue) or "END <message>" (end session)
type AfricasTalking struct{}

// NewAfricasTalking creates an Africa's Talking USSD provider.
func NewAfricasTalking() *AfricasTalking { return &AfricasTalking{} }

func (a *AfricasTalking) ParseRequest(params map[string]string) (Request, error) {
	sessionID := params["sessionId"]
	if sessionID == "" {
		return Request{}, fmt.Errorf("at ussd: missing sessionId")
	}

	text := params["text"]
	parts := strings.Split(text, "*")
	input := ""
	if len(parts) > 0 {
		input = parts[len(parts)-1]
	}

	return Request{
		SessionID:    sessionID,
		ServiceCode:  params["serviceCode"],
		PhoneNumber:  params["phoneNumber"],
		Text:         text,
		Input:        input,
		IsNewSession: text == "",
	}, nil
}

func (a *AfricasTalking) FormatResponse(resp Response) string {
	if resp.Continue {
		return "CON " + resp.Message
	}
	return "END " + resp.Message
}
`
}

func ussdYoUgandaGo() string {
	return `package ussd

import (
	"fmt"
	"strings"
)

// YoUganda implements Provider for Yo! Uganda USSD gateway.
// TODO: Contact Yo! Uganda for API documentation and credentials.
//
// Incoming POST params: msisdn, USSDServiceCode, USSDRequestString, TransactionExists
// Response format: plain text; session continues if provider sends another request
type YoUganda struct{}

// NewYoUganda creates a Yo Uganda USSD provider.
func NewYoUganda() *YoUganda { return &YoUganda{} }

func (y *YoUganda) ParseRequest(params map[string]string) (Request, error) {
	sessionID := params["TransactionExists"]
	if sessionID == "" {
		sessionID = params["msisdn"] + "_" + params["USSDServiceCode"]
	}

	text := params["USSDRequestString"]
	// Remove leading/trailing * or # that some Yo requests include
	text = strings.Trim(text, "*#")

	parts := strings.Split(text, "*")
	input := ""
	if len(parts) > 0 && parts[len(parts)-1] != "" {
		input = parts[len(parts)-1]
	}

	msisdn := params["msisdn"]
	if msisdn == "" {
		return Request{}, fmt.Errorf("yo ussd: missing msisdn")
	}

	// Ensure E.164 format
	if !strings.HasPrefix(msisdn, "+") {
		msisdn = "+" + msisdn
	}

	return Request{
		SessionID:    sessionID,
		ServiceCode:  params["USSDServiceCode"],
		PhoneNumber:  msisdn,
		Text:         text,
		Input:        input,
		IsNewSession: text == "" || params["TransactionExists"] == "No",
	}, nil
}

func (y *YoUganda) FormatResponse(resp Response) string {
	// Yo Uganda expects plain text response
	return resp.Message
}
`
}

func ussdCustomGo() string {
	return `package ussd

import "fmt"

// CustomUSSD is a stub Provider for custom USSD gateway implementations.
type CustomUSSD struct{ name string }

// NewCustomUSSD creates a custom USSD provider stub.
func NewCustomUSSD(name string) *CustomUSSD { return &CustomUSSD{name: name} }

func (c *CustomUSSD) ParseRequest(params map[string]string) (Request, error) {
	return Request{}, fmt.Errorf("custom ussd [%s]: ParseRequest not implemented", c.name)
}

func (c *CustomUSSD) FormatResponse(resp Response) string {
	return resp.Message
}
`
}

func ussdRegistryGo() string {
	return `package ussd

import (
	"fmt"
	"os"
	"strings"
)

// Registry returns the configured USSD Provider based on USSD_PROVIDER env var.
// Supported values: "africastalking", "at", "yo", "yo_uganda", "custom"
// Defaults to "africastalking" if unset.
func Registry() (Provider, error) {
	provider := strings.ToLower(os.Getenv("USSD_PROVIDER"))
	if provider == "" {
		provider = "africastalking"
	}

	switch provider {
	case "africastalking", "at":
		return NewAfricasTalking(), nil

	case "yo", "yo_uganda":
		return NewYoUganda(), nil

	case "custom":
		name := os.Getenv("CUSTOM_USSD_NAME")
		if name == "" {
			name = "custom"
		}
		return NewCustomUSSD(name), nil

	default:
		return nil, fmt.Errorf("ussd registry: unknown USSD_PROVIDER %q (supported: africastalking, yo, custom)", provider)
	}
}
`
}

func ussdSessionGo() string {
	return `package ussd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// SessionStore manages USSD session state across HTTP requests.
// Sessions are short-lived (USSD sessions typically expire in ~3 minutes).
type SessionStore interface {
	Get(ctx context.Context, sessionID string) (map[string]interface{}, error)
	Set(ctx context.Context, sessionID string, data map[string]interface{}, ttl time.Duration) error
	Delete(ctx context.Context, sessionID string) error
}

// InMemorySessionStore is a simple in-memory session store for development.
// Use RedisSessionStore in production.
type InMemorySessionStore struct {
	sessions map[string]string
}

// NewInMemorySessionStore creates a development session store.
func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{sessions: make(map[string]string)}
}

func (s *InMemorySessionStore) Get(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	raw, ok := s.sessions[sessionID]
	if !ok {
		return map[string]interface{}{}, nil
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fmt.Errorf("ussd session get: %w", err)
	}
	return data, nil
}

func (s *InMemorySessionStore) Set(ctx context.Context, sessionID string, data map[string]interface{}, ttl time.Duration) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("ussd session set: %w", err)
	}
	s.sessions[sessionID] = string(b)
	return nil
}

func (s *InMemorySessionStore) Delete(ctx context.Context, sessionID string) error {
	delete(s.sessions, sessionID)
	return nil
}
`
}

func ussdMenuGo() string {
	return `package ussd

import (
	"context"
	"fmt"
	"strings"
)

// MenuItem represents a single option in a USSD menu.
type MenuItem struct {
	Label   string
	Handler func(ctx context.Context, req Request, session map[string]interface{}) Response
}

// Menu builds USSD menus with numbered options.
// Usage:
//
//	m := ussd.NewMenu("Welcome to MyApp")
//	m.Add("Check Balance", handleBalance)
//	m.Add("Buy Airtime", handleAirtime)
//	m.Add("Exit", handleExit)
//	return m.Handle(ctx, req, session)
type Menu struct {
	Title string
	Items []MenuItem
}

// NewMenu creates a new USSD menu with the given title.
func NewMenu(title string) *Menu {
	return &Menu{Title: title}
}

// Add appends a menu item.
func (m *Menu) Add(label string, handler func(ctx context.Context, req Request, session map[string]interface{}) Response) {
	m.Items = append(m.Items, MenuItem{Label: label, Handler: handler})
}

// Handle processes a USSD request against this menu.
// If input is empty (new session or back-navigation), it displays the menu.
// If input matches a menu number, it delegates to that item's handler.
func (m *Menu) Handle(ctx context.Context, req Request, session map[string]interface{}) Response {
	if req.Input == "" || req.IsNewSession {
		return m.display()
	}

	choice := 0
	fmt.Sscanf(req.Input, "%d", &choice)

	if choice < 1 || choice > len(m.Items) {
		return Response{
			Message:  "Invalid choice. " + m.display().Message,
			Continue: true,
		}
	}

	return m.Items[choice-1].Handler(ctx, req, session)
}

// display renders the menu as a CON response.
func (m *Menu) display() Response {
	var sb strings.Builder
	sb.WriteString(m.Title)
	sb.WriteString("\n")
	for i, item := range m.Items {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, item.Label))
	}
	return Response{Message: strings.TrimRight(sb.String(), "\n"), Continue: true}
}
`
}
