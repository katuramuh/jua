package deploy

import "fmt"

// TraefikConfig generates Traefik v3 configurations for VPS deployments.

// TraefikComposeConfig holds the inputs for generating a traefik docker-compose.
type TraefikComposeConfig struct {
	Email           string
	UseCloudflare   bool
	CloudflareEmail string
	CloudflareToken string
}

// GenerateTraefikCompose returns a docker-compose.yml for Traefik v3 with Let's Encrypt.
func GenerateTraefikCompose(cfg TraefikComposeConfig) string {
	certConfig := `
      - --certificatesresolvers.letsencrypt.acme.tlschallenge=true`
	envSection := ""

	if cfg.UseCloudflare {
		certConfig = `
      - --certificatesresolvers.letsencrypt.acme.dnschallenge=true
      - --certificatesresolvers.letsencrypt.acme.dnschallenge.provider=cloudflare`
		envSection = fmt.Sprintf(`    environment:
      - CF_API_EMAIL=%s
      - CF_API_KEY=%s
`, cfg.CloudflareEmail, cfg.CloudflareToken)
	}

	return fmt.Sprintf(`version: "3.8"
services:
  traefik:
    image: traefik:v3.0
    command:
      - --api.insecure=false
      - --providers.docker=true
      - --providers.docker.exposedbydefault=false
      - --entrypoints.web.address=:80
      - --entrypoints.websecure.address=:443
      - --certificatesresolvers.letsencrypt.acme.email=%s
      - --certificatesresolvers.letsencrypt.acme.storage=/acme/acme.json%s
      - --entrypoints.web.http.redirections.entrypoint.to=websecure
      - --entrypoints.web.http.redirections.entrypoint.scheme=https
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./acme.json:/acme/acme.json
    networks:
      - traefik-public
    restart: unless-stopped
%s
networks:
  traefik-public:
    name: traefik-public
    driver: bridge
`, cfg.Email, certConfig, envSection)
}

// ServiceLabels returns the Traefik Docker labels for a service container.
func ServiceLabels(serviceName, domain string, port int, certResolver string) map[string]string {
	if certResolver == "" {
		certResolver = "letsencrypt"
	}
	return map[string]string{
		"traefik.enable": "true",
		fmt.Sprintf("traefik.http.routers.%s.rule", serviceName):                      fmt.Sprintf("Host(`%s`)", domain),
		fmt.Sprintf("traefik.http.routers.%s.entrypoints", serviceName):               "websecure",
		fmt.Sprintf("traefik.http.routers.%s.tls.certresolver", serviceName):          certResolver,
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", serviceName): fmt.Sprintf("%d", port),
	}
}

// WildcardCertConfig returns Traefik config for wildcard SSL (multi-tenant subdomain routing).
func WildcardCertConfig(domain, email, cloudflareToken string) string {
	return fmt.Sprintf(`version: "3.8"
services:
  traefik:
    image: traefik:v3.0
    command:
      - --providers.docker=true
      - --providers.docker.exposedbydefault=false
      - --entrypoints.web.address=:80
      - --entrypoints.websecure.address=:443
      - --certificatesresolvers.wildcard.acme.email=%s
      - --certificatesresolvers.wildcard.acme.storage=/acme/acme.json
      - --certificatesresolvers.wildcard.acme.dnschallenge=true
      - --certificatesresolvers.wildcard.acme.dnschallenge.provider=cloudflare
      - --entrypoints.web.http.redirections.entrypoint.to=websecure
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./acme.json:/acme/acme.json
    environment:
      - CF_API_KEY=%s
    networks:
      - traefik-public
    restart: unless-stopped
    labels:
      - "traefik.http.routers.traefik-wildcard.tls.certresolver=wildcard"
      - "traefik.http.routers.traefik-wildcard.tls.domains[0].main=%s"
      - "traefik.http.routers.traefik-wildcard.tls.domains[0].sans=*.%s"

networks:
  traefik-public:
    name: traefik-public
    driver: bridge
`, email, cloudflareToken, domain, domain)
}
