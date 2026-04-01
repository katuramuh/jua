package providers

// NOTE: Requires a Railway account with an API token.
// API docs: https://docs.railway.app/reference/public-api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/katuramuh/jua/v3/internal/deploy"
)

type RailwayProvider struct{}

const railwayGraphQL = "https://backboard.railway.app/graphql/v2"

func railwayQuery(token, query string, variables map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{"query": query, "variables": variables}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", railwayGraphQL, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if errs, ok := result["errors"]; ok {
		return result, fmt.Errorf("railway API error: %v", errs)
	}
	return result, nil
}

func (p *RailwayProvider) Validate(t *deploy.DeployTarget) error {
	if t.APIToken == "" {
		return fmt.Errorf("railway provider requires 'api_token' in jua.yaml")
	}
	if t.ProjectID == "" {
		return fmt.Errorf("railway provider requires 'project_id' in jua.yaml")
	}
	_, err := railwayQuery(t.APIToken, `query { me { name } }`, nil)
	return err
}

func (p *RailwayProvider) Setup(t *deploy.DeployTarget, opts *SetupOptions) error {
	return fmt.Errorf("railway: infrastructure is managed by Railway — create a project at https://railway.app")
}

func (p *RailwayProvider) Deploy(cfg *deploy.JuaConfig, targetName string, opts *DeployOptions) error {
	t, err := cfg.Target(targetName)
	if err != nil {
		return err
	}
	pr := opts.Progress

	version := opts.Version
	if version == "" {
		version = cfg.Version
	}

	pr.Step("Connecting to Railway")
	start := time.Now()
	if err := p.Validate(t); err != nil {
		pr.StepFailed("Connecting to Railway", err)
		return err
	}
	pr.StepDone("Connecting to Railway", time.Since(start))

	services := opts.Services
	if len(services) == 0 {
		for name := range cfg.Services {
			services = append(services, name)
		}
	}

	for _, svc := range services {
		imageName := imageTag(cfg.Name, svc, version)
		pr.Step("Deploying " + svc)
		start = time.Now()

		// Find or create service
		serviceID, err := p.ensureService(t, cfg.Name+"-"+svc, imageName)
		if err != nil {
			pr.StepFailed("Deploying "+svc, err)
			return fmt.Errorf("ensuring railway service %s: %w", svc, err)
		}

		// Trigger deployment
		mutation := `mutation DeploymentCreate($input: DeploymentCreateInput!) {
			deploymentCreate(input: $input) { id status }
		}`
		result, err := railwayQuery(t.APIToken, mutation, map[string]interface{}{
			"input": map[string]interface{}{
				"serviceId":   serviceID,
				"projectId":   t.ProjectID,
				"dockerImage": imageName,
			},
		})
		if err != nil {
			pr.StepFailed("Deploying "+svc, err)
			return fmt.Errorf("triggering railway deploy for %s: %w", svc, err)
		}

		data, _ := result["data"].(map[string]interface{})
		if data == nil {
			pr.StepFailed("Deploying "+svc, fmt.Errorf("no data in response"))
			return fmt.Errorf("railway: no deployment data returned for %s", svc)
		}

		if deployInfo, ok := data["deploymentCreate"].(map[string]interface{}); ok {
			deployID, _ := deployInfo["id"].(string)
			if err := p.waitForDeploy(t, deployID, 120*time.Second); err != nil {
				pr.StepFailed("Deploying "+svc, err)
				return err
			}
		}

		pr.StepDone("Deploying "+svc, time.Since(start))
	}

	pr.Success("Deployed to Railway")
	for _, svc := range services {
		pr.URL(svc, "https://"+subDomain(svc, t.Domain))
	}
	return nil
}

func (p *RailwayProvider) ensureService(t *deploy.DeployTarget, name, image string) (string, error) {
	// Check existing services
	q := `query Services($projectId: String!) {
		project(id: $projectId) { services { edges { node { id name } } } }
	}`
	result, err := railwayQuery(t.APIToken, q, map[string]interface{}{"projectId": t.ProjectID})
	if err != nil {
		return "", err
	}
	if data, ok := result["data"].(map[string]interface{}); ok {
		if project, ok := data["project"].(map[string]interface{}); ok {
			if services, ok := project["services"].(map[string]interface{}); ok {
				if edges, ok := services["edges"].([]interface{}); ok {
					for _, edge := range edges {
						if e, ok := edge.(map[string]interface{}); ok {
							if node, ok := e["node"].(map[string]interface{}); ok {
								if node["name"] == name {
									return node["id"].(string), nil
								}
							}
						}
					}
				}
			}
		}
	}

	// Create service
	mutation := `mutation ServiceCreate($input: ServiceCreateInput!) {
		serviceCreate(input: $input) { id }
	}`
	created, err := railwayQuery(t.APIToken, mutation, map[string]interface{}{
		"input": map[string]interface{}{
			"projectId":   t.ProjectID,
			"name":        name,
			"source":      map[string]string{"image": image},
		},
	})
	if err != nil {
		return "", err
	}
	if data, ok := created["data"].(map[string]interface{}); ok {
		if sc, ok := data["serviceCreate"].(map[string]interface{}); ok {
			return sc["id"].(string), nil
		}
	}
	return "", fmt.Errorf("railway: could not create service %s", name)
}

