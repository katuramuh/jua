package providers

// NOTE: Requires a Dokploy instance with API access.
// API docs: https://dokploy.com/docs/api

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

type DokployProvider struct{}

func (p *DokployProvider) client(t *deploy.DeployTarget) *dokployClient {
	base := fmt.Sprintf("https://%s", t.Host)
	if !strings.HasPrefix(t.Host, "http") {
		base = "https://" + t.Host
	}
	return &dokployClient{base: base, token: t.APIToken}
}

type dokployClient struct {
	base  string
	token string
}

func (c *dokployClient) do(method, path string, body interface{}) (map[string]interface{}, error) {
	var buf io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, buf)
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
		return result, fmt.Errorf("dokploy API error %d: %v", resp.StatusCode, result["message"])
	}
	return result, nil
}

func (p *DokployProvider) Validate(t *deploy.DeployTarget) error {
	if t.APIToken == "" {
		return fmt.Errorf("dokploy provider requires 'api_token' in jua.yaml")
	}
	if t.Host == "" {
		return fmt.Errorf("dokploy provider requires 'host' (your Dokploy instance URL) in jua.yaml")
	}
	c := p.client(t)
	_, err := c.do("GET", "/api/project.all", nil)
	return err
}

func (p *DokployProvider) Setup(t *deploy.DeployTarget, opts *SetupOptions) error {
	return fmt.Errorf("dokploy: Setup is handled by your Dokploy instance — install at https://dokploy.com")
}

func (p *DokployProvider) Deploy(cfg *deploy.JuaConfig, targetName string, opts *DeployOptions) error {
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

	pr.Step("Connecting to Dokploy")
	start := time.Now()
	if err := p.Validate(t); err != nil {
		pr.StepFailed("Connecting to Dokploy", err)
		return err
	}
	pr.StepDone("Connecting to Dokploy", time.Since(start))

	// Get or create project
	pr.Step("Finding project " + cfg.Name)
	start = time.Now()
	projectID, err := p.ensureProject(c, cfg.Name)
	if err != nil {
		pr.StepFailed("Finding project", err)
		return err
	}
	pr.StepDone("Finding project "+cfg.Name, time.Since(start))

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

		appID, err := p.ensureApplication(c, projectID, cfg.Name+"-"+svc, imageName, domain, svcCfg.Port)
		if err != nil {
			pr.StepFailed("Deploying "+svc, err)
			return fmt.Errorf("ensuring application %s: %w", svc, err)
		}

		// Trigger deployment
		_, err = c.do("POST", "/api/application.deploy", map[string]string{"applicationId": appID})
		if err != nil {
			pr.StepFailed("Deploying "+svc, err)
			return fmt.Errorf("triggering deploy for %s: %w", svc, err)
		}

		// Poll until done
		if err := p.waitForDeploy(c, appID, 120*time.Second); err != nil {
			pr.StepFailed("Deploying "+svc, err)
			return err
		}
		pr.StepDone("Deploying "+svc, time.Since(start))
	}

	pr.Success("Deployed to Dokploy")
	for _, svc := range services {
		pr.URL(svc, "https://"+subDomain(svc, t.Domain))
	}
	return nil
}

func (p *DokployProvider) ensureProject(c *dokployClient, name string) (string, error) {
	result, err := c.do("GET", "/api/project.all", nil)
	if err != nil {
		return "", err
	}
	if projects, ok := result["projects"].([]interface{}); ok {
		for _, proj := range projects {
			if m, ok := proj.(map[string]interface{}); ok {
				if m["name"] == name {
					return m["projectId"].(string), nil
				}
			}
		}
	}
	// Create project
	created, err := c.do("POST", "/api/project.create", map[string]string{"name": name})
	if err != nil {
		return "", err
	}
	return created["projectId"].(string), nil
}

