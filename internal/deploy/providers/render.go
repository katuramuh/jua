package providers

// NOTE: Requires a Render account with an API key.
// API docs: https://api-docs.render.com/reference/introduction

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

type RenderProvider struct{}

const renderBase = "https://api.render.com/v1"

type renderClient struct{ token string }

func (c *renderClient) do(method, path string, body interface{}) (interface{}, error) {
	var buf io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, renderBase+path, buf)
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
	var result interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if resp.StatusCode >= 400 {
		return result, fmt.Errorf("render API error %d", resp.StatusCode)
	}
	return result, nil
}

func (p *RenderProvider) Validate(t *deploy.DeployTarget) error {
	if t.APIToken == "" {
		return fmt.Errorf("render provider requires 'api_token' in jua.yaml")
	}
	c := &renderClient{token: t.APIToken}
	_, err := c.do("GET", "/services?limit=1", nil)
	return err
}

func (p *RenderProvider) Setup(t *deploy.DeployTarget, opts *SetupOptions) error {
	return fmt.Errorf("render: create services at https://dashboard.render.com then add their service IDs to jua.yaml under deploy.production.service_ids")
}

func (p *RenderProvider) Deploy(cfg *deploy.JuaConfig, targetName string, opts *DeployOptions) error {
	t, err := cfg.Target(targetName)
	if err != nil {
		return err
	}
	pr := opts.Progress
	c := &renderClient{token: t.APIToken}

	version := opts.Version
	if version == "" {
		version = cfg.Version
	}

	pr.Step("Connecting to Render")
	start := time.Now()
	if err := p.Validate(t); err != nil {
		pr.StepFailed("Connecting to Render", err)
		return err
	}
	pr.StepDone("Connecting to Render", time.Since(start))

	if len(t.ServiceIDs) == 0 {
		return fmt.Errorf("render provider requires 'service_ids' in jua.yaml:\n  service_ids:\n    api: srv-xxxxxxxxxxxx\n    web: srv-xxxxxxxxxxxx")
	}

	services := opts.Services
	if len(services) == 0 {
		for name := range t.ServiceIDs {
			services = append(services, name)
		}
	}

	// Sync env vars first
	for _, svc := range services {
		svcID, ok := t.ServiceIDs[svc]
		if !ok {
			continue
		}
		if cfg.Env.SyncFrom != "" {
			p.pushEnvToService(c, svcID, cfg)
		}
	}

	// Trigger deploys
	for _, svc := range services {
		svcID, ok := t.ServiceIDs[svc]
		if !ok {
			pr.Log(fmt.Sprintf("  skipping %s — no service_id in jua.yaml", svc))
			continue
		}
		pr.Step("Deploying " + svc)
		start = time.Now()

		result, err := c.do("POST", "/services/"+svcID+"/deploys", map[string]string{
			"imageUrl": imageTag(cfg.Name, svc, version),
		})
		if err != nil {
			pr.StepFailed("Deploying "+svc, err)
			return fmt.Errorf("triggering render deploy for %s: %w", svc, err)
		}

		deployID := ""
		if m, ok := result.(map[string]interface{}); ok {
			deployID, _ = m["id"].(string)
		}

		if deployID != "" {
			if err := p.waitForDeploy(c, svcID, deployID, 120*time.Second); err != nil {
				pr.StepFailed("Deploying "+svc, err)
				return err
			}
		}
		pr.StepDone("Deploying "+svc, time.Since(start))
	}

	pr.Success("Deployed to Render")
	for _, svc := range services {
		pr.URL(svc, "https://"+subDomain(svc, t.Domain))
	}
	return nil
}

func (p *RenderProvider) pushEnvToService(c *renderClient, serviceID string, cfg *deploy.JuaConfig) {
	data, err := readAndFilterEnv(cfg)
	if err != nil {
		return
	}
	var envVars []map[string]string
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			envVars = append(envVars, map[string]string{"key": parts[0], "value": parts[1]})
		}
	}
	if len(envVars) > 0 {
		c.do("PUT", "/services/"+serviceID+"/env-vars", envVars)
	}
}

