package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/katuramuh/jua/v3/internal/generate"
	"github.com/katuramuh/jua/v3/internal/maintenance"
	"github.com/katuramuh/jua/v3/internal/project"
	"github.com/katuramuh/jua/v3/internal/prompt"
	"github.com/katuramuh/jua/v3/internal/routeparser"
	"github.com/katuramuh/jua/v3/internal/scaffold"
)

var version = "3.6.0"

func main() {
	rootCmd := &cobra.Command{
		Use:   "jua",
		Short: "Jua — Go + React. Built with Jua.",
		Long:  "Jua is a full-stack meta-framework that fuses Go (Gin + GORM) with Next.js (React + TypeScript).",
	}

	rootCmd.AddCommand(newCmd())
	rootCmd.AddCommand(newDesktopCmd())
	rootCmd.AddCommand(generateCmd())
	rootCmd.AddCommand(removeCmd())
	rootCmd.AddCommand(addCmd())
	rootCmd.AddCommand(startCmd())
	rootCmd.AddCommand(compileCmd())
	rootCmd.AddCommand(studioCmd())
	rootCmd.AddCommand(syncCmd())
	rootCmd.AddCommand(migrateCmd())
	rootCmd.AddCommand(seedCmd())
	rootCmd.AddCommand(upgradeCmd())
	rootCmd.AddCommand(updateCmd())
	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(routesCmd())
	rootCmd.AddCommand(downCmd())
	rootCmd.AddCommand(upCmd())
	rootCmd.AddCommand(newDeployCmd())
	rootCmd.AddCommand(rollbackCmd())
	rootCmd.AddCommand(deployStatusCmd())
	rootCmd.AddCommand(logsCmd())
	rootCmd.AddCommand(envCmd())
	rootCmd.AddCommand(scaleCmd())
	rootCmd.AddCommand(sshDeployCmd())
	rootCmd.AddCommand(runDeployCmd())
	rootCmd.AddCommand(openCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of Jua CLI",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("jua version %s\n", version)
		},
	}
}