func (p *DokployProvider) ensureApplication(c *dokployClient, projectID, name, image, domain string, port int) (string, error) {
	result, err := c.do("GET", "/api/application.all", nil)
	if err != nil {
		return "", err
	}
	if apps, ok := result["applications"].([]interface{}); ok {
		for _, app := range apps {
			if m, ok := app.(map[string]interface{}); ok {
				if m["name"] == name {
					id := m["applicationId"].(string)
					// Update image
					c.do("POST", "/api/application.update", map[string]interface{}{
						"applicationId": id,
						"dockerImage":   image,
					})
					return id, nil
				}
			}
		}
	}
	created, err := c.do("POST", "/api/application.create", map[string]interface{}{
		"name":        name,
		"projectId":   projectID,
		"dockerImage": image,
		"appPort":     port,
		"domains":     []map[string]string{{"host": domain, "https": "true"}},
	})
	if err != nil {
		return "", err
	}
	return created["applicationId"].(string), nil
}

func (p *DokployProvider) waitForDeploy(c *dokployClient, appID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result, err := c.do("GET", "/api/application.one?applicationId="+appID, nil)
		if err == nil {
			if status, ok := result["applicationStatus"].(string); ok {
				if status == "done" {
					return nil
				}
				if status == "error" {
					return fmt.Errorf("deployment failed on Dokploy")
				}
			}
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("deployment timed out after %s", timeout)
}

func (p *DokployProvider) Rollback(cfg *deploy.JuaConfig, targetName string, version string) error {
	opts := &DeployOptions{Version: version, Progress: &noopProgress{}}
	return p.Deploy(cfg, targetName, opts)
}

func (p *DokployProvider) Status(cfg *deploy.JuaConfig, targetName string) (*DeployStatus, error) {
	t, err := cfg.Target(targetName)
	if err != nil {
		return nil, err
	}
	c := p.client(t)
	result, err := c.do("GET", "/api/application.all", nil)
	if err != nil {
		return nil, err
	}
	status := &DeployStatus{Version: cfg.Version}
	if apps, ok := result["applications"].([]interface{}); ok {
		for _, app := range apps {
			if m, ok := app.(map[string]interface{}); ok {
				name, _ := m["name"].(string)
				if !strings.HasPrefix(name, cfg.Name+"-") {
					continue
				}
				svcName := strings.TrimPrefix(name, cfg.Name+"-")
				appStatus, _ := m["applicationStatus"].(string)
				status.Services = append(status.Services, ServiceStatus{
					Name:   svcName,
					Status: appStatus,
					Health: "unknown",
					URL:    "https://" + subDomain(svcName, t.Domain),
				})
			}
		}
	}
	return status, nil
}

func (p *DokployProvider) Logs(cfg *deploy.JuaConfig, targetName string, service string, follow bool, lines int) error {
	t, err := cfg.Target(targetName)
	if err != nil {
		return err
	}
	c := p.client(t)
	result, err := c.do("GET", "/api/application.readDeployLogs?appName="+cfg.Name+"-"+service, nil)
	if err != nil {
		return err
	}
	if logs, ok := result["logs"].(string); ok {
		fmt.Println(logs)
	}
	return nil
}

func (p *DokployProvider) Scale(cfg *deploy.JuaConfig, targetName string, service string, replicas int) error {
	return fmt.Errorf("scaling not supported via Dokploy API — use the Dokploy web UI")
}

func (p *DokployProvider) EnvPush(cfg *deploy.JuaConfig, targetName string) error {
	return fmt.Errorf("dokploy: use the Dokploy web UI to manage environment variables, or set them via jua.yaml")
}

func (p *DokployProvider) EnvPull(cfg *deploy.JuaConfig, targetName string, dest string) error {
	return fmt.Errorf("dokploy: environment variable pull not supported via API")
}

func (p *DokployProvider) EnvList(cfg *deploy.JuaConfig, targetName string) (map[string]string, error) {
	return nil, fmt.Errorf("dokploy: environment variable list not supported via API")
}

func (p *DokployProvider) EnvSet(cfg *deploy.JuaConfig, targetName string, key, value string) error {
	return fmt.Errorf("dokploy: use the Dokploy web UI to set environment variables")
}

func (p *DokployProvider) SSH(t *deploy.DeployTarget) error {
	return fmt.Errorf("dokploy: SSH not available — use the Dokploy web terminal")
}

func (p *DokployProvider) Run(t *deploy.DeployTarget, command string) error {
	return fmt.Errorf("dokploy: remote run not supported via API")
}
