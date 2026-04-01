package deploy

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// JuaConfig is the top-level structure of jua.yaml.
type JuaConfig struct {
	Name           string                    `yaml:"name"`
	Version        string                    `yaml:"version"`
	Deploy         map[string]*DeployTarget  `yaml:"deploy"`
	Services       map[string]*Service       `yaml:"services"`
	Infrastructure Infrastructure            `yaml:"infrastructure"`
	Env            EnvConfig                 `yaml:"env"`
	Multitenancy   MultitenancyConfig        `yaml:"multitenancy"`
	Strategy       DeployStrategy            `yaml:"strategy"`
}

// DeployTarget describes how to deploy to a specific environment.
type DeployTarget struct {
	Provider    string            `yaml:"provider"`
	Host        string            `yaml:"host"`
	SSHUser     string            `yaml:"ssh_user"`
	SSHKey      string            `yaml:"ssh_key"`
	Domain      string            `yaml:"domain"`
	SSL         bool              `yaml:"ssl"`
	APIToken    string            `yaml:"api_token"`
	ProjectID   string            `yaml:"project_id"`
	TeamID      string            `yaml:"team_id"`
	Region      string            `yaml:"region"`
	Org         string            `yaml:"org"`
	Environment string            `yaml:"environment"`
	ServiceIDs  map[string]string `yaml:"service_ids"`
}

// Service describes a deployable service (api, web, admin, etc.).
type Service struct {
	Build       string `yaml:"build"`
	Port        int    `yaml:"port"`
	HealthCheck string `yaml:"health_check"`
	Replicas    int    `yaml:"replicas"`
	Memory      string `yaml:"memory"`
	EnvFile     string `yaml:"env_file"`
}

// Infrastructure describes managed infrastructure dependencies.
type Infrastructure struct {
	Postgres PostgresConfig `yaml:"postgres"`
	Redis    RedisConfig    `yaml:"redis"`
	Minio    MinioConfig    `yaml:"minio"`
}

// PostgresConfig holds Postgres settings.
type PostgresConfig struct {
	Version string `yaml:"version"`
	Size    string `yaml:"size"`
}

// RedisConfig holds Redis settings.
type RedisConfig struct {
	Version string `yaml:"version"`
}

// MinioConfig holds MinIO/S3 settings.
type MinioConfig struct {
	Enabled bool `yaml:"enabled"`
}

// EnvConfig controls how environment variables are synced.
type EnvConfig struct {
	SyncFrom string   `yaml:"sync_from"`
	Exclude  []string `yaml:"exclude"`
}

// MultitenancyConfig controls multi-tenant deployment behaviour.
type MultitenancyConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Routing     string `yaml:"routing"`
	WildcardSSL bool   `yaml:"wildcard_ssl"`
}

// DeployStrategy controls how deployments are rolled out.
type DeployStrategy struct {
	Type               string `yaml:"type"`
	HealthCheckTimeout string `yaml:"health_check_timeout"`
	RollbackOnFailure  bool   `yaml:"rollback_on_failure"`
}

// LoadConfig reads jua.yaml from the given path.
// If path is empty it falls back to ./jua.yaml.
func LoadConfig(path string) (*JuaConfig, error) {
	if path == "" {
		path = "jua.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"jua.yaml not found at %s\n\nRun 'jua deploy:setup' to create one, or run this command from your project root",
				path,
			)
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var cfg JuaConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// validateConfig checks that required fields are present.
func validateConfig(cfg *JuaConfig) error {
	if cfg.Name == "" {
		return fmt.Errorf("jua.yaml: 'name' is required")
	}
	if len(cfg.Deploy) == 0 {
		return fmt.Errorf("jua.yaml: at least one deploy target is required under 'deploy:'")
	}
	for env, target := range cfg.Deploy {
		if target.Provider == "" {
			return fmt.Errorf("jua.yaml: deploy.%s.provider is required", env)
		}
	}
	return nil
}

// Target returns the named deploy target, defaulting to "production".
func (c *JuaConfig) Target(name string) (*DeployTarget, error) {
	if name == "" {
		name = "production"
	}
	t, ok := c.Deploy[name]
	if !ok {
		keys := make([]string, 0, len(c.Deploy))
		for k := range c.Deploy {
			keys = append(keys, k)
		}
		return nil, fmt.Errorf("deploy target %q not found in jua.yaml (available: %v)", name, keys)
	}
	return t, nil
}