func newCmd() *cobra.Command {
	// New architecture/frontend flags
	var archFlag, frontendFlag, style string
	var inPlace, force bool

	// Legacy flags (backward compatibility)
	var apiOnly, includeExpo, mobileOnly, full bool

	cmd := &cobra.Command{
		Use:   "new <project-name|.>",
		Short: "Create a new Jua project",
		Long:  "Scaffold a new Jua project. Interactive by default — select architecture and frontend.\nUse flags to skip prompts: jua new my-app --single --vite\nUse `jua new .` to scaffold in the current directory.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectArg := strings.TrimSpace(args[0])
			projectName := projectArg

			if filepath.Clean(projectArg) == "." {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("getting current directory: %w", err)
				}
				projectName = filepath.Base(cwd)
				inPlace = true
			}

			if err := scaffold.ValidateProjectName(projectName); err != nil {
				if filepath.Clean(projectArg) == "." {
					return fmt.Errorf("invalid current directory name %q for project generation: %w", projectName, err)
				}
				return err
			}

			opts := scaffold.Options{
				ProjectName: projectName,
				Style:       style,
				InPlace:     inPlace,
				Force:       force,
				// Legacy flags
				APIOnly:     apiOnly,
				IncludeExpo: includeExpo,
				MobileOnly:  mobileOnly,
				Full:        full,
			}

			if !opts.InPlace {
				cwd, err := os.Getwd()
				if err == nil && filepath.Base(cwd) == projectName {
					opts.InPlace = true
				}
			}

			// Map architecture shorthand flags
			switch archFlag {
			case "single":
				opts.Architecture = scaffold.ArchSingle
			case "double":
				opts.Architecture = scaffold.ArchDouble
			case "triple":
				opts.Architecture = scaffold.ArchTriple
			case "api":
				opts.Architecture = scaffold.ArchAPI
			case "mobile":
				opts.Architecture = scaffold.ArchMobile
			case "":
				// Will be set by legacy flags or interactive prompt
			default:
				return fmt.Errorf("invalid architecture %q: must be single, double, triple, api, or mobile", archFlag)
			}

			// Map frontend shorthand flags
			switch frontendFlag {
			case "next":
				opts.Frontend = scaffold.FrontendNext
			case "vite", "tanstack":
				opts.Frontend = scaffold.FrontendTanStack
			case "":
				// Will be set by interactive prompt or default
			default:
				return fmt.Errorf("invalid frontend %q: must be next, vite, or tanstack", frontendFlag)
			}

			// Show interactive selector only when the user did not provide any
			// architecture/frontend shortcuts or explicit long-form flags.
			// We check the resolved flag values directly (set by PreRunE) rather
			// than cmd.Flags().Changed(), which is more reliable across flag sources.
			anyFlagSet := archFlag != "" || frontendFlag != "" ||
				apiOnly || mobileOnly || full || includeExpo

			if !anyFlagSet {
				printLogo()
				// Keep empty values so the prompt can collect architecture/frontend.
				opts.Architecture = ""
				opts.Frontend = ""
				if err := prompt.RunNewProjectPrompt(&opts); err != nil {
					return err
				}
			}

			// Final normalization (sets defaults for anything still empty)
			opts.Normalize()

			if err := opts.ValidateStyle(); err != nil {
				return err
			}

			printLogo()

			purple := color.New(color.FgHiMagenta, color.Bold)
			purple.Printf("\n  Creating new Jua project: %s\n", projectName)

			gray := color.New(color.FgHiBlack)
			gray.Printf("  Architecture: %s | Frontend: %s\n\n", opts.Architecture, opts.Frontend)

			if err := scaffold.Run(opts); err != nil {
				color.Red("\n  Error: %v\n", err)
				return err
			}

			printSuccess(projectName, opts)
			return nil
		},
	}

	// New flags
	cmd.Flags().StringVar(&archFlag, "arch", "", "Architecture: single, double, triple, api, mobile")
	cmd.Flags().StringVar(&frontendFlag, "frontend", "", "Frontend framework: next, vite (tanstack)")
	cmd.Flags().StringVar(&style, "style", "", "Admin panel style variant (default, modern, minimal, glass)")

	// Shorthand architecture flags
	cmd.Flags().BoolVar(&apiOnly, "api", false, "Shorthand for --arch=api")
	cmd.Flags().BoolVar(&mobileOnly, "mobile", false, "Shorthand for --arch=mobile")
	cmd.Flags().BoolVar(&full, "full", false, "Shorthand for --arch=triple with docs")
	cmd.Flags().BoolVar(&includeExpo, "expo", false, "Include Expo mobile app")

	// Shorthand frontend flags
	cmd.Flags().Bool("vite", false, "Shorthand for --frontend=vite (TanStack Router)")
	cmd.Flags().Bool("next", false, "Shorthand for --frontend=next (Next.js)")

	// Handle shorthand frontend flags
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if v, _ := cmd.Flags().GetBool("vite"); v {
			frontendFlag = "vite"
		}
		if v, _ := cmd.Flags().GetBool("next"); v {
			frontendFlag = "next"
		}
		// Shorthand single/double/triple
		if v, _ := cmd.Flags().GetBool("single"); v {
			archFlag = "single"
		}
		if v, _ := cmd.Flags().GetBool("double"); v {
			archFlag = "double"
		}
		if v, _ := cmd.Flags().GetBool("triple"); v {
			archFlag = "triple"
		}
		return nil
	}

	// Shorthand architecture flags
	cmd.Flags().Bool("single", false, "Shorthand for --arch=single")
	cmd.Flags().Bool("double", false, "Shorthand for --arch=double")
	cmd.Flags().Bool("triple", false, "Shorthand for --arch=triple")
	cmd.Flags().BoolVar(&inPlace, "here", false, "Scaffold into the current directory instead of creating a new folder")
	cmd.Flags().BoolVar(&force, "force", false, "Allow scaffolding into a non-empty directory (use with --here)")

	return cmd
}

func generateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "generate",
		Short:   "Generate code for your Jua project",
		Aliases: []string{"g"},
	}

	cmd.AddCommand(generateResourceCmd())
	cmd.AddCommand(generateAgentsCmd())
	cmd.AddCommand(generateNotificationCmd())

	return cmd
}

func generateAgentsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agents",
		Short: "Generate AGENTS.md — context file for AI coding agents",
		Long: `Generate an AGENTS.md file at the project root.

AGENTS.md gives AI coding agents (Claude Code, Cursor, GitHub Copilot, etc.)
full context about the project architecture, conventions, tech stack, and
common development tasks — so they can contribute effectively without
reading every file first.

Run this command again to refresh AGENTS.md after major project changes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			printLogo()

			purple := color.New(color.FgHiMagenta, color.Bold)
			purple.Printf("\n  Generating AGENTS.md...\n\n")

			info, err := project.DetectProject()
			if err != nil {
				return fmt.Errorf("generating agents: %w", err)
			}

			// Derive scaffold options from the detected project
			opts := scaffold.Options{
				ProjectName: filepath.Base(info.Root),
			}
			switch info.Type {
			case project.ProjectDesktop:
				opts.Architecture = scaffold.ArchSingle
			default:
				// Detect triple vs double vs api from directory presence
				if _, err := os.Stat(filepath.Join(info.Root, "apps", "admin")); err == nil {
					opts.Architecture = scaffold.ArchTriple
				} else if _, err := os.Stat(filepath.Join(info.Root, "apps", "web")); err == nil {
					opts.Architecture = scaffold.ArchDouble
				} else {
					opts.Architecture = scaffold.ArchAPI
				}
			}
			opts.Normalize()

			if err := scaffold.WriteAgentsMD(info.Root, opts); err != nil {
				return fmt.Errorf("generating agents: %w", err)
			}

			green := color.New(color.FgGreen, color.Bold)
			green.Printf("  ✓ AGENTS.md written to %s\n\n", filepath.Join(info.Root, "AGENTS.md"))
			fmt.Println("  Give this file to your AI agent to get full project context.")
			fmt.Println("  Refresh it anytime with: jua generate agents")
			return nil
		},
	}
}

func generateNotificationCmd() *cobra.Command {
	var notifType string
	var channels string

	return &cobra.Command{
		Use:   "notification <TemplateName>",
		Short: "Generate a notification template",
		Long: `Generate a typed notification template for the Jua notifications centre.

Creates:
  apps/api/jua/notifications/templates/<name>.go   Go template definition
  apps/api/emails/<name>.html                      Email HTML template

Examples:
  jua generate notification PaymentReceived --type transactional --channels "sms,email,push,inapp"
  jua generate notification OrderShipped --type operational --channels "push,inapp"
  jua generate notification WeeklyDigest --type marketing`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			printLogo()

			// Resolve project root and API directory
			apiDir, err := findAPIDir()
			if err != nil {
				return fmt.Errorf("notification generator: %w", err)
			}

			// snake_case the name for file names
			snake := toSnakeCase(name)

			// Build channel list from flag
			channelList := []string{"sms", "email", "push", "inapp"}
			if channels != "" {
				channelList = strings.Split(channels, ",")
				for i, c := range channelList {
					channelList[i] = strings.TrimSpace(c)
				}
			}

			// Validate notification type
			if notifType == "" {
				notifType = "operational"
			}
			validTypes := map[string]bool{"transactional": true, "operational": true, "marketing": true, "security": true}
			if !validTypes[notifType] {
				return fmt.Errorf("invalid type %q: must be transactional, operational, marketing, or security", notifType)
			}

			// Generate Go template file
			goContent := generateNotificationGoTemplate(name, snake, notifType, channelList)
			goPath := filepath.Join(apiDir, "jua", "notifications", "templates", snake+".go")
			if err := writeFileContent(goPath, goContent); err != nil {
				return fmt.Errorf("writing %s: %w", goPath, err)
			}

			// Generate email HTML template
			htmlContent := generateNotificationHTMLTemplate(name)
			htmlPath := filepath.Join(apiDir, "emails", snake+".html")
			if err := writeFileContent(htmlPath, htmlContent); err != nil {
				return fmt.Errorf("writing %s: %w", htmlPath, err)
			}

			green := color.New(color.FgGreen, color.Bold)
			green.Printf("\n  ✓ Notification template %q created\n\n", name)
			fmt.Printf("  %s\n", goPath)
			fmt.Printf("  %s\n\n", htmlPath)
			fmt.Printf("  Register it: notifications.RegisterTemplate(templates.%s)\n\n", name)
			return nil
		},
	}
}

// generateNotificationGoTemplate produces the Go source for a notification template.
func generateNotificationGoTemplate(name, snake, notifType string, channels []string) string {
	hasChannel := func(ch string) bool {
		for _, c := range channels {
			if c == ch {
				return true
			}
		}
		return false
	}

	smsLine := ""
	if hasChannel("sms") {
		smsLine = "\n\tSMS:  \"{{.Title}}: {{.Body}}\","
	}
	emailSection := ""
	if hasChannel("email") {
		emailSection = fmt.Sprintf("\n\tEmail: templates.EmailTpl{\n\t\tSubject:  \"{{.Title}}\",\n\t\tHTMLFile: \"emails/%s.html\",\n\t},", snake)
	}
	pushSection := ""
	if hasChannel("push") {
		pushSection = "\n\tPush: templates.PushTpl{\n\t\tTitle: \"{{.Title}}\",\n\t\tBody:  \"{{.Body}}\",\n\t},"
	}
	inappSection := ""
	if hasChannel("inapp") {
		inappSection = "\n\tInApp: templates.InAppTpl{\n\t\tTitle: \"{{.Title}}\",\n\t\tBody:  \"{{.Body}}\",\n\t\tIcon:  \"🔔\",\n\t},"
	}

	return fmt.Sprintf(`package templates

