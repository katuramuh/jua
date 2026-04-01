package providers

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/katuramuh/jua/v3/internal/deploy"
)

// VPSProvider deploys to any Linux VPS via SSH + Docker + Traefik.
// NOTE: Requires SSH access to a Linux server (Ubuntu 22.04/24.04 or Debian 12).
type VPSProvider struct{}

// sshArgs builds the base ssh argument slice for a target.
func sshArgs(t *deploy.DeployTarget) []string {
	args := []string{
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=15",
	}
	if t.SSHKey != "" {
		key := t.SSHKey
		if strings.HasPrefix(key, "~/") {
			home, _ := os.UserHomeDir()
			key = home + key[1:]
		}
		args = append(args, "-i", key)
	}
	return args
}

// sshRun executes a command on the remote server and streams output.
func sshRun(t *deploy.DeployTarget, command string) error {
	user := t.SSHUser
	if user == "" {
		user = "root"
	}
	args := sshArgs(t)
	args = append(args, fmt.Sprintf("%s@%s", user, t.Host), command)
	cmd := exec.Command("ssh", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// sshOutput executes a command and returns its stdout.
func sshOutput(t *deploy.DeployTarget, command string) (string, error) {
	user := t.SSHUser
	if user == "" {
		user = "root"
	}
	args := sshArgs(t)
	args = append(args, fmt.Sprintf("%s@%s", user, t.Host), command)
	out, err := exec.Command("ssh", args...).Output()
	return strings.TrimSpace(string(out)), err
}

// scpUpload copies a local file to the remote server.
func scpUpload(t *deploy.DeployTarget, localPath, remotePath string) error {
	user := t.SSHUser
	if user == "" {
		user = "root"
	}
	args := sshArgs(t)
	args = append(args, localPath, fmt.Sprintf("%s@%s:%s", user, t.Host, remotePath))
	cmd := exec.Command("scp", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// --- DeployProvider implementation ---

func (p *VPSProvider) Validate(t *deploy.DeployTarget) error {
	if t.Host == "" {
		return fmt.Errorf("vps provider requires 'host' in jua.yaml")
	}
	// Quick connectivity check
	_, err := sshOutput(t, "echo ok")
	if err != nil {
		return fmt.Errorf("cannot connect to %s: %w\nCheck host, ssh_user, and ssh_key in jua.yaml", t.Host, err)
	}
	return nil
}

// Setup provisions a fresh Linux VPS: Docker, Traefik, UFW, deploy user.
func (p *VPSProvider) Setup(t *deploy.DeployTarget, opts *SetupOptions) error {
	pr := opts.Progress

	pr.Step("Connecting to " + t.Host)
	if err := p.Validate(t); err != nil {
		pr.StepFailed("Connecting", err)
		return err
	}
	pr.StepDone("Connecting", 0)

	steps := []struct {
		name string
		cmd  string
	}{
		{
			"Updating packages",
			"export DEBIAN_FRONTEND=noninteractive && apt-get update -q && apt-get upgrade -yq",
		},
		{
			"Installing Docker",
			"curl -fsSL https://get.docker.com | sh && systemctl enable docker && systemctl start docker",
		},
		{
			"Installing Docker Compose plugin",
			"apt-get install -yq docker-compose-plugin",
		},
		{
			"Configuring firewall",
			"ufw allow 22/tcp && ufw allow 80/tcp && ufw allow 443/tcp && ufw --force enable",
		},
		{
			"Creating /opt/jua directories",
			"mkdir -p /opt/jua/traefik /opt/jua/apps",
		},
	}

	for _, s := range steps {
		start := time.Now()
		pr.Step(s.name)
		if err := sshRun(t, s.cmd); err != nil {
			pr.StepFailed(s.name, err)
			return fmt.Errorf("%s: %w", s.name, err)
		}
		pr.StepDone(s.name, time.Since(start))
	}

	// Deploy Traefik
	pr.Step("Deploying Traefik (SSL)")
	email := opts.Email
	if email == "" {
		email = "admin@" + t.Domain
	}
	traefikCompose := buildTraefikCompose(email, t.Domain, opts.CloudflareToken != "")
	escaped := shellEscape(traefikCompose)
	cmd := fmt.Sprintf(
		"echo %s > /opt/jua/traefik/docker-compose.yml && "+
			"touch /opt/jua/traefik/acme.json && chmod 600 /opt/jua/traefik/acme.json && "+
			"cd /opt/jua/traefik && docker compose up -d",
		escaped,
	)
	start := time.Now()
	if err := sshRun(t, cmd); err != nil {
		pr.StepFailed("Deploying Traefik", err)
		return fmt.Errorf("deploying traefik: %w", err)
	}
	pr.StepDone("Deploying Traefik (SSL)", time.Since(start))

	pr.Success("Server setup complete")
	pr.URL("Server", "https://"+t.Domain)
	pr.Hint("Run 'jua deploy' to deploy your app")
	return nil
}

// Deploy performs a rolling deployment to the VPS.
func (p *VPSProvider) Deploy(cfg *deploy.JuaConfig, targetName string, opts *DeployOptions) error {
	t, err := cfg.Target(targetName)
	if err != nil {
		return err
	}

	version := opts.Version
	if version == "" {
		version = cfg.Version
	}
	if version == "" {
		version = "latest"
	}

	pr := opts.Progress
	overall := time.Now()

	pr.Step("Connecting to " + t.Host)
	start := time.Now()
	if err := p.Validate(t); err != nil {
		pr.StepFailed("Connecting", err)
		return err
	}
	pr.StepDone("Connecting", time.Since(start))

	// Determine which services to deploy
	services := opts.Services
	if len(services) == 0 {
		for name := range cfg.Services {
			services = append(services, name)
		}
	}

	// Build Docker images locally
	pr.Step("Building images")
	fmt.Println()
	for _, svc := range services {
		svcCfg, ok := cfg.Services[svc]
		if !ok {
			continue
		}
		imageName := imageTag(cfg.Name, svc, version)
		pr.SubStep("Building", svc, 0, false)
		buildStart := time.Now()
		buildCmd := exec.Command("docker", "build", "-t", imageName, svcCfg.Build)
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			pr.SubStep("Building", svc, 0, false)
			return fmt.Errorf("building %s: %w", svc, err)
		}
		pr.SubStep("Building", svc, 100, true)
		_ = buildStart
	}

	// Push images to GHCR
	pr.Step("Pushing to registry")
	fmt.Println()
	for _, svc := range services {
		imageName := imageTag(cfg.Name, svc, version)
		pr.SubStep("Pushing", svc, 0, false)
		pushCmd := exec.Command("docker", "push", imageName)
		pushCmd.Stdout = os.Stdout
		pushCmd.Stderr = os.Stderr
		if err := pushCmd.Run(); err != nil {
			return fmt.Errorf("pushing %s: %w", svc, err)
		}
		pr.SubStep("Pushing", svc, 100, true)
	}

	// Sync .env
	if cfg.Env.SyncFrom != "" {
		start = time.Now()
		pr.Step("Syncing environment")
		if err := p.EnvPush(cfg, targetName); err != nil {
			pr.StepFailed("Syncing environment", err)
			return err
		}
		pr.StepDone("Syncing environment", time.Since(start))
	}

	// Run migrations
	start = time.Now()
	pr.Step("Running migrations")
	migrateImg := imageTag(cfg.Name, "api", version)
	migrateCmd := fmt.Sprintf(
		"docker run --rm --env-file /opt/jua/apps/%s/.env %s /app/server migrate",
		cfg.Name, migrateImg,
	)
	if err := sshRun(t, migrateCmd); err != nil {
		pr.Log("  (migration skipped or no migrate command)")
	}
	pr.StepDone("Running migrations", time.Since(start))

	// Rolling deploy each service
	pr.Step("Deploying services")
	fmt.Println()
	for _, svc := range services {
		svcCfg, ok := cfg.Services[svc]
		if !ok {
			continue
		}
		imageName := imageTag(cfg.Name, svc, version)
		containerName := fmt.Sprintf("%s-%s", cfg.Name, svc)
		port := svcCfg.Port
		domain := subDomain(svc, t.Domain)

		labels := traefikLabels(containerName, domain, port)
		runCmd := fmt.Sprintf(
			"docker pull %s && "+
				"docker stop %s 2>/dev/null || true && "+
				"docker rm %s 2>/dev/null || true && "+
				"docker run -d --name %s --restart unless-stopped "+
				"--env-file /opt/jua/apps/%s/.env "+
				"--network traefik-public "+
				"%s %s",
			imageName,
			containerName, containerName,
			containerName,
			cfg.Name,
			labels,
			imageName,
		)

		pr.SubStep("Deploying", svc, 0, false)
		if err := sshRun(t, runCmd); err != nil {
			return fmt.Errorf("deploying %s: %w", svc, err)
		}
		pr.SubStep("Deploying", svc, 100, true)
	}

	// Health checks
	pr.Step("Health checks")
	fmt.Println()
	for _, svc := range services {
		svcCfg, ok := cfg.Services[svc]
		if !ok {
			continue
		}
		domain := subDomain(svc, t.Domain)
		healthURL := fmt.Sprintf("https://%s%s", domain, svcCfg.HealthCheck)
		pr.SubStep("Health", svc, 0, false)
		if err := waitForHealth(t, healthURL, 60*time.Second); err != nil {
			pr.SubStep("Health", svc, 0, false)
			pr.Log(fmt.Sprintf("  %s health check failed: %v", svc, err))
		} else {
			pr.SubStep("Health", svc, 100, true)
		}
	}

	pr.Success(fmt.Sprintf("Deployed in %s", deploy.FormatDuration(time.Since(overall))))
	for _, svc := range services {
		domain := subDomain(svc, t.Domain)
		pr.URL(svc, "https://"+domain)
	}
	pr.Hint("Run 'jua logs' to view live logs")
	pr.Hint("Run 'jua status' to check service health")
	return nil
}

func (p *VPSProvider) Rollback(cfg *deploy.JuaConfig, targetName string, version string) error {
	t, err := cfg.Target(targetName)
	if err != nil {
		return err
	}
	fmt.Printf("Rolling back %s to %s on %s...\n", cfg.Name, version, t.Host)
	for svc := range cfg.Services {
		imageName := imageTag(cfg.Name, svc, version)
		containerName := fmt.Sprintf("%s-%s", cfg.Name, svc)
		svcCfg := cfg.Services[svc]
		domain := subDomain(svc, t.Domain)
		labels := traefikLabels(containerName, domain, svcCfg.Port)
		cmd := fmt.Sprintf(
			"docker pull %s && docker stop %s 2>/dev/null || true && docker rm %s 2>/dev/null || true && "+
				"docker run -d --name %s --restart unless-stopped --env-file /opt/jua/apps/%s/.env --network traefik-public %s %s",
			imageName, containerName, containerName,
			containerName, cfg.Name, labels, imageName,
		)
		if err := sshRun(t, cmd); err != nil {
			return fmt.Errorf("rolling back %s: %w", svc, err)
		}
		fmt.Printf("  ✓ %s\n", svc)
	}
	return nil
}

func (p *VPSProvider) Status(cfg *deploy.JuaConfig, targetName string) (*DeployStatus, error) {
	t, err := cfg.Target(targetName)
	if err != nil {
		return nil, err
	}
	out, err := sshOutput(t, `docker ps --format "{{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null`)
	if err != nil {
		return nil, fmt.Errorf("getting status from %s: %w", t.Host, err)
	}

	status := &DeployStatus{Version: cfg.Version}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		// Only show services belonging to this app
		if !strings.HasPrefix(name, cfg.Name+"-") {
			continue
		}
		svcName := strings.TrimPrefix(name, cfg.Name+"-")
		runStatus := parts[1]
		health := "unknown"
		if strings.Contains(runStatus, "Up") {
			health = "healthy"
		}
		status.Services = append(status.Services, ServiceStatus{
			Name:   svcName,
			Status: "running",
			Health: health,
			URL:    "https://" + subDomain(svcName, t.Domain),
		})
	}
	return status, nil
}

func (p *VPSProvider) Logs(cfg *deploy.JuaConfig, targetName string, service string, follow bool, lines int) error {
	t, err := cfg.Target(targetName)
	if err != nil {
		return err
	}
	containerName := fmt.Sprintf("%s-%s", cfg.Name, service)
	logCmd := fmt.Sprintf("docker logs %s --tail %d", containerName, lines)
	if follow {
		logCmd += " --follow"
	}
	return sshRun(t, logCmd)
}

func (p *VPSProvider) Scale(cfg *deploy.JuaConfig, targetName string, service string, replicas int) error {
	// Docker standalone doesn't support native replicas — use docker-compose scale or Swarm.
	// For simplicity, we restart the container (replicas=1 model for VPS direct).
	fmt.Printf("Note: VPS provider uses single-container deployment. For multi-replica scaling, use Dokploy or Coolify.\n")
	fmt.Printf("Restarting %s-%s on %s...\n", cfg.Name, service, targetName)
	t, err := cfg.Target(targetName)
	if err != nil {
		return err
	}
	return sshRun(t, fmt.Sprintf("docker restart %s-%s", cfg.Name, service))
}

func (p *VPSProvider) EnvPush(cfg *deploy.JuaConfig, targetName string) error {
	t, err := cfg.Target(targetName)
	if err != nil {
		return err
	}
	src := cfg.Env.SyncFrom
	if src == "" {
		src = ".env"
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("reading %s: %w", src, err)
	}
	filtered := filterEnv(string(data), cfg.Env.Exclude)
	remotePath := fmt.Sprintf("/opt/jua/apps/%s/.env", cfg.Name)
	escaped := shellEscape(filtered)
	return sshRun(t, fmt.Sprintf("mkdir -p /opt/jua/apps/%s && echo %s > %s", cfg.Name, escaped, remotePath))
}

func (p *VPSProvider) EnvPull(cfg *deploy.JuaConfig, targetName string, dest string) error {
	t, err := cfg.Target(targetName)
	if err != nil {
		return err
	}
	remotePath := fmt.Sprintf("/opt/jua/apps/%s/.env", cfg.Name)
	if dest == "" {
		dest = ".env.remote"
	}
	return scpDownload(t, remotePath, dest)
}

func (p *VPSProvider) EnvList(cfg *deploy.JuaConfig, targetName string) (map[string]string, error) {
	t, err := cfg.Target(targetName)
	if err != nil {
		return nil, err
	}
	remotePath := fmt.Sprintf("/opt/jua/apps/%s/.env", cfg.Name)
	out, err := sshOutput(t, "cat "+remotePath)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = "***"
		}
	}
	return result, nil
}

