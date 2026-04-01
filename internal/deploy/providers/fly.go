package providers

// NOTE: Requires a Fly.io account and flyctl installed, or an API token for Machines API.
// API docs: https://fly.io/docs/machines/api/

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/katuramuh/jua/v3/internal/deploy"
)

type FlyProvider struct{}

const flyMachinesBase = "https://api.machines.dev/v1"

type flyClient struct{ token string }

func (c *flyClient) do(method, path string, body interface{}) (interface{}, error) {
	var buf io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, flyMachinesBase+path, buf)
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
		return result, fmt.Errorf("fly API error %d", resp.StatusCode)
	}
	return result, nil
}

func (p *FlyProvider) Validate(t *deploy.DeployTarget) error {
	if t.APIToken == "" {
		return fmt.Errorf("fly provider requires 'api_token' in jua.yaml\nGet one at: https://fly.io/user/personal_access_tokens")
	}
	c := &flyClient{token: t.APIToken}
	_, err := c.do("GET", "/apps", nil)
	return err
}

func (p *FlyProvider) Setup(t *deploy.DeployTarget, opts *SetupOptions) error {
	return fmt.Errorf("fly: run 'flyctl launch' in each service directory to create Fly apps, then add api_token to jua.yaml")
}

func (p *FlyProvider) Deploy(cfg *deploy.JuaConfig, targetName string, opts *DeployOptions) error {
	t, err := cfg.Target(targetName)
	if err != nil {
		return err
	}
	pr := opts.Progress

	version := opts.Version
	if version == "" {
		version = cfg.Version
	}

	// Prefer flyctl if installed
	flyctlPath, flyctlErr := exec.LookPath("flyctl")
	if flyctlErr == nil {
		return p.deployWithFlyctl(flyctlPath, cfg, t, version, opts.Services, pr)
	}

	// Fall back to Machines API
	return p.deployWithMachinesAPI(cfg, t, version, opts.Services, pr)
}

func (p *FlyProvider) deployWithFlyctl(flyctlPath string, cfg *deploy.JuaConfig, t *deploy.DeployTarget, version string, services []string, pr ProgressReporter) error {
	pr.Step("Deploying via flyctl")
	fmt.Println()

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
		appName := cfg.Name + "-" + svc
		imageName := imageTag(cfg.Name, svc, version)

		pr.SubStep("Deploying", svc, 0, false)
		cmd := exec.Command(flyctlPath, "deploy",
			"--image", imageName,
			"--app", appName,
			"--region", orDefault(t.Region, "jnb"), // Johannesburg default for Africa
		)
		cmd.Env = append(os.Environ(), "FLY_API_TOKEN="+t.APIToken)
		cmd.Dir = svcCfg.Build
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("flyctl deploy %s: %w", svc, err)
		}
		pr.SubStep("Deploying", svc, 100, true)
	}

	pr.Success("Deployed to Fly.io")
	for _, svc := range services {
		pr.URL(svc, "https://"+subDomain(svc, t.Domain))
	}
	return nil
}

func (p *FlyProvider) deployWithMachinesAPI(cfg *deploy.JuaConfig, t *deploy.DeployTarget, version string, services []string, pr ProgressReporter) error {
	pr.Step("Connecting to Fly.io Machines API")
	start := time.Now()
	if err := p.Validate(t); err != nil {
		pr.StepFailed("Connecting to Fly.io", err)
		return err
	}
	pr.StepDone("Connecting to Fly.io Machines API", time.Since(start))

	c := &flyClient{token: t.APIToken}

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
		appName := cfg.Name + "-" + svc
		imageName := imageTag(cfg.Name, svc, version)
		region := orDefault(t.Region, "jnb")

		pr.Step("Deploying " + svc)
		start = time.Now()

		// Ensure app exists
		p.ensureApp(c, appName, t.Org, region)

		// Create machine
		machineConfig := map[string]interface{}{
			"region": region,
			"config": map[string]interface{}{
				"image": imageName,
				"services": []map[string]interface{}{
					{
						"internal_port": svcCfg.Port,
						"protocol":      "tcp",
						"ports": []map[string]interface{}{
							{"port": 443, "handlers": []string{"tls", "http"}},
							{"port": 80, "handlers": []string{"http"}},
						},
					},
				},
			},
		}

		_, err := c.do("POST", "/apps/"+appName+"/machines", machineConfig)
		if err != nil {
			pr.StepFailed("Deploying "+svc, err)
			return fmt.Errorf("creating fly machine for %s: %w", svc, err)
		}
		pr.StepDone("Deploying "+svc, time.Since(start))
	}

	pr.Success("Deployed to Fly.io")
	for _, svc := range services {
		pr.URL(svc, "https://"+subDomain(svc, t.Domain))
	}
	return nil
}

func (p *FlyProvider) ensureApp(c *flyClient, appName, org, region string) {
	c.do("POST", "/apps", map[string]interface{}{
		"app_name":    appName,
		"org_slug":    org,
		"network":     "default",
	})
}

func (p *FlyProvider) Rollback(cfg *deploy.JuaConfig, targetName string, version string) error {
	opts := &DeployOptions{Version: version, Progress: &noopProgress{}}
	return p.Deploy(cfg, targetName, opts)
}