// %s is a Jua notification template.
// Generated by: jua generate notification %s --type %s
//
// Register it in your app startup:
//
//	notifications.RegisterTemplate(templates.%s)
var %s = &Template{
	Name: "%s",
	Type: "%s",%s%s%s%s
}

func init() {
	// Auto-register on package import
	// Remove this init() if you prefer explicit registration.
	// notifications.RegisterTemplate(%s)
}
`, name, name, notifType, name, name, snake, notifType, smsLine, emailSection, pushSection, inappSection, name)
}

// generateNotificationHTMLTemplate produces a starter email HTML template.
func generateNotificationHTMLTemplate(name string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Title}}</title>
  <style>
    body { font-family: 'Onest', -apple-system, sans-serif; background: #0a0a0f; color: #e8e8f0; margin: 0; padding: 0; }
    .container { max-width: 600px; margin: 40px auto; background: #111118; border-radius: 12px; border: 1px solid #2a2a3a; overflow: hidden; }
    .header { background: #6c5ce7; padding: 32px; text-align: center; }
    .header h1 { margin: 0; font-size: 24px; color: #fff; font-weight: 700; }
    .body { padding: 32px; }
    .body p { color: #9090a8; line-height: 1.6; margin: 0 0 16px; }
    .cta { display: inline-block; padding: 12px 24px; background: #6c5ce7; color: #fff; border-radius: 8px; text-decoration: none; font-weight: 600; margin-top: 8px; }
    .footer { padding: 24px 32px; border-top: 1px solid #2a2a3a; text-align: center; }
    .footer p { color: #606078; font-size: 12px; margin: 0; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>{{.Title}}</h1>
    </div>
    <div class="body">
      <p>{{.Body}}</p>
      <!-- Add template-specific content for %s here -->
    </div>
    <div class="footer">
      <p>Built with Jua — Go + React for Africa</p>
    </div>
  </div>
</body>
</html>
`, name)
}

// toSnakeCase converts PascalCase to snake_case.
func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r|32) // toLower
	}
	return string(result)
}

// writeFileContent writes content to a file, creating parent dirs as needed.
func writeFileContent(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func removeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove",
		Short:   "Remove components from your Jua project",
		Aliases: []string{"rm"},
	}

	cmd.AddCommand(removeResourceCmd())

	return cmd
}

func removeResourceCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "resource <Name>",
		Short: "Remove a previously generated resource",
		Long:  "Delete generated files and reverse all marker-based injections for a resource.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			printLogo()

			if !force {
				fmt.Printf("\n  ⚠ This will remove all files and injections for resource %q.\n", args[0])
				if !generate.ConfirmRemoval() {
					fmt.Println("\n  Cancelled.")
					return nil
				}
			}

			return generate.RemoveResource(args[0])
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")

	return cmd
}

func addCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add components to your Jua project",
	}

	cmd.AddCommand(addRoleCmd())

	return cmd
}

func addRoleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "role <ROLE_NAME>",
		Short: "Add a new role to the project",
		Long:  "Adds a new role constant to Go models, TypeScript types, Zod schemas, constants, and admin resource definitions.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			printLogo()

			purple := color.New(color.FgHiMagenta, color.Bold)
			purple.Printf("\n  Adding role: %s\n\n", strings.ToUpper(args[0]))

			return scaffold.AddRole(args[0])
		},
	}
}