func (p *VPSProvider) EnvSet(cfg *deploy.JuaConfig, targetName string, key, value string) error {
	t, err := cfg.Target(targetName)
	if err != nil {
		return err
	}
	remotePath := fmt.Sprintf("/opt/jua/apps/%s/.env", cfg.Name)
	// Use sed to update or append
	cmd := fmt.Sprintf(
		`grep -q "^%s=" %s && sed -i 's|^%s=.*|%s=%s|' %s || echo '%s=%s' >> %s`,
		key, remotePath,
		key, key, value, remotePath,
		key, value, remotePath,
	)
	return sshRun(t, cmd)
}

func (p *VPSProvider) SSH(t *deploy.DeployTarget) error {
	user := t.SSHUser
	if user == "" {
		user = "root"
	}
	args := sshArgs(t)
	args = append(args, fmt.Sprintf("%s@%s", user, t.Host))
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (p *VPSProvider) Run(t *deploy.DeployTarget, command string) error {
	return sshRun(t, command)
}

// --- helpers ---

func imageTag(appName, service, version string) string {
	return fmt.Sprintf("ghcr.io/%s/%s-%s:%s", "katuramuh", appName, service, version)
}

func subDomain(service, domain string) string {
	switch service {
	case "web":
		return domain
	case "api":
		return "api." + domain
	case "admin":
		return "admin." + domain
	default:
		return service + "." + domain
	}
}

func traefikLabels(name, domain string, port int) string {
	return fmt.Sprintf(
		`--label "traefik.enable=true" `+
			`--label "traefik.http.routers.%s.rule=Host(\`+"`"+`%s\`+"`"+`)" `+
			`--label "traefik.http.routers.%s.tls.certresolver=letsencrypt" `+
			`--label "traefik.http.services.%s.loadbalancer.server.port=%d"`,
		name, domain, name, name, port,
	)
}

