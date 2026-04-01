package providers

// NOTE: Requires Coolify v4 with API access enabled.
// API docs: https://coolify.io/docs/api-reference/introduction

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/katuramuh/jua/v3/internal/deploy"
)

type CoolifyProvider struct{}

type coolifyClient struct {
	base  string
	token string
}

func (p *CoolifyProvider) client(t *deploy.DeployTarget) *coolifyClient {
	base := t.Host
	if !strings.HasPrefix(base, "http") {
		base = "https://" + base
	}
	return &coolifyClient{base: base, token: t.APIToken}
}

func (c *coolifyClient) do(method, path string, body interface{}) (map[string]interface{}, error) {
	var buf io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+"/api/v1"+path, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if resp.StatusCode >= 400 {
		return result, fmt.Errorf("coolify API error %d", resp.StatusCode)
	}
	return result, nil
}

func (c *coolifyClient) doList(method, path string) ([]interface{}, error) {
	var buf io.Reader
	req, err := http.NewRequest(method, c.base+"/api/v1"+path, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result []interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("coolify API error %d", resp.StatusCode)
	}
	return result, nil
}

func (p *CoolifyProvider) Validate(t *deploy.DeployTarget) error {
	if t.APIToken == "" {
		return fmt.Errorf("coolify provider requires 'api_token' in jua.yaml")
	}
	if t.Host == "" {
		return fmt.Errorf("coolify provider requires 'host' (your Coolify instance URL) in jua.yaml")
	}
	c := p.client(t)
	_, err := c.doList("GET", "/teams")
	return err
}

func (p *CoolifyProvider) Setup(t *deploy.DeployTarget, opts *SetupOptions) error {
	return fmt.Errorf("coolify: Setup is handled by your Coolify instance — install at https://coolify.io")
}

func (p *CoolifyProvider) Deploy(cfg *deploy.JuaConfig, targetName string, opts *DeployOptions) error {
	t, err := cfg.Target(targetName)
	if err != nil {
		return err
	}
	pr := opts.Progress
	c := p.client(t)

	version := opts.Version
	if version == "" {
		version = cfg.Version
	}

	pr.Step("Connecting to Coolify")
	start := time.Now()
	if err := p.Validate(t); err != nil {
		pr.StepFailed("Connecting to Coolify", err)
		return err
	}
	pr.StepDone("Connecting to Coolify", time.Since(start))

	// Get project UUID
	pr.Step("Finding project")
	start = time.Now()
	projectUUID, err := p.ensureProject(c, cfg.Name)
	if err != nil {
		pr.StepFailed("Finding project", err)
		return err
	}
	pr.StepDone("Finding project", time.Since(start))

	services := opts.Services
	if len(services) == 0 {
		for name := range cfg.Services {
			services = append(services, name)
		}
	}

	for _, svc := range services {
		svcCfg, ok := cfg.Services[svc]
		if !ok {
			continue
		}
		imageName := imageTag(cfg.Name, svc, version)
		domain := subDomain(svc, t.Domain)

		pr.Step("Deploying " + svc)
		start = time.Now()

		appUUID, err := p.ensureApplication(c, projectUUID, cfg.Name+"-"+svc, imageName, domain, svcCfg.Port)
		if err != nil {
			pr.StepFailed("Deploying "+svc, err)
			return fmt.Errorf("ensuring coolify app %s: %w", svc, err)
		}

		_, err = c.do("POST", "/applications/"+appUUID+"/deploy", nil)
		if err != nil {
			pr.StepFailed("Deploying "+svc, err)
			return fmt.Errorf("triggering coolify deploy for %s: %w", svc, err)
		}

		if err := p.waitForDeploy(c, appUUID, 120*time.Second); err != nil {
			pr.StepFailed("Deploying "+svc, err)
			return err
		}
		pr.StepDone("Deploying "+svc, time.Since(start))
	}

	pr.Success("Deployed to Coolify")
	for _, svc := range services {
		pr.URL(svc, "https://"+subDomain(svc, t.Domain))
	}
	return nil
}

func (p *CoolifyProvider) ensureProject(c *coolifyClient, name string) (string, error) {
	projects, err := c.doList("GET", "/projects")
	if err != nil {
		return "", err
	}
	for _, proj := range projects {
		if m, ok := proj.(map[string]interface{}); ok {
			if m["name"] == name {
				return m["uuid"].(string), nil
			}
		}
	}
	created, err := c.do("POST", "/projects", map[string]string{"name": name})
	if err != nil {
		return "", err
	}
	return created["uuid"].(string), nil
}

