package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/huh"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/katuramuh/jua/v3/internal/deploy"
	"github.com/katuramuh/jua/v3/internal/deploy/providers"
)

// deployCmd is the root deploy command with subcommands.
// It replaces the simple deployCmd() in main.go.
func newDeployCmd() *cobra.Command {
	var (
		env     string
		service string
		ver     string
		dryRun  bool
		force   bool
		cfg     string
	)

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy your Jua app to any provider",
		Long: `Deploy your Jua app to VPS, Dokploy, Coolify, Railway, Render, or Fly.io.

Examples:
  jua deploy                        Deploy to production
  jua deploy --env staging          Deploy to staging
  jua deploy --service api          Deploy only the API service
  jua deploy --version v1.2.0       Deploy a specific version
  jua deploy --dry-run              Preview deployment without executing`,
		RunE: func(cmd *cobra.Command, args []string) error {
			printLogo()

			juaCfg, err := deploy.LoadConfig(cfg)
			if err != nil {
				color.Red("\n  %v\n", err)
				return err
			}

			target, err := juaCfg.Target(env)
			if err != nil {
				color.Red("\n  %v\n", err)
				return err
			}

			provider, err := providers.Get(target.Provider)
			if err != nil {
				color.Red("\n  %v\n", err)
				return err
			}

			pr := deploy.NewProgress(juaCfg.Name, orVer(ver, juaCfg.Version), env)

			var services []string
			if service != "" {
				services = []string{service}
			}

			opts := &providers.DeployOptions{
				Version:  ver,
				Services: services,
				DryRun:   dryRun,
				Force:    force,
				Progress: pr,
			}

			if dryRun {
				color.Yellow("\n  [dry-run] Would deploy %s → %s via %s\n", juaCfg.Name, env, target.Provider)
				return nil
			}

			if err := provider.Deploy(juaCfg, env, opts); err != nil {
				color.Red("\n  Deploy failed: %v\n", err)
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&env, "env", "production", "Deployment environment (matches deploy key in jua.yaml)")
	cmd.Flags().StringVar(&service, "service", "", "Deploy only this service (api|web|admin)")
	cmd.Flags().StringVar(&ver, "version", "", "Version tag to deploy (default: version from jua.yaml)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview deployment without executing")
	cmd.Flags().BoolVar(&force, "force", false, "Force deploy even if health checks fail")
	cmd.Flags().StringVar(&cfg, "config", "jua.yaml", "Path to jua.yaml")

	// Subcommands
	cmd.AddCommand(deploySetupCmd())

	return cmd
}

func deploySetupCmd() *cobra.Command {
	var cfgPath string

	return &cobra.Command{
		Use:   "setup",
		Short: "Provision a fresh VPS for Jua deployment",
		Long:  "Interactive wizard that installs Docker, Traefik, and configures SSL on a fresh Linux server.",
		RunE: func(cmd *cobra.Command, args []string) error {
			printLogo()

			juaCfg, err := deploy.LoadConfig(cfgPath)
			if err != nil {
				color.Red("\n  %v\n", err)
				return err
			}

			target, err := juaCfg.Target("production")
			if err != nil {
				// Interactive wizard — ask for values
				target = &deploy.DeployTarget{Provider: "vps"}
			}

			// Prompt for missing values
			var host, sshUser, sshKey, domain, email string
			var wildcardSSL bool

			host = target.Host
			sshUser = orStr(target.SSHUser, "root")
			sshKey = orStr(target.SSHKey, "~/.ssh/id_rsa")
			domain = target.Domain
			email = "admin@" + domain
			wildcardSSL = juaCfg.Multitenancy.WildcardSSL

			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().Title("VPS IP address").Value(&host).Placeholder("165.22.100.1"),
					huh.NewInput().Title("SSH username").Value(&sshUser),
					huh.NewInput().Title("SSH key path").Value(&sshKey),
					huh.NewInput().Title("Domain name").Value(&domain).Placeholder("yourdomain.com"),
					huh.NewInput().Title("Email (for SSL certificates)").Value(&email),
					huh.NewConfirm().Title("Enable wildcard SSL for multi-tenant?").Value(&wildcardSSL),
				),
			)
			if err := form.Run(); err != nil {
				return err
			}

			target.Host = host
			target.SSHUser = sshUser
			target.SSHKey = sshKey
			target.Domain = domain
			target.Provider = "vps"

			vps := &providers.VPSProvider{}
			pr := deploy.NewProgress(juaCfg.Name, juaCfg.Version, "setup")
			opts := &providers.SetupOptions{
				Email:    email,
				Progress: pr,
			}
			return vps.Setup(target, opts)
		},
	}
}

