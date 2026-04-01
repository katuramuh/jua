# Dokploy Deployment Guide

Deploy to a self-hosted [Dokploy](https://dokploy.com) instance.

## Prerequisites

- A running Dokploy instance
- An API token from your Dokploy dashboard

## Configure jua.yaml

```yaml
deploy:
  production:
    provider: dokploy
    host: dokploy.yourdomain.com   # your Dokploy instance URL
    api_token: YOUR_DOKPLOY_TOKEN
    domain: yourdomain.com
```

## Deploy

```bash
jua deploy
```

Jua will:
1. Find or create a project named after your app
2. Create an application for each service (api, web, admin)
3. Set the Docker image and trigger deployment
4. Poll until deployment completes

## Notes

- Environment variables are managed in the Dokploy web UI
- For logs, use the Dokploy dashboard or `jua logs`
