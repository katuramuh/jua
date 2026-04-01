# Railway Deployment Guide

Deploy to [Railway](https://railway.app).

## Prerequisites

- A Railway account
- A Railway project created at [railway.app](https://railway.app)
- A Railway API token

## Configure jua.yaml

```yaml
deploy:
  production:
    provider: railway
    api_token: YOUR_RAILWAY_TOKEN
    project_id: YOUR_PROJECT_ID   # from Railway dashboard URL
    domain: yourdomain.com
```

## Deploy

```bash
jua deploy
```

## Get your project ID

From `https://railway.app/project/{project_id}` — copy the UUID from the URL.

## Notes

- Railway auto-scales based on your plan
- Use `railway logs --app APP_NAME` for live logs
- Railway manages SSL automatically
