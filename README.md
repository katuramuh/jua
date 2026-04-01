# Jua — Go + React Full-Stack Framework

**Go + React. Built for Africa.**

![Release](https://img.shields.io/github/v/release/katuramuh/jua)
![Build](https://img.shields.io/github/actions/workflow/status/katuramuh/jua/ci.yml)
![License](https://img.shields.io/github/license/katuramuh/jua)
![Go Version](https://img.shields.io/github/go-mod/go-version/katuramuh/jua)

Jua is a batteries-included full-stack framework for CRMs, admin dashboards, SaaS products, and internal tools. Go performance on the backend. React on the frontend. Built with African developers in mind.

> **Jua** means *sun* in Swahili — representing clarity, energy, and the rising African tech ecosystem.

---

## Install

### CLI (recommended)
```bash
go install github.com/katuramuh/jua/v3/cmd/jua@latest
```

### Docker
```bash
docker pull ghcr.io/katuramuh/jua:latest
```

### Download Binary
Download the latest binary for your platform from the [Releases page](https://github.com/katuramuh/jua/releases).

## Quick Start

```bash
jua new myapp
cd myapp
docker compose up -d
jua dev
```

---

## What You Get

- **Go backend** — Fiber framework, GORM ORM, PostgreSQL/SQLite
- **React frontend** — Next.js 15, Tailwind CSS, shadcn/ui
- **Admin panel** — Full CRUD, data tables, forms, role-based access
- **Auth** — JWT + refresh tokens, OAuth (Google, GitHub), TOTP/2FA
- **File storage** — S3-compatible, local disk
- **Email** — Resend, SMTP
- **Jobs** — Background workers, cron scheduling
- **API docs** — Scalar / OpenAPI auto-generated
- **Docker** — Production-ready Dockerfile + compose

---

## Project Structure

```
myapp/
├── cmd/server/         # Go entry point
├── internal/
│   ├── handlers/       # HTTP route handlers
│   ├── models/         # GORM models
│   ├── services/       # Business logic
│   ├── middleware/      # Auth, CORS, rate limiting
│   └── migrations/     # DB migrations + seeders
├── web/                # Next.js frontend
│   ├── app/            # App router pages
│   ├── components/     # React components
│   └── lib/            # Utilities, API client
└── jua.config.yaml     # Project config
```

---

## CLI Commands

```bash
jua new <name>           # Scaffold a new Jua project
jua generate model <name> # Generate a model + handler + service
jua generate page <name>  # Generate a frontend page
jua dev                   # Start dev server (Go + Next.js)
jua build                 # Build for production
jua deploy                # Deploy to server
```

---

## Features

- Full-stack in one command — Go API + React UI
- Admin panel with drag-and-drop resource builder
- Role-based access control (RBAC)
- Multi-tenant SaaS support
- Real-time via WebSockets
- Caching with Redis
- Background jobs with retry logic
- OpenAPI 3.0 documentation
- Desktop app support (Wails)
- Mobile app support (Expo/React Native)

---

## Supported Providers

| Category | Providers |
|---|---|
| SMS | AfricasTalking, Relworx, eSMS Africa, Twilio, Vonage, Custom |
| Mobile Money | MTN MoMo, Airtel Money, Flutterwave, Paystack, PawaPay, Custom |
| International Payments | Stripe, Flutterwave |
| WhatsApp | AfricasTalking, Twilio, Meta Cloud API, Custom |
| Email | Resend, SMTP, Custom |
| Push Notifications | Firebase FCM, OneSignal |
| USSD | AfricasTalking, Yo! Uganda, Custom HTTP |

Adding a new provider? See [CONTRIBUTING.md](CONTRIBUTING.md)

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go, Fiber, GORM |
| Frontend | Next.js 15, React 19, Tailwind CSS |
| Database | PostgreSQL (production), SQLite (dev) |
| Auth | JWT, OAuth2, TOTP |
| Storage | S3 / MinIO / local |
| Email | Resend / SMTP |
| Docs | Scalar / OpenAPI |
| Desktop | Wails |
| Mobile | Expo |

---

## Contributing

Jua is open source under the MIT license. Contributions welcome.

### Adding a New Provider
1. Implement the interface in `apps/api/providers/{category}/`
2. Register it in the category registry file
3. Add env variables to `.env.example`
4. Submit a pull request

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full guide.

---

## Documentation

Visit [juaframework.dev](https://juaframework.dev) for full documentation.

---

## License

MIT — Copyright (c) 2025 Katuramu, WastePay Tech

---

Built by [Katuramu](https://github.com/katuramuh) / WastePay Tech
GitHub: [github.com/katuramuh/jua](https://github.com/katuramuh/jua)