func (p *RailwayProvider) waitForDeploy(t *deploy.DeployTarget, deployID string, timeout time.Duration) error {
	q := `query DeploymentStatus($id: String!) {
		deployment(id: $id) { status }
	}`
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result, err := railwayQuery(t.APIToken, q, map[string]interface{}{"id": deployID})
		if err == nil {
			if data, ok := result["data"].(map[string]interface{}); ok {
				if dep, ok := data["deployment"].(map[string]interface{}); ok {
					status, _ := dep["status"].(string)
					if status == "SUCCESS" {
						return nil
					}
					if status == "FAILED" || status == "CRASHED" {
						return fmt.Errorf("railway deployment failed: status=%s", status)
					}
				}
			}
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("railway deployment timed out after %s", timeout)
}

func (p *RailwayProvider) Rollback(cfg *deploy.JuaConfig, targetName string, version string) error {
	opts := &DeployOptions{Version: version, Progress: &noopProgress{}}
	return p.Deploy(cfg, targetName, opts)
}

func (p *RailwayProvider) Status(cfg *deploy.JuaConfig, targetName string) (*DeployStatus, error) {
	t, err := cfg.Target(targetName)
	if err != nil {
		return nil, err
	}
	q := `query Services($projectId: String!) {
		project(id: $projectId) { services { edges { node { id name } } } }
	}`
	result, err := railwayQuery(t.APIToken, q, map[string]interface{}{"projectId": t.ProjectID})
	if err != nil {
		return nil, err
	}
	status := &DeployStatus{Version: cfg.Version}
	if data, ok := result["data"].(map[string]interface{}); ok {
		if project, ok := data["project"].(map[string]interface{}); ok {
			if services, ok := project["services"].(map[string]interface{}); ok {
				if edges, ok := services["edges"].([]interface{}); ok {
					for _, edge := range edges {
						if e, ok := edge.(map[string]interface{}); ok {
							if node, ok := e["node"].(map[string]interface{}); ok {
								name, _ := node["name"].(string)
								if !strings.HasPrefix(name, cfg.Name+"-") {
									continue
								}
								svcName := strings.TrimPrefix(name, cfg.Name+"-")
								status.Services = append(status.Services, ServiceStatus{
									Name:   svcName,
									Status: "running",
									Health: "unknown",
									URL:    "https://" + subDomain(svcName, t.Domain),
								})
							}
						}
					}
				}
			}
		}
	}
	return status, nil
}

func (p *RailwayProvider) Logs(cfg *deploy.JuaConfig, targetName string, service string, follow bool, lines int) error {
	fmt.Println("Railway logs: use 'railway logs' CLI or the Railway dashboard for live logs.")
	fmt.Println("Install Railway CLI: npm install -g @railway/cli")
	return nil
}

func (p *RailwayProvider) Scale(cfg *deploy.JuaConfig, targetName string, service string, replicas int) error {
	return fmt.Errorf("railway: scaling is managed automatically — upgrade your plan at https://railway.app/pricing")
}
func (p *RailwayProvider) EnvPush(cfg *deploy.JuaConfig, targetName string) error {
	return fmt.Errorf("railway: use 'railway variables set KEY=VALUE' or the Railway dashboard")
}
func (p *RailwayProvider) EnvPull(cfg *deploy.JuaConfig, targetName string, dest string) error {
	return fmt.Errorf("railway: use 'railway variables' CLI command")
}
func (p *RailwayProvider) EnvList(cfg *deploy.JuaConfig, targetName string) (map[string]string, error) {
	return nil, fmt.Errorf("railway: use 'railway variables' CLI command")
}
func (p *RailwayProvider) EnvSet(cfg *deploy.JuaConfig, targetName string, key, value string) error {
	return fmt.Errorf("railway: use 'railway variables set %s=%s'", key, value)
}
func (p *RailwayProvider) SSH(t *deploy.DeployTarget) error {
	return fmt.Errorf("railway: SSH not available — use 'railway run' for one-off commands")
}
func (p *RailwayProvider) Run(t *deploy.DeployTarget, command string) error {
	fmt.Printf("Hint: railway run %s\n", command)
	return fmt.Errorf("railway: remote run not supported via API — install the Railway CLI")
}
