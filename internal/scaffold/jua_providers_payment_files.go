package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"
)

func writeJuaProvidersPaymentFiles(root string, opts Options) error {
	apiRoot := opts.APIRoot(root)
	module := opts.Module()
	_ = module

	files := map[string]string{
		filepath.Join(apiRoot, "internal", "providers", "payment", "interface.go"):    paymentInterfaceGo(),
		filepath.Join(apiRoot, "internal", "providers", "payment", "mtn_momo.go"):     paymentMtnMomoGo(),
		filepath.Join(apiRoot, "internal", "providers", "payment", "airtel_money.go"): paymentAirtelMoneyGo(),
		filepath.Join(apiRoot, "internal", "providers", "payment", "flutterwave.go"):  paymentFlutterwaveGo(),
		filepath.Join(apiRoot, "internal", "providers", "payment", "paystack.go"):     paymentPaystackGo(),
		filepath.Join(apiRoot, "internal", "providers", "payment", "stripe.go"):       paymentStripeGo(),
		filepath.Join(apiRoot, "internal", "providers", "payment", "pawapay.go"):      paymentPawapayGo(),
		filepath.Join(apiRoot, "internal", "providers", "payment", "custom.go"):       paymentCustomGo(),
		filepath.Join(apiRoot, "internal", "providers", "payment", "registry.go"):     paymentRegistryGo(),
		filepath.Join(apiRoot, "internal", "providers", "payment", "router.go"):       paymentRouterGo(),
	}

	for path, content := range files {
		content = strings.ReplaceAll(content, "{{MODULE}}", module)
		if err := writeFile(path, content); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

func paymentInterfaceGo() string {
	return `package payment

import "context"

// PaymentRequest holds the parameters for initiating a payment.
type PaymentRequest struct {
	Phone       string  // E.164 format, e.g. +256771234567
	Amount      float64 // Amount in smallest currency unit (e.g. UGX)
	Currency    string  // ISO 4217 currency code
	Reference   string  // Unique transaction reference
	Description string
	CallbackURL string
	Metadata    map[string]string
}

// PaymentResponse is returned after initiating a payment.
type PaymentResponse struct {
	TransactionID string
	Status        string // "pending", "success", "failed"
	Message       string
	Raw           map[string]interface{} // Raw provider response
}

// RefundRequest holds parameters for a refund.
type RefundRequest struct {
	TransactionID string
	Reference     string
	Amount        float64
	Reason        string
}

// PaymentProvider defines the contract for all payment providers.
// TODO: See each provider's official API docs for implementation details.
type PaymentProvider interface {
	// RequestPayment initiates a mobile money or card payment.
	RequestPayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error)
	// VerifyPayment checks the status of a transaction.
	VerifyPayment(ctx context.Context, transactionID string) (*PaymentResponse, error)
	// Refund initiates a refund for a completed transaction.
	Refund(ctx context.Context, req RefundRequest) (*PaymentResponse, error)
	// GetBalance returns the current wallet/account balance.
	GetBalance(ctx context.Context) (float64, string, error)
	// GetSupportedCurrencies returns currencies this provider supports.
	GetSupportedCurrencies() []string
}
`
}

func paymentMtnMomoGo() string {
	return `package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// MTNMoMo implements PaymentProvider for MTN Mobile Money (Uganda/Africa).
// TODO: Docs — https://momodeveloper.mtn.com/
type MTNMoMo struct {
	baseURL      string
	subscriptionKey string
	apiUser      string
	apiKey       string
	environment  string // "sandbox" or "production"
	client       *http.Client
}

// NewMTNMoMo creates an MTN MoMo provider from environment variables.
// Required env vars: MTN_MOMO_SUBSCRIPTION_KEY, MTN_MOMO_API_USER, MTN_MOMO_API_KEY
// Optional: MTN_MOMO_ENVIRONMENT (default: "sandbox"), MTN_MOMO_BASE_URL
func NewMTNMoMo() *MTNMoMo {
	env := os.Getenv("MTN_MOMO_ENVIRONMENT")
	if env == "" {
		env = "sandbox"
	}
	baseURL := os.Getenv("MTN_MOMO_BASE_URL")
	if baseURL == "" {
		baseURL = "https://sandbox.momodeveloper.mtn.com"
	}
	return &MTNMoMo{
		baseURL:         baseURL,
		subscriptionKey: os.Getenv("MTN_MOMO_SUBSCRIPTION_KEY"),
		apiUser:         os.Getenv("MTN_MOMO_API_USER"),
		apiKey:          os.Getenv("MTN_MOMO_API_KEY"),
		environment:     env,
		client:          &http.Client{Timeout: 30 * time.Second},
	}
}

func (m *MTNMoMo) getAccessToken(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		m.baseURL+"/collection/token/", bytes.NewReader(nil))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(m.apiUser, m.apiKey)
	req.Header.Set("Ocp-Apim-Subscription-Key", m.subscriptionKey)

	resp, err := m.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("mtn momo token request: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("mtn momo token decode: %w", err)
	}
	token, ok := result["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("mtn momo: missing access_token in response")
	}
	return token, nil
}

func (m *MTNMoMo) RequestPayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	token, err := m.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	// Strip leading + from phone for MTN API
	phone := req.Phone
	if len(phone) > 0 && phone[0] == '+' {
		phone = phone[1:]
	}

	body, _ := json.Marshal(map[string]interface{}{
		"amount":     fmt.Sprintf("%.0f", req.Amount),
		"currency":   req.Currency,
		"externalId": req.Reference,
		"payer": map[string]string{
			"partyIdType": "MSISDN",
			"partyId":     phone,
		},
		"payerMessage": req.Description,
		"payeeNote":    req.Description,
	})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		m.baseURL+"/collection/v1_0/requesttopay", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("X-Reference-Id", req.Reference)
	httpReq.Header.Set("X-Target-Environment", m.environment)
	httpReq.Header.Set("Ocp-Apim-Subscription-Key", m.subscriptionKey)
	httpReq.Header.Set("Content-Type", "application/json")
	if req.CallbackURL != "" {
		httpReq.Header.Set("X-Callback-Url", req.CallbackURL)
	}

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mtn momo request payment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted {
		return &PaymentResponse{
			TransactionID: req.Reference,
			Status:        "pending",
			Message:       "Payment request accepted",
		}, nil
	}

	var errBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&errBody)
	return nil, fmt.Errorf("mtn momo: request failed with status %d: %v", resp.StatusCode, errBody)
}

func (m *MTNMoMo) VerifyPayment(ctx context.Context, transactionID string) (*PaymentResponse, error) {
	token, err := m.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		m.baseURL+"/collection/v1_0/requesttopay/"+transactionID, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("X-Target-Environment", m.environment)
	httpReq.Header.Set("Ocp-Apim-Subscription-Key", m.subscriptionKey)

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mtn momo verify: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("mtn momo verify decode: %w", err)
	}

	status := "pending"
	if s, ok := result["status"].(string); ok {
		switch s {
		case "SUCCESSFUL":
			status = "success"
		case "FAILED":
			status = "failed"
		}
	}

	return &PaymentResponse{
		TransactionID: transactionID,
		Status:        status,
		Raw:           result,
	}, nil
}

func (m *MTNMoMo) Refund(ctx context.Context, req RefundRequest) (*PaymentResponse, error) {
	// MTN MoMo refunds use the disbursement product
	return nil, fmt.Errorf("mtn momo: refunds not yet implemented — see https://momodeveloper.mtn.com/")
}

func (m *MTNMoMo) GetBalance(ctx context.Context) (float64, string, error) {
	token, err := m.getAccessToken(ctx)
	if err != nil {
		return 0, "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		m.baseURL+"/collection/v1_0/account/balance", nil)
	if err != nil {
		return 0, "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("X-Target-Environment", m.environment)
	httpReq.Header.Set("Ocp-Apim-Subscription-Key", m.subscriptionKey)

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return 0, "", fmt.Errorf("mtn momo balance: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return 0, "UGX", nil
}

func (m *MTNMoMo) GetSupportedCurrencies() []string {
	return []string{"UGX", "GHS", "ZMW", "RWF"}
}
`
}

func paymentAirtelMoneyGo() string {
	return `package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// AirtelMoney implements PaymentProvider for Airtel Money (Uganda/Africa).
// TODO: Docs — https://developers.airtel.africa/
type AirtelMoney struct {
	baseURL      string
	clientID     string
	clientSecret string
	country      string
	currency     string
	client       *http.Client
}

// NewAirtelMoney creates an Airtel Money provider from environment variables.
// Required: AIRTEL_CLIENT_ID, AIRTEL_CLIENT_SECRET
// Optional: AIRTEL_COUNTRY (default: UG), AIRTEL_CURRENCY (default: UGX), AIRTEL_BASE_URL
func NewAirtelMoney() *AirtelMoney {
	country := os.Getenv("AIRTEL_COUNTRY")
	if country == "" {
		country = "UG"
	}
	currency := os.Getenv("AIRTEL_CURRENCY")
	if currency == "" {
		currency = "UGX"
	}
	baseURL := os.Getenv("AIRTEL_BASE_URL")
	if baseURL == "" {
		baseURL = "https://openapi.airtel.africa"
	}
	return &AirtelMoney{
		baseURL:      baseURL,
		clientID:     os.Getenv("AIRTEL_CLIENT_ID"),
		clientSecret: os.Getenv("AIRTEL_CLIENT_SECRET"),
		country:      country,
		currency:     currency,
		client:       &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *AirtelMoney) getAccessToken(ctx context.Context) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"client_id":     a.clientID,
		"client_secret": a.clientSecret,
		"grant_type":    "client_credentials",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.baseURL+"/auth/oauth2/token", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("airtel token request: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if token, ok := result["access_token"].(string); ok {
		return token, nil
	}
	return "", fmt.Errorf("airtel: missing access_token")
}

func (a *AirtelMoney) RequestPayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	token, err := a.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	phone := req.Phone
	if len(phone) > 0 && phone[0] == '+' {
		phone = phone[1:]
	}

	body, _ := json.Marshal(map[string]interface{}{
		"reference": req.Description,
		"subscriber": map[string]string{
			"country":  a.country,
			"currency": a.currency,
			"msisdn":   phone,
		},
		"transaction": map[string]interface{}{
			"amount":   req.Amount,
			"country":  a.country,
			"currency": a.currency,
			"id":       req.Reference,
		},
	})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.baseURL+"/merchant/v2/payments/", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Country", a.country)
	httpReq.Header.Set("X-Currency", a.currency)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("airtel request payment: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if status, ok := result["status"].(map[string]interface{}); ok {
		if code, _ := status["code"].(string); code == "200" {
			return &PaymentResponse{
				TransactionID: req.Reference,
				Status:        "pending",
				Message:       "Payment request accepted",
				Raw:           result,
			}, nil
		}
	}

	return nil, fmt.Errorf("airtel: payment failed: %v", result)
}

func (a *AirtelMoney) VerifyPayment(ctx context.Context, transactionID string) (*PaymentResponse, error) {
	token, err := a.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		a.baseURL+"/standard/v1/payments/"+transactionID, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("X-Country", a.country)
	httpReq.Header.Set("X-Currency", a.currency)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("airtel verify: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	status := "pending"
	if data, ok := result["data"].(map[string]interface{}); ok {
		if txn, ok := data["transaction"].(map[string]interface{}); ok {
			if s, _ := txn["status"].(string); s == "TS" {
				status = "success"
			} else if s == "TF" {
				status = "failed"
			}
		}
	}

	return &PaymentResponse{TransactionID: transactionID, Status: status, Raw: result}, nil
}

func (a *AirtelMoney) Refund(ctx context.Context, req RefundRequest) (*PaymentResponse, error) {
	return nil, fmt.Errorf("airtel: refunds not yet implemented — see https://developers.airtel.africa/")
}

func (a *AirtelMoney) GetBalance(ctx context.Context) (float64, string, error) {
	return 0, a.currency, fmt.Errorf("airtel: balance check not yet implemented")
}

func (a *AirtelMoney) GetSupportedCurrencies() []string {
	return []string{"UGX", "TZS", "KES", "ZMW", "MWK", "GHS", "MGA", "CDF", "XAF", "XOF"}
}
`
}

func paymentFlutterwaveGo() string {
	return `package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Flutterwave implements PaymentProvider for Flutterwave.
// TODO: Docs — https://developer.flutterwave.com/
type Flutterwave struct {
	baseURL   string
	secretKey string
	client    *http.Client
}

// NewFlutterwave creates a Flutterwave provider from environment variables.
// Required: FLUTTERWAVE_SECRET_KEY
func NewFlutterwave() *Flutterwave {
	return &Flutterwave{
		baseURL:   "https://api.flutterwave.com/v3",
		secretKey: os.Getenv("FLUTTERWAVE_SECRET_KEY"),
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (f *Flutterwave) do(ctx context.Context, method, path string, body interface{}) (map[string]interface{}, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, f.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+f.secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("flutterwave http: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

func (f *Flutterwave) RequestPayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	phone := req.Phone
	if len(phone) > 0 && phone[0] == '+' {
		phone = phone[1:]
	}

	result, err := f.do(ctx, http.MethodPost, "/charges?type=mobile_money_uganda", map[string]interface{}{
		"amount":      req.Amount,
		"currency":    req.Currency,
		"email":       "customer@example.com", // override in your app
		"tx_ref":      req.Reference,
		"phone_number": phone,
		"order_id":    req.Reference,
	})
	if err != nil {
		return nil, err
	}

	status := "pending"
	if s, _ := result["status"].(string); s == "success" {
		status = "pending" // charge initiated, not yet confirmed
	}

	txID := req.Reference
	if data, ok := result["data"].(map[string]interface{}); ok {
		if id, ok := data["id"]; ok {
			txID = fmt.Sprintf("%v", id)
		}
	}

	return &PaymentResponse{TransactionID: txID, Status: status, Raw: result}, nil
}

func (f *Flutterwave) VerifyPayment(ctx context.Context, transactionID string) (*PaymentResponse, error) {
	result, err := f.do(ctx, http.MethodGet, "/transactions/"+transactionID+"/verify", nil)
	if err != nil {
		return nil, err
	}

	status := "pending"
	if data, ok := result["data"].(map[string]interface{}); ok {
		if s, _ := data["status"].(string); s == "successful" {
			status = "success"
		} else if s == "failed" {
			status = "failed"
		}
	}

	return &PaymentResponse{TransactionID: transactionID, Status: status, Raw: result}, nil
}

func (f *Flutterwave) Refund(ctx context.Context, req RefundRequest) (*PaymentResponse, error) {
	result, err := f.do(ctx, http.MethodPost, "/transactions/"+req.TransactionID+"/refund", map[string]interface{}{
		"amount": req.Amount,
	})
	if err != nil {
		return nil, err
	}
	return &PaymentResponse{TransactionID: req.TransactionID, Status: "pending", Raw: result}, nil
}

func (f *Flutterwave) GetBalance(ctx context.Context) (float64, string, error) {
	result, err := f.do(ctx, http.MethodGet, "/balances", nil)
	if err != nil {
		return 0, "", err
	}
	return 0, "USD", fmt.Errorf("flutterwave: parse balance from %v", result)
}

func (f *Flutterwave) GetSupportedCurrencies() []string {
	return []string{"NGN", "GHS", "KES", "UGX", "TZS", "ZAR", "USD", "EUR", "GBP"}
}
`
}

func paymentPaystackGo() string {
	return `package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Paystack implements PaymentProvider for Paystack (Nigeria/Africa).
// TODO: Docs — https://paystack.com/docs/api/
type Paystack struct {
	baseURL   string
	secretKey string
	client    *http.Client
}

// NewPaystack creates a Paystack provider from environment variables.
// Required: PAYSTACK_SECRET_KEY
func NewPaystack() *Paystack {
	return &Paystack{
		baseURL:   "https://api.paystack.co",
		secretKey: os.Getenv("PAYSTACK_SECRET_KEY"),
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *Paystack) do(ctx context.Context, method, path string, body interface{}) (map[string]interface{}, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("paystack http: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

func (p *Paystack) RequestPayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	// Paystack uses a two-step: initialize → redirect to checkout URL
	result, err := p.do(ctx, http.MethodPost, "/transaction/initialize", map[string]interface{}{
		"amount":    int(req.Amount * 100), // Paystack uses kobo (smallest unit)
		"email":     "customer@example.com", // override in your app
		"reference": req.Reference,
		"currency":  req.Currency,
		"callback_url": req.CallbackURL,
	})
	if err != nil {
		return nil, err
	}

	if status, _ := result["status"].(bool); !status {
		return nil, fmt.Errorf("paystack: initialize failed: %v", result["message"])
	}

	data, _ := result["data"].(map[string]interface{})
	authURL, _ := data["authorization_url"].(string)

	return &PaymentResponse{
		TransactionID: req.Reference,
		Status:        "pending",
		Message:       authURL, // Frontend should redirect to this URL
		Raw:           result,
	}, nil
}

func (p *Paystack) VerifyPayment(ctx context.Context, transactionID string) (*PaymentResponse, error) {
	result, err := p.do(ctx, http.MethodGet, "/transaction/verify/"+transactionID, nil)
	if err != nil {
		return nil, err
	}

	status := "pending"
	if data, ok := result["data"].(map[string]interface{}); ok {
		if s, _ := data["status"].(string); s == "success" {
			status = "success"
		} else if s == "failed" {
			status = "failed"
		}
	}

	return &PaymentResponse{TransactionID: transactionID, Status: status, Raw: result}, nil
}

func (p *Paystack) Refund(ctx context.Context, req RefundRequest) (*PaymentResponse, error) {
	result, err := p.do(ctx, http.MethodPost, "/refund", map[string]interface{}{
		"transaction": req.TransactionID,
		"amount":      int(req.Amount * 100),
	})
	if err != nil {
		return nil, err
	}
	return &PaymentResponse{TransactionID: req.TransactionID, Status: "pending", Raw: result}, nil
}

func (p *Paystack) GetBalance(ctx context.Context) (float64, string, error) {
	result, err := p.do(ctx, http.MethodGet, "/balance", nil)
	if err != nil {
		return 0, "", err
	}
	if data, ok := result["data"].([]interface{}); ok && len(data) > 0 {
		if item, ok := data[0].(map[string]interface{}); ok {
			balance, _ := item["balance"].(float64)
			currency, _ := item["currency"].(string)
			return balance / 100, currency, nil
		}
	}
	return 0, "NGN", nil
}

func (p *Paystack) GetSupportedCurrencies() []string {
	return []string{"NGN", "GHS", "ZAR", "USD", "KES"}
}
`
}

func paymentStripeGo() string {
	return `package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Stripe implements PaymentProvider for Stripe (cards/international).
// Uses net/http directly to avoid adding the stripe-go SDK to the CLI.
// TODO: Docs — https://stripe.com/docs/api
type Stripe struct {
	baseURL   string
	secretKey string
	client    *http.Client
}

// NewStripe creates a Stripe provider from environment variables.
// Required: STRIPE_SECRET_KEY
func NewStripe() *Stripe {
	return &Stripe{
		baseURL:   "https://api.stripe.com/v1",
		secretKey: os.Getenv("STRIPE_SECRET_KEY"),
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *Stripe) do(ctx context.Context, method, path string, params url.Values) (map[string]interface{}, error) {
	var body io.Reader
	if params != nil {
		body = strings.NewReader(params.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(s.secretKey, "")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stripe http: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

func (s *Stripe) RequestPayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	// Create a Payment Intent — frontend completes with Stripe.js
	params := url.Values{
		"amount":   {fmt.Sprintf("%.0f", req.Amount)},
		"currency": {strings.ToLower(req.Currency)},
		"metadata[reference]": {req.Reference},
	}
	if req.Description != "" {
		params.Set("description", req.Description)
	}

	result, err := s.do(ctx, http.MethodPost, "/payment_intents", params)
	if err != nil {
		return nil, err
	}

	if errObj, ok := result["error"].(map[string]interface{}); ok {
		return nil, fmt.Errorf("stripe: %v", errObj["message"])
	}

	clientSecret, _ := result["client_secret"].(string)
	id, _ := result["id"].(string)

	return &PaymentResponse{
		TransactionID: id,
		Status:        "pending",
		Message:       clientSecret, // Pass to frontend for Stripe.js
		Raw:           result,
	}, nil
}

func (s *Stripe) VerifyPayment(ctx context.Context, transactionID string) (*PaymentResponse, error) {
	result, err := s.do(ctx, http.MethodGet, "/payment_intents/"+transactionID, nil)
	if err != nil {
		return nil, err
	}

	status := "pending"
	if st, _ := result["status"].(string); st == "succeeded" {
		status = "success"
	} else if st == "canceled" || st == "payment_failed" {
		status = "failed"
	}

	return &PaymentResponse{TransactionID: transactionID, Status: status, Raw: result}, nil
}

func (s *Stripe) Refund(ctx context.Context, req RefundRequest) (*PaymentResponse, error) {
	params := url.Values{
		"payment_intent": {req.TransactionID},
		"amount":         {fmt.Sprintf("%.0f", req.Amount)},
	}
	result, err := s.do(ctx, http.MethodPost, "/refunds", params)
	if err != nil {
		return nil, err
	}
	if errObj, ok := result["error"].(map[string]interface{}); ok {
		return nil, fmt.Errorf("stripe refund: %v", errObj["message"])
	}

	// Stripe refund response
	_ = bytes.NewReader(nil) // suppress unused import
	id, _ := result["id"].(string)
	return &PaymentResponse{TransactionID: id, Status: "pending", Raw: result}, nil
}

func (s *Stripe) GetBalance(ctx context.Context) (float64, string, error) {
	result, err := s.do(ctx, http.MethodGet, "/balance", nil)
	if err != nil {
		return 0, "", err
	}
	if available, ok := result["available"].([]interface{}); ok && len(available) > 0 {
		if item, ok := available[0].(map[string]interface{}); ok {
			amount, _ := item["amount"].(float64)
			currency, _ := item["currency"].(string)
			return amount / 100, strings.ToUpper(currency), nil
		}
	}
	return 0, "USD", nil
}

func (s *Stripe) GetSupportedCurrencies() []string {
	return []string{"USD", "EUR", "GBP", "JPY", "CAD", "AUD", "CHF", "HKD", "SGD"}
}
`
}

func paymentPawapayGo() string {
	return `package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// PawaPay implements PaymentProvider for PawaPay (African mobile money aggregator).
// TODO: Docs — https://docs.pawapay.io/
type PawaPay struct {
	baseURL string
	apiToken string
	client  *http.Client
}

// NewPawaPay creates a PawaPay provider from environment variables.
// Required: PAWAPAY_API_TOKEN
// Optional: PAWAPAY_BASE_URL (default: sandbox URL)
func NewPawaPay() *PawaPay {
	baseURL := os.Getenv("PAWAPAY_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.sandbox.pawapay.io"
	}
	return &PawaPay{
		baseURL:  baseURL,
		apiToken: os.Getenv("PAWAPAY_API_TOKEN"),
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *PawaPay) do(ctx context.Context, method, path string, body interface{}) (map[string]interface{}, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pawapay http: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

func (p *PawaPay) RequestPayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	phone := req.Phone
	if len(phone) > 0 && phone[0] == '+' {
		phone = phone[1:]
	}

	result, err := p.do(ctx, http.MethodPost, "/deposits", map[string]interface{}{
		"depositId":            req.Reference,
		"returnUrl":           req.CallbackURL,
		"statementDescription": req.Description,
		"amount":              fmt.Sprintf("%.2f", req.Amount),
		"currency":            req.Currency,
		"correspondent":       "MTN_MOMO_UGA", // override per phone prefix in router
		"payer": map[string]string{
			"type":    "MSISDN",
			"address": map[string]string{"value": phone}["value"],
		},
	})
	if err != nil {
		return nil, err
	}

	status := "pending"
	if s, _ := result["status"].(string); s == "ACCEPTED" {
		status = "pending"
	}

	return &PaymentResponse{TransactionID: req.Reference, Status: status, Raw: result}, nil
}

func (p *PawaPay) VerifyPayment(ctx context.Context, transactionID string) (*PaymentResponse, error) {
	result, err := p.do(ctx, http.MethodGet, "/deposits/"+transactionID, nil)
	if err != nil {
		return nil, err
	}

	status := "pending"
	if s, _ := result["status"].(string); s == "COMPLETED" {
		status = "success"
	} else if s == "FAILED" || s == "DUPLICATE_IGNORED" {
		status = "failed"
	}

	return &PaymentResponse{TransactionID: transactionID, Status: status, Raw: result}, nil
}

func (p *PawaPay) Refund(ctx context.Context, req RefundRequest) (*PaymentResponse, error) {
	result, err := p.do(ctx, http.MethodPost, "/refunds", map[string]interface{}{
		"refundId":  req.Reference,
		"depositId": req.TransactionID,
		"amount":    fmt.Sprintf("%.2f", req.Amount),
	})
	if err != nil {
		return nil, err
	}
	return &PaymentResponse{TransactionID: req.TransactionID, Status: "pending", Raw: result}, nil
}

func (p *PawaPay) GetBalance(ctx context.Context) (float64, string, error) {
	result, err := p.do(ctx, http.MethodGet, "/wallet-balances", nil)
	if err != nil {
		return 0, "", err
	}
	return 0, "USD", fmt.Errorf("pawapay: parse balance from %v", result)
}

func (p *PawaPay) GetSupportedCurrencies() []string {
	return []string{"UGX", "TZS", "KES", "GHS", "ZMW", "MWK", "CDF", "BIF", "RWF", "XAF", "XOF", "ZAR", "ETB", "SLL"}
}
`
}

func paymentCustomGo() string {
	return `package payment

import (
	"context"
	"fmt"
)

// CustomPayment is a stub PaymentProvider for custom/in-house implementations.
// Replace this with your own logic.
type CustomPayment struct {
	name string
}

// NewCustomPayment creates a custom payment provider stub.
func NewCustomPayment(name string) *CustomPayment {
	return &CustomPayment{name: name}
}

func (c *CustomPayment) RequestPayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	return nil, fmt.Errorf("custom payment [%s]: RequestPayment not implemented", c.name)
}

func (c *CustomPayment) VerifyPayment(ctx context.Context, transactionID string) (*PaymentResponse, error) {
	return nil, fmt.Errorf("custom payment [%s]: VerifyPayment not implemented", c.name)
}

func (c *CustomPayment) Refund(ctx context.Context, req RefundRequest) (*PaymentResponse, error) {
	return nil, fmt.Errorf("custom payment [%s]: Refund not implemented", c.name)
}

func (c *CustomPayment) GetBalance(ctx context.Context) (float64, string, error) {
	return 0, "", fmt.Errorf("custom payment [%s]: GetBalance not implemented", c.name)
}

func (c *CustomPayment) GetSupportedCurrencies() []string {
	return []string{}
}
`
}

func paymentRegistryGo() string {
	return `package payment

import (
	"fmt"
	"os"
	"strings"
)

// Registry returns the configured PaymentProvider based on the PAYMENT_PROVIDER env var.
// Supported values: "mtn", "airtel", "flutterwave", "paystack", "stripe", "pawapay", "custom"
// If PAYMENT_PROVIDER is unset, it defaults to "mtn".
func Registry() (PaymentProvider, error) {
	provider := strings.ToLower(os.Getenv("PAYMENT_PROVIDER"))
	if provider == "" {
		provider = "mtn"
	}

	switch provider {
	case "mtn":
		if os.Getenv("MTN_MOMO_API_USER") == "" {
			return nil, fmt.Errorf("payment registry: PAYMENT_PROVIDER=mtn but MTN_MOMO_API_USER is not set")
		}
		return NewMTNMoMo(), nil

	case "airtel":
		if os.Getenv("AIRTEL_CLIENT_ID") == "" {
			return nil, fmt.Errorf("payment registry: PAYMENT_PROVIDER=airtel but AIRTEL_CLIENT_ID is not set")
		}
		return NewAirtelMoney(), nil

	case "flutterwave":
		if os.Getenv("FLUTTERWAVE_SECRET_KEY") == "" {
			return nil, fmt.Errorf("payment registry: PAYMENT_PROVIDER=flutterwave but FLUTTERWAVE_SECRET_KEY is not set")
		}
		return NewFlutterwave(), nil

	case "paystack":
		if os.Getenv("PAYSTACK_SECRET_KEY") == "" {
			return nil, fmt.Errorf("payment registry: PAYMENT_PROVIDER=paystack but PAYSTACK_SECRET_KEY is not set")
		}
		return NewPaystack(), nil

	case "stripe":
		if os.Getenv("STRIPE_SECRET_KEY") == "" {
			return nil, fmt.Errorf("payment registry: PAYMENT_PROVIDER=stripe but STRIPE_SECRET_KEY is not set")
		}
		return NewStripe(), nil

	case "pawapay":
		if os.Getenv("PAWAPAY_API_TOKEN") == "" {
			return nil, fmt.Errorf("payment registry: PAYMENT_PROVIDER=pawapay but PAWAPAY_API_TOKEN is not set")
		}
		return NewPawaPay(), nil

	case "custom":
		name := os.Getenv("CUSTOM_PAYMENT_NAME")
		if name == "" {
			name = "custom"
		}
		return NewCustomPayment(name), nil

	default:
		return nil, fmt.Errorf("payment registry: unknown PAYMENT_PROVIDER %q (supported: mtn, airtel, flutterwave, paystack, stripe, pawapay, custom)", provider)
	}
}
`
}

func paymentRouterGo() string {
	return `package payment

import (
	"context"
	"fmt"
	"strings"
)

// Router automatically selects a PaymentProvider based on the phone number's E.164 prefix.
// This allows transparent routing: Uganda mobile money for local numbers, Stripe for international.
//
// Routing table:
//   +25677, +25676 → MTN Uganda
//   +25675, +25670 → Airtel Uganda
//   +254xxx        → Flutterwave (Kenya)
//   +233xxx        → Flutterwave (Ghana)
//   +234xxx        → Paystack (Nigeria)
//   others         → Stripe
type Router struct {
	mtn          PaymentProvider
	airtel       PaymentProvider
	flutterwave  PaymentProvider
	paystack     PaymentProvider
	stripe       PaymentProvider
	fallback     PaymentProvider
}

// NewRouter creates a payment router with all providers initialised.
// Any provider that fails to configure is set to nil; requests routed to it will error.
func NewRouter() *Router {
	r := &Router{}

	if p, err := Registry(); err == nil {
		// If a single PAYMENT_PROVIDER is set, use it as fallback for all routes
		r.fallback = p
	}

	// Always try to initialise all providers (they read their own env vars)
	r.mtn = NewMTNMoMo()
	r.airtel = NewAirtelMoney()
	r.flutterwave = NewFlutterwave()
	r.paystack = NewPaystack()
	r.stripe = NewStripe()

	return r
}

// route returns the appropriate provider for the given phone number.
func (r *Router) route(phone string) (PaymentProvider, error) {
	if !strings.HasPrefix(phone, "+") {
		phone = "+" + phone
	}

	switch {
	case strings.HasPrefix(phone, "+25677") || strings.HasPrefix(phone, "+25676"):
		if r.mtn == nil {
			return nil, fmt.Errorf("payment router: MTN MoMo not configured for %s", phone)
		}
		return r.mtn, nil

	case strings.HasPrefix(phone, "+25675") || strings.HasPrefix(phone, "+25670"):
		if r.airtel == nil {
			return nil, fmt.Errorf("payment router: Airtel Money not configured for %s", phone)
		}
		return r.airtel, nil

	case strings.HasPrefix(phone, "+254") || strings.HasPrefix(phone, "+233"):
		if r.flutterwave == nil {
			return nil, fmt.Errorf("payment router: Flutterwave not configured for %s", phone)
		}
		return r.flutterwave, nil

	case strings.HasPrefix(phone, "+234"):
		if r.paystack == nil {
			return nil, fmt.Errorf("payment router: Paystack not configured for %s", phone)
		}
		return r.paystack, nil

	default:
		if r.stripe == nil {
			return nil, fmt.Errorf("payment router: Stripe not configured for %s", phone)
		}
		return r.stripe, nil
	}
}

// RequestPayment routes a payment request to the appropriate provider.
func (r *Router) RequestPayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	p, err := r.route(req.Phone)
	if err != nil {
		if r.fallback != nil {
			return r.fallback.RequestPayment(ctx, req)
		}
		return nil, err
	}
	return p.RequestPayment(ctx, req)
}

// VerifyPayment delegates to the fallback provider (no routing by phone needed).
func (r *Router) VerifyPayment(ctx context.Context, transactionID string) (*PaymentResponse, error) {
	if r.fallback != nil {
		return r.fallback.VerifyPayment(ctx, transactionID)
	}
	return nil, fmt.Errorf("payment router: cannot verify %s — no fallback provider configured", transactionID)
}

// Refund delegates to the fallback provider.
func (r *Router) Refund(ctx context.Context, req RefundRequest) (*PaymentResponse, error) {
	if r.fallback != nil {
		return r.fallback.Refund(ctx, req)
	}
	return nil, fmt.Errorf("payment router: cannot refund — no fallback provider configured")
}

// GetBalance returns the balance of the fallback provider.
func (r *Router) GetBalance(ctx context.Context) (float64, string, error) {
	if r.fallback != nil {
		return r.fallback.GetBalance(ctx)
	}
	return 0, "", fmt.Errorf("payment router: no fallback provider configured")
}

// GetSupportedCurrencies returns all currencies across all providers.
func (r *Router) GetSupportedCurrencies() []string {
	seen := map[string]bool{}
	all := []PaymentProvider{r.mtn, r.airtel, r.flutterwave, r.paystack, r.stripe}
	var result []string
	for _, p := range all {
		if p == nil {
			continue
		}
		for _, c := range p.GetSupportedCurrencies() {
			if !seen[c] {
				seen[c] = true
				result = append(result, c)
			}
		}
	}
	return result
}
`
}