func readAndFilterEnv(cfg *deploy.JuaConfig) (string, error) {
	src := cfg.Env.SyncFrom
	if src == "" {
		src = ".env"
	}
	import_bytes, err := readFileBytes(src)
	if err != nil {
		return "", err
	}
	return filterEnv(string(import_bytes), cfg.Env.Exclude), nil
}

func (p *RenderProvider) waitForDeploy(c *renderClient, serviceID, deployID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result, err := c.do("GET", "/services/"+serviceID+"/deploys/"+deployID, nil)
		if err == nil {
			if m, ok := result.(map[string]interface{}); ok {
				status, _ := m["status"].(string)
				if status == "live" {
					return nil
				}
				if status == "failed" || status == "canceled" {
					return fmt.Errorf("render deployment failed: status=%s", status)
				}
			}
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("render deployment timed out after %s", timeout)
}

func (p *RenderProvider) Rollback(cfg *deploy.JuaConfig, targetName string, version string) error {
	opts := &DeployOptions{Version: version, Progress: &noopProgress{}}
	return p.Deploy(cfg, targetName, opts)
}

func (p *RenderProvider) Status(cfg *deploy.JuaConfig, targetName string) (*DeployStatus, error) {
	t, err := cfg.Target(targetName)
	if err != nil {
		return nil, err
	}
	c := &renderClient{token: t.APIToken}
	status := &DeployStatus{Version: cfg.Version}
	for svc, svcID := range t.ServiceIDs {
		result, err := c.do("GET", "/services/"+svcID, nil)
		if err != nil {
			continue
		}
		svcStatus := "unknown"
		if m, ok := result.(map[string]interface{}); ok {
			if s, ok := m["serviceDetails"].(map[string]interface{}); ok {
				svcStatus, _ = s["status"].(string)
			}
		}
		status.Services = append(status.Services, ServiceStatus{
			Name:   svc,
			Status: svcStatus,
			Health: "unknown",
			URL:    "https://" + subDomain(svc, t.Domain),
		})
	}
	return status, nil
}

func (p *RenderProvider) Logs(cfg *deploy.JuaConfig, targetName string, service string, follow bool, lines int) error {
	fmt.Println("Render logs: visit https://dashboard.render.com for live logs.")
	return nil
}

func (p *RenderProvider) Scale(cfg *deploy.JuaConfig, targetName string, service string, replicas int) error {
	return fmt.Errorf("render: scaling is plan-based — upgrade at https://render.com/pricing")
}
func (p *RenderProvider) EnvPush(cfg *deploy.JuaConfig, targetName string) error {
	t, err := cfg.Target(targetName)
	if err != nil {
		return err
	}
	c := &renderClient{token: t.APIToken}
	for svc, svcID := range t.ServiceIDs {
		p.pushEnvToService(c, svcID, cfg)
		fmt.Printf("  ✓ env pushed to %s\n", svc)
	}
	return nil
}
func (p *RenderProvider) EnvPull(cfg *deploy.JuaConfig, targetName string, dest string) error {
	return fmt.Errorf("render: env pull not supported via API")
}
func (p *RenderProvider) EnvList(cfg *deploy.JuaConfig, targetName string) (map[string]string, error) {
	return nil, fmt.Errorf("render: env list not supported via API")
}
func (p *RenderProvider) EnvSet(cfg *deploy.JuaConfig, targetName string, key, value string) error {
	return fmt.Errorf("render: use the Render dashboard to set environment variables")
}
func (p *RenderProvider) SSH(t *deploy.DeployTarget) error {
	return fmt.Errorf("render: SSH only available on paid plans — use the Render shell in the dashboard")
}
func (p *RenderProvider) Run(t *deploy.DeployTarget, command string) error {
	return fmt.Errorf("render: remote run not supported via API")
}