// rollbackCmd rolls back to a previous version.
func rollbackCmd() *cobra.Command {
	var env, ver, cfgPath string

	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Roll back to a previous deployment version",
		RunE: func(cmd *cobra.Command, args []string) error {
			printLogo()
			juaCfg, err := deploy.LoadConfig(cfgPath)
			if err != nil {
				return err
			}
			target, err := juaCfg.Target(env)
			if err != nil {
				return err
			}
			provider, err := providers.Get(target.Provider)
			if err != nil {
				return err
			}
			if ver == "" {
				color.Red("\n  Specify a version: jua rollback --version v1.0.0\n")
				return fmt.Errorf("version required")
			}
			color.New(color.FgHiMagenta, color.Bold).Printf("\n  Rolling back %s to %s on %s...\n\n", juaCfg.Name, ver, env)
			if err := provider.Rollback(juaCfg, env, ver); err != nil {
				color.Red("\n  Rollback failed: %v\n", err)
				return err
			}
			color.New(color.FgHiGreen, color.Bold).Printf("\n  ✓ Rolled back to %s\n\n", ver)
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "production", "Deployment environment")
	cmd.Flags().StringVar(&ver, "version", "", "Version to roll back to")
	cmd.Flags().StringVar(&cfgPath, "config", "jua.yaml", "Path to jua.yaml")
	return cmd
}

// statusCmd prints the deployment status of all services.
func deployStatusCmd() *cobra.Command {
	var env, cfgPath string

	return &cobra.Command{
		Use:   "status",
		Short: "Show deployment health of all services",
		RunE: func(cmd *cobra.Command, args []string) error {
			juaCfg, err := deploy.LoadConfig(cfgPath)
			if err != nil {
				return err
			}
			target, err := juaCfg.Target(env)
			if err != nil {
				return err
			}
			provider, err := providers.Get(target.Provider)
			if err != nil {
				return err
			}

			status, err := provider.Status(juaCfg, env)
			if err != nil {
				return err
			}

			bold := color.New(color.FgHiWhite, color.Bold)
			green := color.New(color.FgHiGreen)
			red := color.New(color.FgHiRed)
			cyan := color.New(color.FgHiCyan)
			gray := color.New(color.FgHiBlack)

			fmt.Println()
			bold.Printf("  %s — %s\n\n", juaCfg.Name, env)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			gray.Fprintln(w, "  Service\tStatus\tHealth\tURL")
			gray.Fprintln(w, "  ───────\t──────\t──────\t───")
			for _, svc := range status.Services {
				statusStr := svc.Status
				healthStr := svc.Health
				if svc.Health == "healthy" {
					healthStr = green.Sprint("healthy")
				} else if svc.Health == "unhealthy" {
					healthStr = red.Sprint("unhealthy")
				}
				if svc.Status == "running" {
					statusStr = green.Sprint("running")
				} else if strings.Contains(svc.Status, "err") {
					statusStr = red.Sprint(svc.Status)
				}
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n",
					svc.Name, statusStr, healthStr, cyan.Sprint(svc.URL))
			}
			w.Flush()
			fmt.Println()
			return nil
		},
	}
}

