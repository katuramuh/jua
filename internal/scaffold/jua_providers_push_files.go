package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"
)

func writeJuaProvidersPushFiles(root string, opts Options) error {
	apiRoot := opts.APIRoot(root)
	module := opts.Module()
	_ = module

	files := map[string]string{
		filepath.Join(apiRoot, "internal", "providers", "push", "interface.go"):  pushInterfaceGo(),
		filepath.Join(apiRoot, "internal", "providers", "push", "fcm.go"):        pushFCMGo(),
		filepath.Join(apiRoot, "internal", "providers", "push", "onesignal.go"):  pushOneSignalGo(),
		filepath.Join(apiRoot, "internal", "providers", "push", "custom.go"):     pushCustomGo(),
		filepath.Join(apiRoot, "internal", "providers", "push", "registry.go"):   pushRegistryGo(),
	}

	for path, content := range files {
		content = strings.ReplaceAll(content, "{{MODULE}}", module)
		if err := writeFile(path, content); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

func pushInterfaceGo() string {
	return `package push

import "context"

// PushNotification holds the payload for a push notification.
type PushNotification struct {
	// Token is a single device FCM/APNs token.
	Token string
	// Tokens is used for batch sends (multicast).
	Tokens []string
	// Topic sends to a topic instead of a device token.
	Topic string
	// Title is the notification title.
	Title string
	// Body is the notification body.
	Body string
	// Data is a key-value map of extra data sent to the app.
	Data map[string]string
	// ImageURL is an optional image to show in the notification.
	ImageURL string
	// Badge count for iOS.
	Badge int
	// Sound for the notification ("default" or custom sound name).
	Sound string
}

// PushResponse is returned after sending a push notification.
type PushResponse struct {
	MessageID    string
	SuccessCount int
	FailureCount int
	Errors       []string
	Raw          map[string]interface{}
}

// PushProvider defines the contract for push notification providers.
type PushProvider interface {
	// Send sends a push notification to a single device token.
	Send(ctx context.Context, n PushNotification) (*PushResponse, error)
	// SendMulticast sends to multiple device tokens at once.
	SendMulticast(ctx context.Context, n PushNotification) (*PushResponse, error)
	// SendToTopic sends to all subscribers of a topic.
	SendToTopic(ctx context.Context, n PushNotification) (*PushResponse, error)
	// SubscribeToTopic subscribes device tokens to a topic.
	SubscribeToTopic(ctx context.Context, tokens []string, topic string) error
	// UnsubscribeFromTopic unsubscribes device tokens from a topic.
	UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) error
}
`
}

func pushFCMGo() string {
	return `package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// FCM implements PushProvider using Firebase Cloud Messaging HTTP v1 API.
// Uses a service account access token (OAuth2) — no firebase-admin SDK required.
// TODO: Docs — https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages
type FCM struct {
	projectID   string
	serverKey   string // Legacy server key (for v1 API fallback)
	accessToken string // OAuth2 access token from service account
	baseURL     string
	client      *http.Client
}

// NewFCM creates an FCM push provider.
// Required env vars: FCM_PROJECT_ID, FCM_SERVER_KEY (legacy) OR FCM_ACCESS_TOKEN (v1)
func NewFCM() *FCM {
	return &FCM{
		projectID:   os.Getenv("FCM_PROJECT_ID"),
		serverKey:   os.Getenv("FCM_SERVER_KEY"),
		accessToken: os.Getenv("FCM_ACCESS_TOKEN"),
		baseURL:     "https://fcm.googleapis.com",
		client:      &http.Client{Timeout: 30 * time.Second},
	}
}

func (f *FCM) buildMessage(n PushNotification) map[string]interface{} {
	notification := map[string]interface{}{
		"title": n.Title,
		"body":  n.Body,
	}
	if n.ImageURL != "" {
		notification["image"] = n.ImageURL
	}

	android := map[string]interface{}{
		"notification": map[string]interface{}{
			"sound":       n.Sound,
			"click_action": "FLUTTER_NOTIFICATION_CLICK",
		},
	}
	if n.Sound == "" {
		android["notification"].(map[string]interface{})["sound"] = "default"
	}

	apns := map[string]interface{}{
		"payload": map[string]interface{}{
			"aps": map[string]interface{}{
				"sound": n.Sound,
				"badge": n.Badge,
			},
		},
	}
	if n.Sound == "" {
		apns["payload"].(map[string]interface{})["aps"].(map[string]interface{})["sound"] = "default"
	}

	msg := map[string]interface{}{
		"notification": notification,
		"android":      android,
		"apns":         apns,
	}
	if len(n.Data) > 0 {
		msg["data"] = n.Data
	}
	return msg
}

func (f *FCM) sendV1(ctx context.Context, message map[string]interface{}) (*PushResponse, error) {
	url := fmt.Sprintf("%s/v1/projects/%s/messages:send", f.baseURL, f.projectID)
	body, _ := json.Marshal(map[string]interface{}{"message": message})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+f.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fcm http: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fcm: %v", result["error"])
	}

	msgID, _ := result["name"].(string)
	return &PushResponse{MessageID: msgID, SuccessCount: 1, Raw: result}, nil
}

func (f *FCM) Send(ctx context.Context, n PushNotification) (*PushResponse, error) {
	msg := f.buildMessage(n)
	msg["token"] = n.Token
	return f.sendV1(ctx, msg)
}

func (f *FCM) SendMulticast(ctx context.Context, n PushNotification) (*PushResponse, error) {
	if len(n.Tokens) == 0 {
		return &PushResponse{}, nil
	}

	// FCM v1 doesn't support multicast natively — use legacy batch endpoint
	url := f.baseURL + "/fcm/send"
	body, _ := json.Marshal(map[string]interface{}{
		"registration_ids": n.Tokens,
		"notification": map[string]string{
			"title": n.Title,
			"body":  n.Body,
		},
		"data": n.Data,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "key="+f.serverKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fcm multicast http: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	success, _ := result["success"].(float64)
	failure, _ := result["failure"].(float64)
	return &PushResponse{
		SuccessCount: int(success),
		FailureCount: int(failure),
		Raw:          result,
	}, nil
}

func (f *FCM) SendToTopic(ctx context.Context, n PushNotification) (*PushResponse, error) {
	msg := f.buildMessage(n)
	msg["topic"] = n.Topic
	return f.sendV1(ctx, msg)
}

func (f *FCM) SubscribeToTopic(ctx context.Context, tokens []string, topic string) error {
	url := fmt.Sprintf("https://iid.googleapis.com/iid/v1:batchAdd")
	body, _ := json.Marshal(map[string]interface{}{
		"to":                "/topics/" + topic,
		"registration_tokens": tokens,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "key="+f.serverKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("fcm subscribe: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("fcm subscribe: status %d", resp.StatusCode)
	}
	return nil
}

func (f *FCM) UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) error {
	url := fmt.Sprintf("https://iid.googleapis.com/iid/v1:batchRemove")
	body, _ := json.Marshal(map[string]interface{}{
		"to":                "/topics/" + topic,
		"registration_tokens": tokens,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "key="+f.serverKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("fcm unsubscribe: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("fcm unsubscribe: status %d", resp.StatusCode)
	}
	return nil
}
`
}

func pushOneSignalGo() string {
	return `package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// OneSignal implements PushProvider using the OneSignal REST API.
// TODO: Docs — https://documentation.onesignal.com/reference/create-notification
type OneSignal struct {
	baseURL string
	appID   string
	apiKey  string
	client  *http.Client
}

// NewOneSignal creates a OneSignal push provider.
// Required env vars: ONESIGNAL_APP_ID, ONESIGNAL_API_KEY
func NewOneSignal() *OneSignal {
	return &OneSignal{
		baseURL: "https://onesignal.com/api/v1",
		appID:   os.Getenv("ONESIGNAL_APP_ID"),
		apiKey:  os.Getenv("ONESIGNAL_API_KEY"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (o *OneSignal) post(ctx context.Context, payload interface{}) (map[string]interface{}, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/notifications", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Basic "+o.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("onesignal http: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if errList, ok := result["errors"]; ok {
		return nil, fmt.Errorf("onesignal: %v", errList)
	}
	return result, nil
}

func (o *OneSignal) buildPayload(n PushNotification) map[string]interface{} {
	payload := map[string]interface{}{
		"app_id":   o.appID,
		"headings": map[string]string{"en": n.Title},
		"contents": map[string]string{"en": n.Body},
	}
	if len(n.Data) > 0 {
		payload["data"] = n.Data
	}
	if n.ImageURL != "" {
		payload["big_picture"] = n.ImageURL
		payload["ios_attachments"] = map[string]string{"id1": n.ImageURL}
	}
	return payload
}

func (o *OneSignal) Send(ctx context.Context, n PushNotification) (*PushResponse, error) {
	payload := o.buildPayload(n)
	payload["include_player_ids"] = []string{n.Token}

	result, err := o.post(ctx, payload)
	if err != nil {
		return nil, err
	}

	id, _ := result["id"].(string)
	return &PushResponse{MessageID: id, SuccessCount: 1, Raw: result}, nil
}

func (o *OneSignal) SendMulticast(ctx context.Context, n PushNotification) (*PushResponse, error) {
	payload := o.buildPayload(n)
	payload["include_player_ids"] = n.Tokens

	result, err := o.post(ctx, payload)
	if err != nil {
		return nil, err
	}

	id, _ := result["id"].(string)
	recipients, _ := result["recipients"].(float64)
	return &PushResponse{MessageID: id, SuccessCount: int(recipients), Raw: result}, nil
}

func (o *OneSignal) SendToTopic(ctx context.Context, n PushNotification) (*PushResponse, error) {
	payload := o.buildPayload(n)
	// OneSignal uses segments for topic-like targeting
	payload["included_segments"] = []string{n.Topic}

	result, err := o.post(ctx, payload)
	if err != nil {
		return nil, err
	}

	id, _ := result["id"].(string)
	recipients, _ := result["recipients"].(float64)
	return &PushResponse{MessageID: id, SuccessCount: int(recipients), Raw: result}, nil
}

func (o *OneSignal) SubscribeToTopic(ctx context.Context, tokens []string, topic string) error {
	// OneSignal uses tags for segmentation — set a tag on each device
	for _, token := range tokens {
		url := fmt.Sprintf("%s/players/%s", o.baseURL, token)
		body, _ := json.Marshal(map[string]interface{}{
			"app_id": o.appID,
			"tags":   map[string]string{topic: "1"},
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := o.client.Do(req)
		if err != nil {
			return fmt.Errorf("onesignal subscribe %s: %w", token, err)
		}
		resp.Body.Close()
	}
	return nil
}

func (o *OneSignal) UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) error {
	for _, token := range tokens {
		url := fmt.Sprintf("%s/players/%s", o.baseURL, token)
		body, _ := json.Marshal(map[string]interface{}{
			"app_id": o.appID,
			"tags":   map[string]string{topic: ""},
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := o.client.Do(req)
		if err != nil {
			return fmt.Errorf("onesignal unsubscribe %s: %w", token, err)
		}
		resp.Body.Close()
	}
	return nil
}
`
}

func pushCustomGo() string {
	return `package push

import (
	"context"
	"fmt"
)

// CustomPush is a stub PushProvider for custom implementations.
type CustomPush struct{ name string }

// NewCustomPush creates a custom push provider stub.
func NewCustomPush(name string) *CustomPush { return &CustomPush{name: name} }

func (c *CustomPush) Send(ctx context.Context, n PushNotification) (*PushResponse, error) {
	return nil, fmt.Errorf("custom push [%s]: Send not implemented", c.name)
}

func (c *CustomPush) SendMulticast(ctx context.Context, n PushNotification) (*PushResponse, error) {
	return nil, fmt.Errorf("custom push [%s]: SendMulticast not implemented", c.name)
}

func (c *CustomPush) SendToTopic(ctx context.Context, n PushNotification) (*PushResponse, error) {
	return nil, fmt.Errorf("custom push [%s]: SendToTopic not implemented", c.name)
}

func (c *CustomPush) SubscribeToTopic(ctx context.Context, tokens []string, topic string) error {
	return fmt.Errorf("custom push [%s]: SubscribeToTopic not implemented", c.name)
}

func (c *CustomPush) UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) error {
	return fmt.Errorf("custom push [%s]: UnsubscribeFromTopic not implemented", c.name)
}
`
}

func pushRegistryGo() string {
	return `package push

import (
	"fmt"
	"os"
	"strings"
)

// Registry returns the configured PushProvider based on PUSH_PROVIDER env var.
// Supported values: "fcm", "onesignal", "custom"
// Defaults to "fcm" if unset.
func Registry() (PushProvider, error) {
	provider := strings.ToLower(os.Getenv("PUSH_PROVIDER"))
	if provider == "" {
		provider = "fcm"
	}

	switch provider {
	case "fcm":
		if os.Getenv("FCM_PROJECT_ID") == "" {
			return nil, fmt.Errorf("push registry: PUSH_PROVIDER=fcm but FCM_PROJECT_ID is not set")
		}
		return NewFCM(), nil

	case "onesignal":
		if os.Getenv("ONESIGNAL_APP_ID") == "" {
			return nil, fmt.Errorf("push registry: PUSH_PROVIDER=onesignal but ONESIGNAL_APP_ID is not set")
		}
		return NewOneSignal(), nil

	case "custom":
		name := os.Getenv("CUSTOM_PUSH_NAME")
		if name == "" {
			name = "custom"
		}
		return NewCustomPush(name), nil

	default:
		return nil, fmt.Errorf("push registry: unknown PUSH_PROVIDER %q (supported: fcm, onesignal, custom)", provider)
	}
}
`
}
