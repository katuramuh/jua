# jua.yaml Reference

Full reference for the `jua.yaml` deployment configuration file.

## Top-level fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Application name |
| `version` | string | no | Current version (e.g. `"0.1.0"`) |
| `deploy` | map | yes | Deploy targets (e.g. `production`, `staging`) |
| `services` | map | no | Service definitions (api, web, admin) |
| `infrastructure` | object | no | Managed infrastructure (Postgres, Redis, MinIO) |
| `env` | object | no | Environment variable sync config |
| `multitenancy` | object | no | Multi-tenant deployment options |
| `strategy` | object | no | Deployment strategy (rolling, blue-green) |

## deploy

Each key is an environment name (e.g. `production`, `staging`).

```yaml
deploy:
  production:
    provider: vps          # vps | dokploy | coolify | railway | render | fly
    host: 165.22.100.1
    ssh_user: root
    ssh_key: ~/.ssh/id_rsa
    domain: yourdomain.com
    ssl: true
```

| Field | Description |
|-------|-------------|
| `provider` | Provider name |
| `host` | Server IP or hostname |
| `ssh_user` | SSH username (VPS only, default: `root`) |
| `ssh_key` | Path to SSH private key (VPS only) |
| `domain` | Root domain for the app |
| `ssl` | Enable Let's Encrypt SSL |
| `api_token` | API token (Dokploy / Coolify / Railway / Render / Fly) |
| `project_id` | Project ID (Railway) |
| `region` | Deployment region (Fly: `jnb`, `lhr`, `iad`, etc.) |
| `service_ids` | Render service IDs map (Render only) |

## services

```yaml
services:
  api:
    build: ./apps/api
    port: 8080
    health_check: /health
    replicas: 1
    memory: 512mb
```

## strategy

```yaml
strategy:
  type: rolling                  # rolling (default)
  health_check_timeout: 60s
  rollback_on_failure: true
```

## multitenancy

```yaml
multitenancy:
  enabled: true
  routing: subdomain             # subdomain | path | header
  wildcard_ssl: true             # requires Cloudflare DNS token
```