func generateResourceCmd() *cobra.Command {
	var fromFile string
	var interactive bool
	var fields string
	var roles string

	cmd := &cobra.Command{
		Use:   "resource <Name>",
		Short: "Generate a full-stack CRUD resource",
		Long:  "Generate Go model, handler, service, Zod schemas, TypeScript types, React Query hooks, and admin page.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			printLogo()

			var def *generate.ResourceDefinition
			var err error

			switch {
			case fromFile != "":
				def, err = generate.LoadFromYAML(fromFile)
				if err != nil {
					return err
				}
				// Override name if provided
				if name != "" {
					def.Name = name
				}
			case fields != "":
				def, err = generate.ParseInlineFields(name, fields)
				if err != nil {
					return err
				}
			case interactive:
				def, err = generate.PromptInteractive(name)
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("specify fields with --fields, --from, or use -i for interactive mode\n\nExamples:\n  jua generate resource Post --fields \"title:string,content:text,published:bool\"\n  jua generate resource Post --fields \"title:string,slug:string:unique,views:int\"\n  jua generate resource Post --from post.yaml\n  jua generate resource Post -i")
			}

			// Detect project type and dispatch
			info, _ := project.DetectProject()
			if info != nil && info.Type == project.ProjectDesktop {
				return generate.RunDesktop(def)
			}

			gen, err := generate.NewGenerator(def)
			if err != nil {
				return err
			}

			// Parse --roles flag
			if roles != "" {
				parts := strings.Split(roles, ",")
				for _, r := range parts {
					r = strings.TrimSpace(strings.ToUpper(r))
					if r != "" {
						gen.Roles = append(gen.Roles, r)
					}
				}
			}

			return gen.Run()
		},
	}

	cmd.Flags().StringVar(&fromFile, "from", "", "YAML file defining the resource fields")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactively define fields")
	cmd.Flags().StringVar(&fields, "fields", "", "Inline field definitions (e.g., \"title:string,content:text,published:bool\")")
	cmd.Flags().StringVar(&roles, "roles", "", "Restrict routes to specific roles (comma-separated, e.g., \"ADMIN,EDITOR\")")

	return cmd
}

func migrateCmd() *cobra.Command {
	var fresh bool

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		Long:  "Connect to the database and run GORM AutoMigrate for all models. Use --fresh to drop all tables first.",
		RunE: func(cmd *cobra.Command, args []string) error {
			printLogo()

			apiDir, err := findAPIDir()
			if err != nil {
				return err
			}

			goArgs := []string{"run", "cmd/migrate/main.go"}
			if fresh {
				goArgs = append(goArgs, "--fresh")
			}

			c := exec.Command("go", goArgs...)
			c.Dir = apiDir
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			c.Stdin = os.Stdin

			purple := color.New(color.FgHiMagenta, color.Bold)
			if fresh {
				purple.Println("\n  Running fresh migration (drop + re-migrate)...")
			} else {
				purple.Println("\n  Running database migrations...")
			}

			return c.Run()
		},
	}

	cmd.Flags().BoolVar(&fresh, "fresh", false, "Drop all tables before migrating (migrate:fresh)")

	return cmd
}

func seedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "seed",
		Short: "Run database seeders",
		Long:  "Populate the database with initial data (admin user, demo users).",
		RunE: func(cmd *cobra.Command, args []string) error {
			printLogo()

			apiDir, err := findAPIDir()
			if err != nil {
				return err
			}

			c := exec.Command("go", "run", "cmd/seed/main.go")
			c.Dir = apiDir
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			c.Stdin = os.Stdin

			purple := color.New(color.FgHiMagenta, color.Bold)
			purple.Println("\n  Seeding database...")

			return c.Run()
		},
	}
}

// findAPIDir locates the apps/api directory from the project root.
func findAPIDir() (string, error) {
	root, err := scaffold.FindProjectRoot()
	if err != nil {
		return "", err
	}
	apiDir := filepath.Join(root, "apps", "api")
	if _, err := os.Stat(apiDir); os.IsNotExist(err) {
		return "", fmt.Errorf("apps/api directory not found in %s", root)
	}
	return apiDir, nil
}

func upgradeCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade an existing Jua project to the latest scaffold templates",
		Long:  "Regenerates framework components (admin panel, web app, configs) while preserving your resource definitions and API code.",
		RunE: func(cmd *cobra.Command, args []string) error {
			printLogo()

			purple := color.New(color.FgHiMagenta, color.Bold)
			purple.Printf("\n  Upgrading project to Jua v%s\n\n", version)

			return scaffold.Upgrade(scaffold.UpgradeOptions{
				Force: force,
			})
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite all files without prompting")

	return cmd
}

func updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update the Jua CLI to the latest version",
		Long:  "Removes the current Jua binary and installs the latest version from GitHub using go install.",
		RunE: func(cmd *cobra.Command, args []string) error {
			printLogo()

			purple := color.New(color.FgHiMagenta, color.Bold)
			green := color.New(color.FgHiGreen, color.Bold)
			spinner := color.New(color.FgHiBlack)

			purple.Printf("\n  Updating Jua CLI (current: v%s)...\n\n", version)

			// Find the current binary path
			binPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("finding current binary: %w", err)
			}
			binPath, err = filepath.EvalSymlinks(binPath)
			if err != nil {
				return fmt.Errorf("resolving binary path: %w", err)
			}

			// On Windows, a running binary can't be deleted but can be renamed
			if runtime.GOOS == "windows" {
				oldPath := binPath + ".old"
				// Clean up any previous .old file
				os.Remove(oldPath)
				spinner.Printf("  → Renaming old binary: %s\n", binPath)
				if err := os.Rename(binPath, oldPath); err != nil {
					return fmt.Errorf("renaming old binary: %w", err)
				}
			} else {
				spinner.Printf("  → Removing old binary: %s\n", binPath)
				if err := os.Remove(binPath); err != nil {
					return fmt.Errorf("removing old binary: %w", err)
				}
			}

			spinner.Println("  → Installing latest version...")
			c := exec.Command("go", "install", "github.com/katuramuh/jua/v3/cmd/jua@latest")
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				return fmt.Errorf("installing latest version: %w", err)
			}

			// Clean up renamed binary on Windows
			if runtime.GOOS == "windows" {
				os.Remove(binPath + ".old")
			}

			fmt.Println()
			green.Println("  ✓ Jua CLI updated successfully!")
			spinner.Println("  Run 'jua version' to verify the new version.")
			fmt.Println()

			return nil
		},
	}
}

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start development servers",
		Long:  "Start the Go API server or frontend client apps for local development. In a desktop project, runs wails dev.",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := project.DetectProject()
			if err != nil {
				// Not inside a project — show subcommand help
				return cmd.Help()
			}
			if info.Type == project.ProjectDesktop {
				printLogo()
				purple := color.New(color.FgHiMagenta, color.Bold)
				purple.Println("\n  Starting Wails desktop app...")

				c := exec.Command("wails", "dev")
				c.Dir = info.Root
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
				c.Stdin = os.Stdin
				return c.Run()
			}
			// Web project — show subcommand help
			return cmd.Help()
		},
	}

	cmd.AddCommand(startClientCmd())
	cmd.AddCommand(startServerCmd())

	return cmd
}

func compileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "compile",
		Short: "Build desktop application executable",
		Long:  "Compile the Wails desktop app into a distributable binary. Only available in desktop projects.",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := project.DetectProject()
			if err != nil {
				return fmt.Errorf("not inside a Jua project: %w", err)
			}
			if info.Type != project.ProjectDesktop {
				return fmt.Errorf("jua compile is only available in desktop projects (wails.json not found)")
			}

			printLogo()
			purple := color.New(color.FgHiMagenta, color.Bold)
			purple.Println("\n  Building desktop executable...")

			c := exec.Command("wails", "build")
			c.Dir = info.Root
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			c.Stdin = os.Stdin
			return c.Run()
		},
	}
}

func studioCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "studio",
		Short: "Open GORM Studio database browser",
		Long:  "Launch GORM Studio at http://localhost:8080/studio. For web projects, ensure your API server is running first. For desktop, starts the studio server and opens the browser.",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := project.DetectProject()
			if err != nil {
				return fmt.Errorf("not inside a Jua project: %w", err)
			}

			printLogo()

			if info.Type == project.ProjectDesktop {
				purple := color.New(color.FgHiMagenta, color.Bold)
				purple.Println("\n  Starting GORM Studio...")

				c := exec.Command("go", "run", "cmd/studio/main.go")
				c.Dir = info.Root
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
				c.Stdin = os.Stdin
				return c.Run()
			}

			// Web project — open browser
			gray := color.New(color.FgHiBlack)
			gray.Println("\n  GORM Studio is available at http://localhost:8080/studio")
			gray.Println("  Make sure your API server is running (jua start server)")

			openURL("http://localhost:8080/studio")
			return nil
		},
	}
}

func startClientCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "client",
		Short: "Start frontend apps (web, admin, expo)",
		Long:  "Runs 'pnpm dev' from the project root to start all frontend apps via Turborepo.",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := scaffold.FindProjectRoot()
			if err != nil {
				return err
			}

			purple := color.New(color.FgHiMagenta, color.Bold)
			purple.Println("\n  Starting client apps...")

			c := exec.Command("pnpm", "dev")
			c.Dir = root
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			c.Stdin = os.Stdin

			return c.Run()
		},
	}
}

func startServerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Start the Go API server",
		Long:  "Runs 'go run cmd/server/main.go' from the apps/api directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			apiDir, err := findAPIDir()
			if err != nil {
				return err
			}

			purple := color.New(color.FgHiMagenta, color.Bold)
			purple.Println("\n  Starting API server...")

			c := exec.Command("go", "run", "cmd/server/main.go")
			c.Dir = apiDir
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			c.Stdin = os.Stdin

			return c.Run()
		},
	}
}

func syncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync Go types to TypeScript types and Zod schemas",
		Long:  "Parse Go model files and regenerate TypeScript types and Zod schemas in packages/shared.",
		RunE: func(cmd *cobra.Command, args []string) error {
			printLogo()
			return generate.Sync()
		},
	}
}

func openURL(url string) {
	switch runtime.GOOS {
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		exec.Command("open", url).Start()
	default:
		exec.Command("xdg-open", url).Start()
	}
}

func printLogo() {
	purple := color.New(color.FgHiMagenta, color.Bold)
	purple.Println(`
   ██████╗ ██████╗ ██╗████████╗
  ██╔════╝ ██╔══██╗██║╚══██╔══╝
  ██║  ███╗██████╔╝██║   ██║
  ██║   ██║██╔══██╗██║   ██║
  ╚██████╔╝██║  ██║██║   ██║
   ╚═════╝ ╚═╝  ╚═╝╚═╝   ╚═╝`)
	gray := color.New(color.FgHiBlack)
	gray.Printf("  Go + React. Built with Jua. v%s\n", version)
}

func printSuccess(name string, opts scaffold.Options) {
	green := color.New(color.FgHiGreen, color.Bold)
	white := color.New(color.FgWhite)
	cyan := color.New(color.FgHiCyan)
	gray := color.New(color.FgHiBlack)

	fmt.Println()
	green.Println("  ✓ Project created successfully!")
	fmt.Println()

	white.Println("  Next steps:")
	fmt.Println()
	if !opts.InPlace {
		cyan.Printf("    cd %s\n", name)
	}
	cyan.Println("    docker compose up -d")

	switch opts.Architecture {
	case scaffold.ArchAPI:
		cyan.Println("    cd apps/api && go run cmd/server/main.go")
	case scaffold.ArchSingle:
		cyan.Println("    cd frontend && pnpm install")
		cyan.Println("    go run main.go")
	default:
		cyan.Println("    pnpm install")
		cyan.Println("    pnpm dev")
	}

	if opts.ShouldIncludeExpo() {
		cyan.Println("    cd apps/expo && npx expo start")
	}

	fmt.Println()
	gray.Println("  ─────────────────────────────────────")
	gray.Printf("  API:         http://localhost:8080\n")
	gray.Printf("  API Docs:    http://localhost:8080/docs\n")
	gray.Printf("  GORM Studio: http://localhost:8080/studio\n")
	gray.Printf("  Sentinel:    http://localhost:8080/sentinel/ui\n")

	if opts.ShouldIncludeWeb() {
		gray.Printf("  Web App:     http://localhost:3000\n")
	}
	if opts.ShouldIncludeAdmin() {
		gray.Printf("  Admin:       http://localhost:3001\n")
	}
	if opts.ShouldIncludeSingleSPA() {
		gray.Printf("  Frontend:    http://localhost:5173\n")
	}
	if opts.ShouldIncludeExpo() {
		gray.Printf("  Expo:        exp://localhost:8081\n")
	}
	if opts.ShouldIncludeDocs() {
		gray.Printf("  Docs:        http://localhost:3002\n")
	}

	gray.Printf("  PostgreSQL:  localhost:5432\n")
	gray.Printf("  Redis:       localhost:6379\n")
	gray.Printf("  MinIO:       http://localhost:9001\n")
	gray.Printf("  Mailhog:     http://localhost:8025\n")
	gray.Println("  ─────────────────────────────────────")
	fmt.Println()
}

func newDesktopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new-desktop <project-name>",
		Short: "Create a new Wails desktop application",
		Long:  "Scaffold a standalone desktop app with Go backend (Wails), React frontend, SQLite/PostgreSQL, and local auth.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectName := args[0]

			if err := scaffold.ValidateProjectName(projectName); err != nil {
				return err
			}

			printLogo()

			purple := color.New(color.FgHiMagenta, color.Bold)
			purple.Printf("\n  Creating new Jua desktop app: %s\n\n", projectName)

			opts := scaffold.DesktopOptions{
				ProjectName: projectName,
			}

			if err := scaffold.RunDesktop(opts); err != nil {
				color.Red("\n  Error: %v\n", err)
				return err
			}

			printDesktopSuccess(projectName)
			return nil
		},
	}

	return cmd
}

func printDesktopSuccess(name string) {
	green := color.New(color.FgHiGreen, color.Bold)
	white := color.New(color.FgWhite)
	cyan := color.New(color.FgHiCyan)
	gray := color.New(color.FgHiBlack)

	fmt.Println()
	green.Println("  ✓ Desktop app created successfully!")
	fmt.Println()

	white.Println("  Next steps:")
	fmt.Println()
	cyan.Printf("    cd %s\n", name)
	cyan.Println("    wails dev")
	fmt.Println()

	gray.Println("  ─────────────────────────────────────")
	gray.Printf("  Desktop App: http://localhost:34115\n")
	gray.Printf("  Database:    SQLite (%s.db)\n", name)
	gray.Println("  ─────────────────────────────────────")
	fmt.Println()

	gray.Println("  To build for production:")
	cyan.Println("    wails build")
	fmt.Println()
}

// ── jua routes ──────────────────────────────────────────────────────────────

func routesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "routes",
		Short: "List all registered API routes",
		Long:  "Parse the routes.go file and display a table of all registered HTTP routes with their methods, paths, handlers, and middleware groups.",
		RunE: func(cmd *cobra.Command, args []string) error {
			printLogo()

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}

			routesFile, err := routeparser.FindRoutesFile(cwd)
			if err != nil {
				return err
			}

			routes, err := routeparser.Parse(routesFile)
			if err != nil {
				return err
			}

			purple := color.New(color.FgHiMagenta, color.Bold)
			purple.Printf("\n  API Routes (%s)\n\n", routesFile)

			fmt.Println(routeparser.FormatTable(routes))
			return nil
		},
	}
}

// ── jua down / jua up ─────────────────────────────────────────────────────

func downCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Put the application in maintenance mode",
		Long:  "Creates a .maintenance file that triggers the maintenance middleware, returning 503 for all requests.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			if maintenance.IsEnabled(cwd) {
				color.Yellow("\n  Application is already in maintenance mode.\n")
				return nil
			}

			if err := maintenance.Enable(cwd); err != nil {
				return err
			}

			color.New(color.FgHiYellow, color.Bold).Println("\n  Application is now in maintenance mode.")
			color.New(color.FgHiBlack).Println("  All requests will receive 503 Service Unavailable.")
			color.New(color.FgHiBlack).Println("  Run 'jua up' to bring it back online.")
			fmt.Println()
			return nil
		},
	}
}

func upCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Bring the application back online",
		Long:  "Removes the .maintenance file, allowing normal request handling to resume.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			if err := maintenance.Disable(cwd); err != nil {
				return err
			}

			color.New(color.FgHiGreen, color.Bold).Println("\n  Application is back online!")
			color.New(color.FgHiBlack).Println("  Normal request handling has resumed.")
			fmt.Println()
			return nil
		},
	}
}

// deploy commands are defined in deploy_commands.go