// logsCmd streams logs from a service.
func logsCmd() *cobra.Command {
	var (
		env, cfgPath, service string
		follow                bool
		lines                 int
	)

	cmd := &cobra.Command{
		Use:   "logs [service]",
		Short: "View or stream logs from a service",
		Long: `Stream logs from a deployed service.

Examples:
  jua logs              All services, last 100 lines
  jua logs api          API service only
  jua logs --follow     Stream live
  jua logs --lines 500  Last 500 lines`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				service = args[0]
			}
			if service == "" {
				service = "api"
			}
			juaCfg, err := deploy.LoadConfig(cfgPath)
			if err != nil {
				return err
			}
			target, err := juaCfg.Target(env)
			if err != nil {
				return err
			}
			provider, err := providers.Get(target.Provider)
			if err != nil {
				return err
			}
			return provider.Logs(juaCfg, env, service, follow, lines)
		},
	}
	cmd.Flags().StringVar(&env, "env", "production", "Deployment environment")
	cmd.Flags().StringVar(&cfgPath, "config", "jua.yaml", "Path to jua.yaml")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Stream logs continuously")
	cmd.Flags().IntVar(&lines, "lines", 100, "Number of lines to show")
	return cmd
}

// envCmd is the parent for env subcommands.
func envCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage remote environment variables",
	}
	cmd.AddCommand(envPushCmd(), envPullCmd(), envListCmd(), envSetCmd())
	return cmd
}

func envPushCmd() *cobra.Command {
	var env, cfgPath string
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Sync local .env to the remote server",
		RunE: func(cmd *cobra.Command, args []string) error {
			juaCfg, err := deploy.LoadConfig(cfgPath)
			if err != nil {
				return err
			}
			target, err := juaCfg.Target(env)
			if err != nil {
				return err
			}
			provider, err := providers.Get(target.Provider)
			if err != nil {
				return err
			}
			color.New(color.FgHiMagenta).Printf("\n  Pushing env to %s...\n\n", env)
			if err := provider.EnvPush(juaCfg, env); err != nil {
				return err
			}
			color.New(color.FgHiGreen, color.Bold).Println("  ✓ Environment synced")
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "production", "Deployment environment")
	cmd.Flags().StringVar(&cfgPath, "config", "jua.yaml", "Path to jua.yaml")
	return cmd
}

func envPullCmd() *cobra.Command {
	var env, cfgPath, dest string
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull remote .env to local file",
		RunE: func(cmd *cobra.Command, args []string) error {
			juaCfg, err := deploy.LoadConfig(cfgPath)
			if err != nil {
				return err
			}
			target, err := juaCfg.Target(env)
			if err != nil {
				return err
			}
			provider, err := providers.Get(target.Provider)
			if err != nil {
				return err
			}
			if err := provider.EnvPull(juaCfg, env, dest); err != nil {
				return err
			}
			color.New(color.FgHiGreen).Printf("  ✓ Pulled to %s\n", dest)
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "production", "Deployment environment")
	cmd.Flags().StringVar(&cfgPath, "config", "jua.yaml", "Path to jua.yaml")
	cmd.Flags().StringVar(&dest, "out", ".env.remote", "Output file path")
	return cmd
}

func envListCmd() *cobra.Command {
	var env, cfgPath string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List remote environment variable keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			juaCfg, err := deploy.LoadConfig(cfgPath)
			if err != nil {
				return err
			}
			target, err := juaCfg.Target(env)
			if err != nil {
				return err
			}
			provider, err := providers.Get(target.Provider)
			if err != nil {
				return err
			}
			vars, err := provider.EnvList(juaCfg, env)
			if err != nil {
				return err
			}
			for k, v := range vars {
				fmt.Printf("  %s=%s\n", k, v)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "production", "Deployment environment")
	cmd.Flags().StringVar(&cfgPath, "config", "jua.yaml", "Path to jua.yaml")
	return cmd
}

func envSetCmd() *cobra.Command {
	var env, cfgPath string
	cmd := &cobra.Command{
		Use:   "set KEY=VALUE",
		Short: "Set a single remote environment variable",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parts := strings.SplitN(args[0], "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("format: KEY=VALUE")
			}
			juaCfg, err := deploy.LoadConfig(cfgPath)
			if err != nil {
				return err
			}
			target, err := juaCfg.Target(env)
			if err != nil {
				return err
			}
			provider, err := providers.Get(target.Provider)
			if err != nil {
				return err
			}
			if err := provider.EnvSet(juaCfg, env, parts[0], parts[1]); err != nil {
				return err
			}
			color.New(color.FgHiGreen).Printf("  ✓ Set %s on %s\n", parts[0], env)
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "production", "Deployment environment")
	cmd.Flags().StringVar(&cfgPath, "config", "jua.yaml", "Path to jua.yaml")
	return cmd
}

