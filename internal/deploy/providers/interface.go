package providers

import (
	"time"

	"github.com/katuramuh/jua/v3/internal/deploy"
)

// DeployProvider is the interface all deployment providers must implement.
type DeployProvider interface {
	// Validate checks credentials and connectivity before deploying.
	Validate(target *deploy.DeployTarget) error

	// Setup provisions a fresh server (Docker, Traefik, firewall, etc.).
	Setup(target *deploy.DeployTarget, opts *SetupOptions) error

	// Deploy deploys the application using the given config and target name.
	Deploy(cfg *deploy.JuaConfig, targetName string, opts *DeployOptions) error

	// Rollback rolls back to a previous version.
	Rollback(cfg *deploy.JuaConfig, targetName string, version string) error

	// Status returns the current deployment status.
	Status(cfg *deploy.JuaConfig, targetName string) (*DeployStatus, error)

	// Logs streams logs from a service. When follow is true, it tails continuously.
	Logs(cfg *deploy.JuaConfig, targetName string, service string, follow bool, lines int) error

	// Scale changes the replica count for a service.
	Scale(cfg *deploy.JuaConfig, targetName string, service string, replicas int) error

	// EnvPush syncs the local .env file to the remote server.
	EnvPush(cfg *deploy.JuaConfig, targetName string) error

	// EnvPull pulls the remote .env to a local file.
	EnvPull(cfg *deploy.JuaConfig, targetName string, dest string) error

	// EnvList lists all remote environment variables (values masked).
	EnvList(cfg *deploy.JuaConfig, targetName string) (map[string]string, error)

	// EnvSet sets a single environment variable on the server.
	EnvSet(cfg *deploy.JuaConfig, targetName string, key, value string) error

	// SSH opens an interactive SSH session to the server.
	SSH(target *deploy.DeployTarget) error

	// Run executes a command on the remote server and streams output.
	Run(target *deploy.DeployTarget, command string) error
}

// DeployOptions controls deployment behaviour.
type DeployOptions struct {
	Version  string
	Services []string // empty = all services
	DryRun   bool
	Force    bool
	Progress ProgressReporter
}

// SetupOptions controls VPS provisioning.
type SetupOptions struct {
	DNSProvider     string
	CloudflareToken string
	Email           string // for Let's Encrypt
	Progress        ProgressReporter
}

// DeployStatus is the overall deployment status returned by Status().
type DeployStatus struct {
	Services  []ServiceStatus
	URL       string
	Version   string
	UpdatedAt time.Time
}

// ServiceStatus holds the runtime state of a single service.
type ServiceStatus struct {
	Name     string
	Status   string // running | stopped | error | starting
	Replicas int
	Health   string // healthy | unhealthy | unknown
	URL      string
	CPU      string
	Memory   string
}

// ProgressReporter drives terminal progress output during deployments.
type ProgressReporter interface {
	// Step announces the start of a named step.
	Step(name string)
	// StepDone marks a step as completed with optional duration.
	StepDone(name string, d time.Duration)
	// StepFailed marks a step as failed.
	StepFailed(name string, err error)
	// SubStep announces progress on a named sub-task (service build, etc.).
	SubStep(parent, name string, percent int, done bool)
	// Log prints an informational line.
	Log(message string)
	// Success prints a success summary.
	Success(message string)
	// Error prints an error line.
	Error(message string)
	// URL prints a service URL.
	URL(label, url string)
	// Hint prints a follow-up command hint.
	Hint(message string)
}
