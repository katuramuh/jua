# Contributing to Jua

Thank you for helping build Africa's Go + React framework.

## Ways to Contribute

- Report bugs via [GitHub Issues](https://github.com/katuramuh/jua/issues)
- Request features or new providers
- Add provider implementations (SMS, Payment, WhatsApp, USSD, Push, Email)
- Improve documentation
- Fix bugs
- Build example apps

## Adding a New Provider

This is the most valuable contribution. Here is exactly how to do it.

### Step 1 — Create the provider file

Location: `apps/api/providers/{category}/{provider_name}.go`

Implement the interface for the category. Example for SMS:

```go
type MyProviderSMS struct {
    apiKey   string
    senderID string
}

func (p *MyProviderSMS) Send(to string, message string) error {
    // your implementation using net/http
}

func (p *MyProviderSMS) SendOTP(to string, otp string) error {
    return p.Send(to, fmt.Sprintf("Your OTP is: %s. Expires in 5 minutes.", otp))
}
```

### Step 2 — Register it

Add a case to `apps/api/providers/{category}/registry.go`:

```go
case "myprovider":
    return NewMyProviderSMS(cfg), nil
```

### Step 3 — Add env variables

Add to `.env.example`:

```env
MY_PROVIDER_API_KEY=
MY_PROVIDER_SENDER_ID=
```

### Step 4 — Submit a PR

Use the pull request template. Include:
- Provider name and website
- Regions/countries supported
- API docs link
- Test evidence (curl output or test code)

## Development Setup

```bash
# Clone the repo
git clone https://github.com/katuramuh/jua.git
cd jua

# Build the CLI
go build ./cmd/jua

# Run tests
go test ./... -race

# Lint
go vet ./...
```

## Commit Message Format

```
feat: add Pesapal payment provider
fix: resolve MTN MoMo callback verification bug
docs: add Relworx setup guide
test: add Paystack webhook verification tests
chore: update dependencies
```

## Code Style

- Follow existing Go patterns in the codebase
- Run `go vet ./...` before submitting
- No hardcoded credentials — always use env variables
- Handlers are thin — business logic goes in services
- All errors must be handled explicitly (no `_` discards)
- Use `fmt.Errorf("context: %w", err)` for error wrapping

## Provider Checklist (PR review will check these)

- [ ] Implements the correct Jua interface
- [ ] Added to the category registry
- [ ] Env variables added to `.env.example`
- [ ] No external Go packages added to CLI's `go.mod` (use `net/http` only)
- [ ] Passes `go vet ./...`

## License

By contributing, you agree your code is licensed under the MIT License.
