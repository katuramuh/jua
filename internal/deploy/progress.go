package deploy

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
)

// TerminalProgress implements providers.ProgressReporter with colored terminal output.
type TerminalProgress struct {
	AppName    string
	Version    string
	TargetName string
	startTime  time.Time
	stepStart  time.Time
}

// NewProgress creates a TerminalProgress and prints the deploy header.
func NewProgress(appName, version, targetName string) *TerminalProgress {
	p := &TerminalProgress{
		AppName:    appName,
		Version:    version,
		TargetName: targetName,
		startTime:  time.Now(),
	}
	p.printHeader()
	return p
}

func (p *TerminalProgress) printHeader() {
	bold := color.New(color.FgHiMagenta, color.Bold)
	gray := color.New(color.FgHiBlack)

	fmt.Println()
	bold.Printf("  Jua Deploy — %s %s → %s\n", p.AppName, p.Version, p.TargetName)
	gray.Println("  " + strings.Repeat("━", 44))
}

// Step announces the start of a named step.
func (p *TerminalProgress) Step(name string) {
	p.stepStart = time.Now()
	fmt.Printf("  ▸ %-34s", name)
}

// StepDone marks the current step as done.
func (p *TerminalProgress) StepDone(name string, d time.Duration) {
	green := color.New(color.FgHiGreen)
	gray := color.New(color.FgHiBlack)
	if d > 0 {
		gray.Printf(" ")
		green.Printf("✓")
		gray.Printf("  (%s)\n", formatDuration(d))
	} else {
		green.Println(" ✓")
	}
}

// StepFailed marks the current step as failed.
func (p *TerminalProgress) StepFailed(name string, err error) {
	red := color.New(color.FgHiRed)
	red.Printf(" ✗  %v\n", err)
}

// SubStep prints a service-level progress line.
func (p *TerminalProgress) SubStep(parent, name string, percent int, done bool) {
	bar := buildBar(percent, 12)
	green := color.New(color.FgHiGreen)
	gray := color.New(color.FgHiBlack)

	if done {
		green.Printf("  └─ %-8s [%s] 100%%  ✓\n", name, bar)
	} else {
		gray.Printf("  └─ %-8s [%s] %3d%%\r", name, bar, percent)
	}
}

// Log prints an informational message.
func (p *TerminalProgress) Log(message string) {
	gray := color.New(color.FgHiBlack)
	gray.Printf("     %s\n", message)
}

// Success prints the final success summary.
func (p *TerminalProgress) Success(message string) {
	elapsed := time.Since(p.startTime)
	gray := color.New(color.FgHiBlack)
	green := color.New(color.FgHiGreen, color.Bold)

	fmt.Println()
	gray.Println("  " + strings.Repeat("━", 44))
	green.Printf("  ✓ %s in %s\n", message, formatDuration(elapsed))
}

// Error prints an error message.
func (p *TerminalProgress) Error(message string) {
	color.New(color.FgHiRed).Printf("  ✗ %s\n", message)
}

// URL prints a service URL in cyan.
func (p *TerminalProgress) URL(label, url string) {
	cyan := color.New(color.FgHiCyan)
	fmt.Printf("  %-8s→ ", label)
	cyan.Println(url)
}

// Hint prints a follow-up command hint.
func (p *TerminalProgress) Hint(message string) {
	gray := color.New(color.FgHiBlack)
	gray.Printf("  %s\n", message)
}

// --- helpers ---

func buildBar(percent, width int) string {
	filled := (percent * width) / 100
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// FormatDuration formats a duration for display.
func FormatDuration(d time.Duration) string {
	return formatDuration(d)
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm %ds", m, s)
}
