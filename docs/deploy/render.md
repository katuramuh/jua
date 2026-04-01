# Render Deployment Guide

Deploy to [Render](https://render.com).

## Prerequisites

- A Render account
- Services created in the Render dashboard
- A Render API key

## 1. Create services in Render dashboard

Create a Web Service for each of: api, web, admin.
Note the service IDs (format: `srv-xxxxxxxxxxxx`).

## 2. Configure jua.yaml

```yaml
deploy:
  production:
    provider: render
    api_token: YOUR_RENDER_API_KEY
    domain: yourdomain.com
    service_ids:
      api: srv-xxxxxxxxxxxx
      web: srv-xxxxxxxxxxxx
      admin: srv-xxxxxxxxxxxx
```

## 3. Deploy

```bash
jua deploy
```

Jua will trigger a new deployment for each service and poll until live.

## Notes

- Environment variables can be pushed with `jua env push`
- Render manages SSL automatically for `*.onrender.com` domains
- Custom domains require manual configuration in the Render dashboard
