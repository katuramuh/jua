# Changelog

All notable changes to Jua Framework will be documented here.
Format follows [Keep a Changelog](https://keepachangelog.com)
Versioning follows [Semantic Versioning](https://semver.org)

## [Unreleased]

## [0.2.0] - 2026-04-01

### Added
- `jua deploy` — one-command deployment to any provider (VPS, Dokploy, Coolify, Railway, Render, Fly.io)
- `jua deploy setup` — automated fresh VPS provisioning (Docker, Traefik, UFW, Let's Encrypt SSL)
- `jua rollback --version vX.Y.Z` — instant rollback to any previous version
- `jua status` — service health dashboard (name, status, health, URL)
- `jua logs [service] [--follow]` — live log streaming
- `jua env push/pull/list/set` — remote environment variable management
- `jua scale SERVICE --replicas N` — service replica scaling
- `jua ssh` — direct SSH session to deployment server
- `jua run COMMAND` — execute commands on the remote server
- `jua open [service]` — open deployed app in browser
- `internal/deploy/config.go` — `JuaConfig` struct + `LoadConfig()` for `jua.yaml`
- `internal/deploy/config_writer.go` — `WriteDefaultConfig()` generates `jua.yaml` on `jua new`
- `internal/deploy/progress.go` — colored terminal progress reporter
- `internal/deploy/health.go` — concurrent HTTP health checker
- `internal/deploy/builder.go` — Docker image builder/pusher
- `internal/deploy/traefik.go` — Traefik v3 config generator (standard + wildcard SSL)
- `internal/deploy/providers/` — provider abstraction layer (interface + 6 implementations)
- Zero-downtime rolling deployments with automatic rollback on health check failure
- Traefik v3 reverse proxy with automatic Let's Encrypt SSL
- Wildcard SSL support for multi-tenant subdomain routing (via Cloudflare DNS challenge)
- `docs/deploy/` — provider-specific deployment guides (VPS, Dokploy, Railway, Render, Fly.io, troubleshooting)

## [0.1.0] - 2026-04-01

### Added
- Initial release of Jua Framework
- Go + React full-stack monorepo scaffolder (`jua new <app>`)
- Go backend — Gin framework, GORM ORM, PostgreSQL/SQLite
- Next.js 14+ frontend with App Router, Tailwind CSS, shadcn/ui
- Admin panel with dark theme, data tables, resource builder
- JWT authentication system with refresh tokens
- GORM Studio database browser embedded in the API
- Background jobs with Asynq + cron scheduler
- Redis cache service + middleware
- S3-compatible file storage (AWS S3, Cloudflare R2, MinIO)
- Docker development environment (docker-compose.yml + production variant)
- CLI commands: `jua new`, `jua generate resource`, `jua sync`, `jua version`
- Code generator: full-stack resource scaffold (model, service, handler, Zod, TS types, React hooks, admin page)
- Jua UI Component Registry — 91 shadcn-compatible components
- Go API test templates (testify, SQLite in-memory)
- Frontend test templates (Vitest, RTL, Playwright)
- CI/CD: GitHub Actions workflows (CI + release + Docker)

### Provider Abstraction Layer (Built for Africa)
- **SMS:** AfricasTalking, Relworx, eSMS Africa, Twilio, Vonage
- **Mobile Money:** MTN MoMo, Airtel Money, Flutterwave, Paystack, PawaPay, Stripe
- **WhatsApp:** AfricasTalking, Twilio, Meta Cloud API
- **Email:** Resend, SMTP
- **Push Notifications:** Firebase FCM, OneSignal
- **USSD:** AfricasTalking, Yo! Uganda, custom HTTP

### SaaS Modules
- Multi-tenancy (subdomain/header/domain resolution, GORM scopes)
- Phone OTP authentication (6-digit, bcrypt, Redis rate-limiting)
- USSD handler engine (session state machine, declarative menus)
- Webhook system (incoming verification + outgoing HMAC signing + retry)
- PWA + offline support (service worker, install prompt, background sync)
- Subscription billing engine (Plan/Subscription/Invoice, mobile money recurring)
- `jua generate agents` — AGENTS.md generator for AI coding agents

### Phone Number Routing (Uganda-first)
- +25677 / +25676 → MTN Uganda
- +25675 / +25670 → Airtel Uganda
- +254 → Flutterwave (Kenya)
- +234 → Paystack (Nigeria)
- Others → Stripe / Flutterwave

[0.1.0]: https://github.com/katuramuh/jua/releases/tag/v0.1.0