// scaleCmd changes replica count for a service.
func scaleCmd() *cobra.Command {
	var env, cfgPath string
	var replicas int

	cmd := &cobra.Command{
		Use:   "scale SERVICE",
		Short: "Scale a service to a given replica count",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			juaCfg, err := deploy.LoadConfig(cfgPath)
			if err != nil {
				return err
			}
			target, err := juaCfg.Target(env)
			if err != nil {
				return err
			}
			provider, err := providers.Get(target.Provider)
			if err != nil {
				return err
			}
			svc := args[0]
			color.New(color.FgHiMagenta).Printf("\n  Scaling %s to %d replicas on %s...\n\n", svc, replicas, env)
			if err := provider.Scale(juaCfg, env, svc, replicas); err != nil {
				return err
			}
			color.New(color.FgHiGreen, color.Bold).Printf("  ✓ %s scaled to %d\n\n", svc, replicas)
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "production", "Deployment environment")
	cmd.Flags().StringVar(&cfgPath, "config", "jua.yaml", "Path to jua.yaml")
	cmd.Flags().IntVar(&replicas, "replicas", 1, "Number of replicas")
	return cmd
}

// sshCmd opens an interactive SSH session.
func sshDeployCmd() *cobra.Command {
	var env, cfgPath string

	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Open an SSH session to the deployment server",
		RunE: func(cmd *cobra.Command, args []string) error {
			juaCfg, err := deploy.LoadConfig(cfgPath)
			if err != nil {
				return err
			}
			target, err := juaCfg.Target(env)
			if err != nil {
				return err
			}
			provider, err := providers.Get(target.Provider)
			if err != nil {
				return err
			}
			return provider.SSH(target)
		},
	}
	cmd.Flags().StringVar(&env, "env", "production", "Deployment environment")
	cmd.Flags().StringVar(&cfgPath, "config", "jua.yaml", "Path to jua.yaml")
	return cmd
}

// runCmd executes a command on the remote server.
func runDeployCmd() *cobra.Command {
	var env, cfgPath string

	cmd := &cobra.Command{
		Use:   "run COMMAND",
		Short: "Execute a command on the remote server",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			command := strings.Join(args, " ")
			juaCfg, err := deploy.LoadConfig(cfgPath)
			if err != nil {
				return err
			}
			target, err := juaCfg.Target(env)
			if err != nil {
				return err
			}
			provider, err := providers.Get(target.Provider)
			if err != nil {
				return err
			}
			return provider.Run(target, command)
		},
	}
	cmd.Flags().StringVar(&env, "env", "production", "Deployment environment")
	cmd.Flags().StringVar(&cfgPath, "config", "jua.yaml", "Path to jua.yaml")
	return cmd
}

// openCmd opens the deployed app in the browser.
func openCmd() *cobra.Command {
	var env, cfgPath string

	cmd := &cobra.Command{
		Use:   "open [service]",
		Short: "Open the deployed app in your browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			juaCfg, err := deploy.LoadConfig(cfgPath)
			if err != nil {
				return err
			}
			target, err := juaCfg.Target(env)
			if err != nil {
				return err
			}

			svc := "web"
			if len(args) > 0 {
				svc = args[0]
			}

			domain := target.Domain
			var url string
			switch svc {
			case "api":
				url = "https://api." + domain + "/health"
			case "admin":
				url = "https://admin." + domain
			default:
				url = "https://" + domain
			}

			color.New(color.FgHiCyan).Printf("\n  Opening %s\n\n", url)
			return openBrowser(url)
		},
	}
	cmd.Flags().StringVar(&env, "env", "production", "Deployment environment")
	cmd.Flags().StringVar(&cfgPath, "config", "jua.yaml", "Path to jua.yaml")
	return cmd
}

// --- helpers ---

func orStr(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func orVer(v, fallback string) string {
	if v != "" {
		return v
	}
	if fallback != "" {
		return fallback
	}
	return "latest"
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