func (p *CoolifyProvider) ensureApplication(c *coolifyClient, projectUUID, name, image, domain string, port int) (string, error) {
	apps, err := c.doList("GET", "/applications")
	if err != nil {
		return "", err
	}
	for _, app := range apps {
		if m, ok := app.(map[string]interface{}); ok {
			if m["name"] == name {
				uuid := m["uuid"].(string)
				c.do("PATCH", "/applications/"+uuid, map[string]interface{}{
					"docker_image": image,
				})
				return uuid, nil
			}
		}
	}
	created, err := c.do("POST", "/applications", map[string]interface{}{
		"name":         name,
		"project_uuid": projectUUID,
		"type":         "dockerfile",
		"docker_image": image,
		"ports_exposes": fmt.Sprintf("%d", port),
		"fqdn":         "https://" + domain,
	})
	if err != nil {
		return "", err
	}
	return created["uuid"].(string), nil
}

func (p *CoolifyProvider) waitForDeploy(c *coolifyClient, uuid string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result, err := c.do("GET", "/applications/"+uuid, nil)
		if err == nil {
			if status, ok := result["status"].(string); ok {
				if status == "running" {
					return nil
				}
				if status == "stopped" || status == "errored" {
					return fmt.Errorf("coolify deployment failed: status=%s", status)
				}
			}
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("coolify deployment timed out after %s", timeout)
}

func (p *CoolifyProvider) Rollback(cfg *deploy.JuaConfig, targetName string, version string) error {
	opts := &DeployOptions{Version: version, Progress: &noopProgress{}}
	return p.Deploy(cfg, targetName, opts)
}

func (p *CoolifyProvider) Status(cfg *deploy.JuaConfig, targetName string) (*DeployStatus, error) {
	t, err := cfg.Target(targetName)
	if err != nil {
		return nil, err
	}
	c := p.client(t)
	apps, err := c.doList("GET", "/applications")
	if err != nil {
		return nil, err
	}
	status := &DeployStatus{Version: cfg.Version}
	for _, app := range apps {
		if m, ok := app.(map[string]interface{}); ok {
			name, _ := m["name"].(string)
			if !strings.HasPrefix(name, cfg.Name+"-") {
				continue
			}
			svcName := strings.TrimPrefix(name, cfg.Name+"-")
			appStatus, _ := m["status"].(string)
			status.Services = append(status.Services, ServiceStatus{
				Name:   svcName,
				Status: appStatus,
				Health: "unknown",
				URL:    "https://" + subDomain(svcName, t.Domain),
			})
		}
	}
	return status, nil
}

func (p *CoolifyProvider) Logs(cfg *deploy.JuaConfig, targetName string, service string, follow bool, lines int) error {
	t, err := cfg.Target(targetName)
	if err != nil {
		return err
	}
	c := p.client(t)
	// Find app UUID first
	apps, err := c.doList("GET", "/applications")
	if err != nil {
		return err
	}
	for _, app := range apps {
		if m, ok := app.(map[string]interface{}); ok {
			if m["name"] == cfg.Name+"-"+service {
				uuid := m["uuid"].(string)
				result, err := c.do("GET", "/applications/"+uuid+"/logs", nil)
				if err != nil {
					return err
				}
				if logs, ok := result["logs"].(string); ok {
					fmt.Println(logs)
				}
				return nil
			}
		}
	}
	return fmt.Errorf("service %s not found on Coolify", service)
}

func (p *CoolifyProvider) Scale(cfg *deploy.JuaConfig, targetName string, service string, replicas int) error {
	return fmt.Errorf("scaling not supported via Coolify API — use the Coolify web UI")
}

func (p *CoolifyProvider) EnvPush(cfg *deploy.JuaConfig, targetName string) error {
	return fmt.Errorf("coolify: use the Coolify web UI to manage environment variables")
}

func (p *CoolifyProvider) EnvPull(cfg *deploy.JuaConfig, targetName string, dest string) error {
	return fmt.Errorf("coolify: env pull not supported via API")
}

func (p *CoolifyProvider) EnvList(cfg *deploy.JuaConfig, targetName string) (map[string]string, error) {
	return nil, fmt.Errorf("coolify: env list not supported via API")
}

func (p *CoolifyProvider) EnvSet(cfg *deploy.JuaConfig, targetName string, key, value string) error {
	return fmt.Errorf("coolify: use the Coolify web UI to set environment variables")
}

func (p *CoolifyProvider) SSH(t *deploy.DeployTarget) error {
	return fmt.Errorf("coolify: SSH not available — use the Coolify web terminal")
}

func (p *CoolifyProvider) Run(t *deploy.DeployTarget, command string) error {
	return fmt.Errorf("coolify: remote run not supported via API")
}
