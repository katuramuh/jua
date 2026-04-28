package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"
)

func writeJuaUSSDEngineFiles(root string, opts Options) error {
	apiRoot := opts.APIRoot(root)
	module := opts.Module()

	files := map[string]string{
		filepath.Join(apiRoot, "internal", "ussd", "app.go"):     ussdAppGo(),
		filepath.Join(apiRoot, "internal", "ussd", "router.go"):  ussdRouterGo(),
		filepath.Join(apiRoot, "internal", "ussd", "flow.go"):    ussdFlowGo(),
		filepath.Join(apiRoot, "internal", "ussd", "handler.go"): ussdGinHandlerGo(),
		filepath.Join(apiRoot, "internal", "ussd", "example.go"): ussdExampleGo(),
	}

	for path, content := range files {
		content = strings.ReplaceAll(content, "{{MODULE}}", module)
		if err := writeFile(path, content); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

func ussdAppGo() string {
	return `package ussd

import (
	"context"
	"log"

	"{{MODULE}}/internal/providers/ussd"
	// Jua events — fire session completion event
	"{{MODULE}}/jua/events"
)

// App is the USSD application engine.
// It connects an incoming provider request to the correct flow handler
// and manages session state across multiple USSD round-trips.
//
// Usage:
//
//	app := ussd.NewApp(ussd.NewAfricasTalking(), store)
//	app.Handle("*123#", myFlow.Dispatch)
//	app.Handle("*456#", anotherFlow.Dispatch)
//
//	// In your Gin route:
//	router.POST("/ussd", ussd.GinHandler(app))
type App struct {
	provider ussd.Provider
	store    ussd.SessionStore
	router   *Router
}

// NewApp creates a new USSD application.
func NewApp(provider ussd.Provider, store ussd.SessionStore) *App {
	return &App{
		provider: provider,
		store:    store,
		router:   NewRouter(),
	}
}

// Handle registers a handler function for a given USSD service code.
//
//	app.Handle("*123#", flow.Dispatch)
func (a *App) Handle(serviceCode string, handler HandlerFunc) {
	a.router.Handle(serviceCode, handler)
}

// Dispatch processes a raw parameter map from the HTTP request
// and returns the formatted response string for the provider.
func (a *App) Dispatch(ctx context.Context, params map[string]string) string {
	req, err := a.provider.ParseRequest(params)
	if err != nil {
		log.Printf("ussd: parse request error: %v", err)
		return a.provider.FormatResponse(ussd.Response{
			Message:  "An error occurred. Please try again.",
			Continue: false,
		})
	}

	// Load session state
	session, err := a.store.Get(ctx, req.SessionID)
	if err != nil {
		log.Printf("ussd: load session %s error: %v", req.SessionID, err)
		session = map[string]interface{}{}
	}

	// Route to the correct handler
	resp := a.router.Dispatch(ctx, req, session)

	// Persist updated session state (or delete on end)
	if resp.Continue {
		if err := a.store.Set(ctx, req.SessionID, session, 0); err != nil {
			log.Printf("ussd: save session %s error: %v", req.SessionID, err)
		}
	} else {
		if err := a.store.Delete(ctx, req.SessionID); err != nil {
			log.Printf("ussd: delete session %s error: %v", req.SessionID, err)
		}
		// Jua events: session ended — fire for real-time dashboards and notifications
		events.Publish(events.USSDSessionCompleted, "", map[string]interface{}{
			"session_id":   req.SessionID,
			"phone":        req.PhoneNumber,
			"service_code": req.ServiceCode,
		})
	}

	return a.provider.FormatResponse(resp)
}
`
}

func ussdRouterGo() string {
	return `package ussd

import (
	"context"
	"fmt"

	"{{MODULE}}/internal/providers/ussd"
)

// HandlerFunc is the function signature for USSD route handlers.
// It receives the request, the mutable session map, and returns a response.
type HandlerFunc func(ctx context.Context, req ussd.Request, session map[string]interface{}) ussd.Response

// Router maps USSD service codes to handler functions.
type Router struct {
	routes map[string]HandlerFunc
}

// NewRouter creates a new USSD router.
func NewRouter() *Router {
	return &Router{routes: make(map[string]HandlerFunc)}
}

// Handle registers a handler for a service code.
func (r *Router) Handle(serviceCode string, handler HandlerFunc) {
	r.routes[serviceCode] = handler
}

// Dispatch routes the request to the correct handler and returns the response.
func (r *Router) Dispatch(ctx context.Context, req ussd.Request, session map[string]interface{}) ussd.Response {
	handler, ok := r.routes[req.ServiceCode]
	if !ok {
		return ussd.Response{
			Message:  fmt.Sprintf("Service %s is not registered.", req.ServiceCode),
			Continue: false,
		}
	}
	return handler(ctx, req, session)
}
`
}

func ussdFlowGo() string {
	return `package ussd

import (
	"context"
	"fmt"
	"strings"

	"{{MODULE}}/internal/providers/ussd"
)

// Step represents a single step in a USSD flow.
type Step struct {
	// Key is the unique identifier for this step within the flow.
	Key string
	// Prompt is the message shown to the user.
	Prompt string
	// Validate optionally validates the user's input before advancing.
	// Return ("", nil) to accept any input.
	// Return ("error message", nil) to show an error and re-prompt.
	Validate func(input string) (errMsg string, fatal error)
	// Next returns the key of the next step, or "" to end the session.
	Next func(input string, session map[string]interface{}) string
	// Action is called after successful validation to perform side effects.
	Action func(ctx context.Context, input string, session map[string]interface{}) error
}

// Flow is a declarative multi-step USSD interaction.
// Steps are executed in order based on the current session state.
//
// Usage:
//
//	flow := ussd.NewFlow("main")
//	flow.AddStep(ussd.Step{
//	    Key:    "phone",
//	    Prompt: "Enter your phone number:",
//	    Next:   func(input string, _ map[string]interface{}) string { return "pin" },
//	})
//	flow.AddStep(ussd.Step{
//	    Key:    "pin",
//	    Prompt: "Enter your 4-digit PIN:",
//	    Validate: func(input string) (string, error) {
//	        if len(input) != 4 { return "PIN must be 4 digits", nil }
//	        return "", nil
//	    },
//	    Next: func(_ string, _ map[string]interface{}) string { return "" }, // end
//	})
//	app.Handle("*123#", flow.Dispatch)
type Flow struct {
	name       string
	steps      map[string]*Step
	firstStep  string
	stepOrder  []string
}

// NewFlow creates a new USSD flow.
func NewFlow(name string) *Flow {
	return &Flow{
		name:  name,
		steps: make(map[string]*Step),
	}
}

// AddStep appends a step to the flow.
func (f *Flow) AddStep(step Step) {
	if f.firstStep == "" {
		f.firstStep = step.Key
	}
	f.steps[step.Key] = &step
	f.stepOrder = append(f.stepOrder, step.Key)
}

const sessionCurrentStep = "__step"
const sessionLastError = "__error"

// Dispatch is the HandlerFunc for this flow.
func (f *Flow) Dispatch(ctx context.Context, req ussd.Request, session map[string]interface{}) ussd.Response {
	// Determine current step
	currentKey := f.firstStep
	if v, ok := session[sessionCurrentStep].(string); ok && v != "" {
		currentKey = v
	}

	step, ok := f.steps[currentKey]
	if !ok {
		return ussd.Response{
			Message:  fmt.Sprintf("Flow error: unknown step %q", currentKey),
			Continue: false,
		}
	}

	// New session or re-prompt after error: show the current step's prompt
	if req.IsNewSession || req.Input == "" {
		delete(session, sessionLastError)
		return ussd.Response{Message: f.buildPrompt(step, session), Continue: true}
	}

	// Handle back navigation ("0" = go back)
	if req.Input == "0" && currentKey != f.firstStep {
		prev := f.previousStep(currentKey)
		if prev != "" {
			session[sessionCurrentStep] = prev
			return ussd.Response{Message: f.buildPrompt(f.steps[prev], session), Continue: true}
		}
	}

	// Validate input
	if step.Validate != nil {
		errMsg, fatalErr := step.Validate(req.Input)
		if fatalErr != nil {
			return ussd.Response{Message: "An error occurred. Please try again.", Continue: false}
		}
		if errMsg != "" {
			session[sessionLastError] = errMsg
			prompt := errMsg + "\n" + step.Prompt
			return ussd.Response{Message: prompt, Continue: true}
		}
	}

	// Store input in session
	session[step.Key] = req.Input

	// Run action
	if step.Action != nil {
		if err := step.Action(ctx, req.Input, session); err != nil {
			return ussd.Response{Message: "An error occurred: " + err.Error(), Continue: false}
		}
	}

	// Determine next step
	nextKey := ""
	if step.Next != nil {
		nextKey = step.Next(req.Input, session)
	}

	if nextKey == "" {
		// End of flow
		delete(session, sessionCurrentStep)
		return ussd.Response{Message: f.endMessage(session), Continue: false}
	}

	nextStep, ok := f.steps[nextKey]
	if !ok {
		return ussd.Response{
			Message:  fmt.Sprintf("Flow error: step %q not found", nextKey),
			Continue: false,
		}
	}

	session[sessionCurrentStep] = nextKey
	return ussd.Response{Message: f.buildPrompt(nextStep, session), Continue: true}
}

func (f *Flow) buildPrompt(step *Step, session map[string]interface{}) string {
	prompt := step.Prompt
	// Simple template substitution: {{key}} → session[key]
	for k, v := range session {
		if s, ok := v.(string); ok {
			prompt = strings.ReplaceAll(prompt, "{{"+k+"}}", s)
		}
	}
	return prompt
}

func (f *Flow) previousStep(currentKey string) string {
	for i, key := range f.stepOrder {
		if key == currentKey && i > 0 {
			return f.stepOrder[i-1]
		}
	}
	return ""
}

func (f *Flow) endMessage(session map[string]interface{}) string {
	if msg, ok := session["__end_message"].(string); ok {
		return msg
	}
	return "Thank you. Goodbye."
}
`
}

func ussdGinHandlerGo() string {
	return `package ussd

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GinHandler returns a Gin handler that processes USSD requests via an App.
// Mount it on POST /ussd (or whatever path your provider is configured to call).
//
// Example:
//
//	app := ussd.NewApp(ussd.NewAfricasTalking(), store)
//	app.Handle("*123#", myFlow.Dispatch)
//	router.POST("/ussd", ussd.GinHandler(app))
func GinHandler(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Collect all POST form params + query params into a single map
		params := make(map[string]string)

		// Parse form body
		if err := c.Request.ParseForm(); err == nil {
			for k, vs := range c.Request.Form {
				if len(vs) > 0 {
					params[k] = vs[0]
				}
			}
		}

		// Query params override (some providers send GET)
		for k, vs := range c.Request.URL.Query() {
			if len(vs) > 0 {
				params[k] = vs[0]
			}
		}

		response := app.Dispatch(c.Request.Context(), params)

		// USSD responses are plain text
		c.Header("Content-Type", "text/plain")
		c.String(http.StatusOK, response)
	}
}
`
}

func ussdExampleGo() string {
	return `package ussd

// This file demonstrates how to build a USSD flow using the Jua USSD engine.
// Delete or replace this file with your own flows.
//
// Dial *123# to see:
//   Welcome to MyApp
//   1. Check Balance
//   2. Buy Airtime
//   3. Exit

// ExampleBalanceFlow demonstrates a simple 2-step USSD flow.
// Step 1: Ask for phone number
// Step 2: Show mock balance
func ExampleBalanceFlow() *Flow {
	flow := NewFlow("balance")

	flow.AddStep(Step{
		Key:    "phone",
		Prompt: "Enter your phone number (e.g. 0771234567):",
		Validate: func(input string) (string, error) {
			if len(input) < 9 {
				return "Please enter a valid phone number.", nil
			}
			return "", nil
		},
		Next: func(input string, session map[string]interface{}) string {
			return "result"
		},
	})

	flow.AddStep(Step{
		Key:    "result",
		Prompt: "Balance for {{phone}}: UGX 12,500\n\n0. Back",
		Next: func(input string, session map[string]interface{}) string {
			return "" // end session
		},
		Action: func(ctx context.Context, input string, session map[string]interface{}) error {
			// TODO: Look up actual balance from your database/service
			session["__end_message"] = "Your balance is UGX 12,500. Thank you."
			return nil
		},
	})

	return flow
}
`
}