func (p *FlyProvider) Status(cfg *deploy.JuaConfig, targetName string) (*DeployStatus, error) {
	t, err := cfg.Target(targetName)
	if err != nil {
		return nil, err
	}
	c := &flyClient{token: t.APIToken}
	status := &DeployStatus{Version: cfg.Version}
	for svc := range cfg.Services {
		appName := cfg.Name + "-" + svc
		result, err := c.do("GET", "/apps/"+appName+"/machines", nil)
		if err != nil {
			continue
		}
		machineStatus := "unknown"
		if machines, ok := result.([]interface{}); ok && len(machines) > 0 {
			if m, ok := machines[0].(map[string]interface{}); ok {
				machineStatus, _ = m["state"].(string)
			}
		}
		status.Services = append(status.Services, ServiceStatus{
			Name:   svc,
			Status: machineStatus,
			Health: "unknown",
			URL:    "https://" + subDomain(svc, t.Domain),
		})
	}
	return status, nil
}

func (p *FlyProvider) Logs(cfg *deploy.JuaConfig, targetName string, service string, follow bool, lines int) error {
	t, err := cfg.Target(targetName)
	if err != nil {
		return err
	}
	flyctlPath, err := exec.LookPath("flyctl")
	if err != nil {
		return fmt.Errorf("flyctl not installed — install from https://fly.io/docs/hands-on/install-flyctl/")
	}
	appName := cfg.Name + "-" + service
	args := []string{"logs", "--app", appName}
	if follow {
		args = append(args, "--follow")
	}
	cmd := exec.Command(flyctlPath, args...)
	cmd.Env = append(os.Environ(), "FLY_API_TOKEN="+t.APIToken)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (p *FlyProvider) Scale(cfg *deploy.JuaConfig, targetName string, service string, replicas int) error {
	t, err := cfg.Target(targetName)
	if err != nil {
		return err
	}
	flyctlPath, err := exec.LookPath("flyctl")
	if err != nil {
		return fmt.Errorf("flyctl not installed")
	}
	appName := cfg.Name + "-" + service
	cmd := exec.Command(flyctlPath, "scale", "count", fmt.Sprintf("%d", replicas), "--app", appName)
	cmd.Env = append(os.Environ(), "FLY_API_TOKEN="+t.APIToken)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (p *FlyProvider) EnvPush(cfg *deploy.JuaConfig, targetName string) error {
	t, err := cfg.Target(targetName)
	if err != nil {
		return err
	}
	flyctlPath, err := exec.LookPath("flyctl")
	if err != nil {
		return fmt.Errorf("flyctl not installed")
	}
	src := cfg.Env.SyncFrom
	if src == "" {
		src = ".env"
	}
	for svc := range cfg.Services {
		appName := cfg.Name + "-" + svc
		cmd := exec.Command(flyctlPath, "secrets", "import", "--app", appName)
		cmd.Env = append(os.Environ(), "FLY_API_TOKEN="+t.APIToken)
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		cmd.Stdin = f
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			f.Close()
			return fmt.Errorf("pushing env to %s: %w", svc, err)
		}
		f.Close()
		fmt.Printf("  ✓ env pushed to %s\n", svc)
	}
	return nil
}

func (p *FlyProvider) EnvPull(cfg *deploy.JuaConfig, targetName string, dest string) error {
	return fmt.Errorf("fly: use 'flyctl secrets list --app APP_NAME' to view secrets")
}

func (p *FlyProvider) EnvList(cfg *deploy.JuaConfig, targetName string) (map[string]string, error) {
	return nil, fmt.Errorf("fly: use 'flyctl secrets list --app APP_NAME'")
}

func (p *FlyProvider) EnvSet(cfg *deploy.JuaConfig, targetName string, key, value string) error {
	t, err := cfg.Target(targetName)
	if err != nil {
		return err
	}
	flyctlPath, err := exec.LookPath("flyctl")
	if err != nil {
		return fmt.Errorf("flyctl not installed")
	}
	for svc := range cfg.Services {
		appName := cfg.Name + "-" + svc
		cmd := exec.Command(flyctlPath, "secrets", "set", key+"="+value, "--app", appName)
		cmd.Env = append(os.Environ(), "FLY_API_TOKEN="+t.APIToken)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}
	return nil
}

func (p *FlyProvider) SSH(t *deploy.DeployTarget) error {
	flyctlPath, err := exec.LookPath("flyctl")
	if err != nil {
		return fmt.Errorf("flyctl not installed — install from https://fly.io/docs/hands-on/install-flyctl/")
	}
	fmt.Println("Use: flyctl ssh console --app APP_NAME")
	_ = flyctlPath
	return nil
}

func (p *FlyProvider) Run(t *deploy.DeployTarget, command string) error {
	flyctlPath, err := exec.LookPath("flyctl")
	if err != nil {
		return fmt.Errorf("flyctl not installed")
	}
	fmt.Printf("Hint: flyctl ssh console --app APP_NAME -C '%s'\n", command)
	_ = flyctlPath
	return nil
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