func buildTraefikCompose(email, domain string, useCloudflare bool) string {
	certResolver := `
      - --certificatesresolvers.letsencrypt.acme.tlschallenge=true`
	if useCloudflare {
		certResolver = `
      - --certificatesresolvers.letsencrypt.acme.dnschallenge=true
      - --certificatesresolvers.letsencrypt.acme.dnschallenge.provider=cloudflare`
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
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./acme.json:/acme/acme.json
    networks:
      - traefik-public
    restart: unless-stopped

networks:
  traefik-public:
    external: true
`, email, certResolver)
}

func filterEnv(content string, exclude []string) string {
	var out []string
	for _, line := range strings.Split(content, "\n") {
		skip := false
		key := ""
		if idx := strings.Index(line, "="); idx > 0 {
			key = line[:idx]
		}
		for _, pattern := range exclude {
			if matchGlob(pattern, key) {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func matchGlob(pattern, s string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(s, pattern[:len(pattern)-1])
	}
	return pattern == s
}

func shellEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `'\''`)
	return `'` + s + `'`
}

func scpDownload(t *deploy.DeployTarget, remotePath, localPath string) error {
	user := t.SSHUser
	if user == "" {
		user = "root"
	}
	args := sshArgs(t)
	args = append(args, fmt.Sprintf("%s@%s:%s", user, t.Host, remotePath), localPath)
	cmd := exec.Command("scp", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func waitForHealth(t *deploy.DeployTarget, url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := sshOutput(t, fmt.Sprintf("curl -sf -o /dev/null -w '%%{http_code}' %s 2>/dev/null", url))
		if err == nil && out == "200" {
			return nil
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("health check timed out after %s", timeout)
}
