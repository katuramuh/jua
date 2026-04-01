# VPS Deployment Guide

Deploy your Jua app to any Linux server (Ubuntu 22.04/24.04, Debian 12).

## Prerequisites

- A Linux VPS with SSH access
- A domain name pointing to the server IP
- Docker installed locally (for building images)

## 1. Configure jua.yaml

```yaml
deploy:
  production:
    provider: vps
    host: YOUR_SERVER_IP
    ssh_user: root
    ssh_key: ~/.ssh/id_rsa
    domain: yourdomain.com
    ssl: true
```

## 2. Provision the server

Run once on a fresh server:

```bash
jua deploy setup
```

This installs: Docker, Docker Compose, Traefik (SSL), UFW firewall.

## 3. Deploy

```bash
jua deploy
```

## 4. Other commands

```bash
jua status              # check health
jua logs api            # view API logs
jua logs --follow       # live log stream
jua env push            # sync .env to server
jua ssh                 # open SSH session
jua rollback --version v1.0.0
```

## Wildcard SSL (multi-tenant)

For `*.yourdomain.com` SSL (required for multi-tenant subdomain routing):

```yaml
multitenancy:
  enabled: true
  wildcard_ssl: true

deploy:
  production:
    provider: vps
    # ... other fields ...
```

During `jua deploy setup` you will be prompted for your Cloudflare API token.
