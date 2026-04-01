package deploy

import (
	"fmt"
	"os"
	"strings"
)

// WriteDefaultConfig writes a jua.yaml template to the current directory.
// Called by `jua new` after scaffolding.
func WriteDefaultConfig(appName string) error {
	path := "jua.yaml"
	if _, err := os.Stat(path); err == nil {
		return nil // already exists — don't overwrite
	}

	content := buildDefaultConfig(appName)
	return os.WriteFile(path, []byte(content), 0644)
}

func buildDefaultConfig(appName string) string {
	s := strings.NewReplacer("{{APP_NAME}}", appName)
	return s.Replace(`# jua.yaml — Deployment configuration for {{APP_NAME}}
# Run 'jua deploy' to deploy your app
# Run 'jua deploy:setup' to prepare a fresh VPS

name: {{APP_NAME}}
version: "0.1.0"

deploy:
  production:
    provider: vps              # vps | dokploy | coolify | railway | render | fly
    host: YOUR_SERVER_IP
    ssh_user: root
    ssh_key: ~/.ssh/id_rsa
    domain: yourdomain.com
    ssl: true

  staging:
    provider: vps
    host: YOUR_STAGING_IP
    ssh_user: root
    ssh_key: ~/.ssh/id_rsa
    domain: staging.yourdomain.com
    ssl: true

services:
  api:
    build: ./apps/api
    port: 8080
    health_check: /health
    replicas: 1
    memory: 512mb

  web:
    build: ./apps/web
    port: 3000
    health_check: /
    replicas: 1
    memory: 256mb

  admin:
    build: ./apps/admin
    port: 3001
    health_check: /
    replicas: 1
    memory: 256mb

infrastructure:
  postgres:
    version: "16"
    size: 10gb
  redis:
    version: "7"
  minio:
    enabled: true

env:
  sync_from: .env
  exclude:
    - LOCAL_*
    - DEBUG

multitenancy:
  enabled: false
  routing: subdomain
  wildcard_ssl: false

strategy:
  type: rolling
  health_check_timeout: 60s
  rollback_on_failure: true
`)
}

// PrintConfigHelp prints a helpful message after jua.yaml is written.
func PrintConfigHelp(appName string) {
	fmt.Printf("\n  Created jua.yaml for %s\n", appName)
	fmt.Println("  Edit it to set your server IP, domain, and provider.")
	fmt.Println("  Then run: jua deploy")
}
