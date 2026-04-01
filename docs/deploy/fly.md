# Fly.io Deployment Guide

Deploy to [Fly.io](https://fly.io) — great for African latency (Johannesburg region: `jnb`).

## Prerequisites

- A Fly.io account
- `flyctl` installed: `curl -L https://fly.io/install.sh | sh`
- A Fly.io API token

## Configure jua.yaml

```yaml
deploy:
  production:
    provider: fly
    api_token: YOUR_FLY_TOKEN
    region: jnb              # jnb=Johannesburg, lhr=London, iad=Virginia
    org: personal            # your Fly.io org slug
    domain: yourdomain.com
```

## Deploy

```bash
jua deploy
```

If `flyctl` is installed, Jua uses it directly. Otherwise it falls back to the Machines API.

## Regions for Africa

| Region | Location |
|--------|----------|
| `jnb` | Johannesburg, South Africa |
| `lhr` | London (low latency to East Africa) |
| `bom` | Mumbai (low latency to East Africa) |

## Notes

- Scale with `jua scale api --replicas 3`
- View logs with `jua logs api --follow`
- SSH into a machine: `flyctl ssh console --app myapp-api`
