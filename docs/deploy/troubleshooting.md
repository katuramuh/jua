# Deployment Troubleshooting

## jua.yaml not found

```
jua.yaml not found at jua.yaml
```

Run `jua deploy` from your project root (where `jua.yaml` lives), or pass `--config path/to/jua.yaml`.

## SSH connection refused (VPS)

```
cannot connect to 165.22.100.1
```

- Check that the server IP is correct
- Verify your SSH key is correct: `ssh -i ~/.ssh/id_rsa root@YOUR_IP`
- Ensure port 22 is open in your cloud provider firewall

## Docker build failed

```
building api: exit status 1
```

- Check your `apps/api/Dockerfile` exists
- Test locally: `docker build -t test ./apps/api`
- Ensure Docker is running: `docker ps`

## Health check timeout (VPS)

```
health check timed out after 60s
```

- Check the container started: `jua ssh` then `docker ps`
- View logs: `jua logs api`
- Verify your `health_check` path returns HTTP 200

## Traefik SSL certificate not issuing

- Ensure port 80 and 443 are open in UFW: `ufw status`
- Check your DNS A record points to the server IP
- View Traefik logs: `docker logs traefik`

## Railway: project_id not found

Get the project UUID from your Railway dashboard URL:
`https://railway.app/project/{THIS_IS_YOUR_PROJECT_ID}`

## Render: service_ids required

Create services manually in the Render dashboard first, then add their IDs to `jua.yaml` under `deploy.production.service_ids`.
